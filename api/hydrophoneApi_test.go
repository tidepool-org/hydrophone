package api

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"./../clients"
	"./../models"

	"github.com/gorilla/mux"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
)

const (
	make_store_fail           = true
	make_store_return_nothing = true

	testing_token = "a.fake.token.to.use.in.tests"

	testing_token_uid1 = "a.fake.token.for.uid.1"
	testing_uid1       = "UID123"

	testing_token_uid2 = "a.fake.token.for.uid.2"
	testing_uid2       = "UID999"
)

var (
	NO_PARAMS = map[string]string{}

	FAKE_CONFIG = Config{
		ServerSecret: "shhh! don't tell",
		Templates: &models.TemplateConfig{
			PasswordReset:  `{{define "reset_test"}} {{ .Email }} {{ .Key }} {{end}}{{template "reset_test" .}}`,
			CareteamInvite: `{{define "invite_test"}} {{ .CareteamName }} {{ .Key }} {{end}}{{template "invite_test" .}}`,
			Signup:         `{{define "confirm_test"}} {{ .UserId }} {{ .Key }} {{end}}{{template "confirm_test" .}}`,
		},
		InviteTimeoutDays: 7,
		ResetTimeoutDays:  7,
		SignUpTimeoutDays: 7,
	}
	/*
	 * basics setup
	 */
	rtr            = mux.NewRouter()
	mockNotifier   = clients.NewMockNotifier()
	mockShoreline  = shoreline.NewMock(testing_token)
	mockGatekeeper = commonClients.NewGatekeeperMock(nil, &status.StatusError{status.NewStatus(500, "Unable to parse response.")})

	mockMetrics = highwater.NewMock()
	mockSeagull = commonClients.NewSeagullMock()
	/*
	 * stores
	 */
	mockStore      = clients.NewMockStoreClient(false, false)
	mockStoreEmpty = clients.NewMockStoreClient(make_store_return_nothing, false)
	mockStoreFails = clients.NewMockStoreClient(false, make_store_fail)

	/*
	 * users permissons scenarios
	 */
	mock_NoPermsGatekeeper = commonClients.NewGatekeeperMock(commonClients.Permissions{"upload": commonClients.Permission{"userid": "other-id"}}, nil)
	mock_uid1Shoreline     = newtestingShorelingMock(testing_uid1)

	responsableGatekeeper = NewResponsableGatekeeper()
	responsableHydrophone = InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, responsableGatekeeper, mockMetrics, mockSeagull)
)

// In an effort to mock shoreline so that we can return the token we wish
type testingShorelingMock struct{ userid string }

func newtestingShorelingMock(userid string) *testingShorelingMock {
	return &testingShorelingMock{userid: userid}
}

func (m *testingShorelingMock) Start() error { return nil }
func (m *testingShorelingMock) Close()       { return }
func (m *testingShorelingMock) Login(username, password string) (*shoreline.UserData, string, error) {
	return &shoreline.UserData{UserID: m.userid, Emails: []string{m.userid + "@email.org"}, UserName: m.userid + "@email.org"}, "", nil
}
func (m *testingShorelingMock) Signup(username, password, email string) (*shoreline.UserData, error) {
	return &shoreline.UserData{UserID: m.userid, Emails: []string{m.userid + "@email.org"}, UserName: m.userid + "@email.org"}, nil
}
func (m *testingShorelingMock) TokenProvide() string { return testing_token }
func (m *testingShorelingMock) GetUser(userID, token string) (*shoreline.UserData, error) {
	return &shoreline.UserData{UserID: m.userid, Emails: []string{m.userid + "@email.org"}, UserName: m.userid + "@email.org"}, nil
}
func (m *testingShorelingMock) UpdateUser(user shoreline.UserUpdate, token string) error { return nil }
func (m *testingShorelingMock) CheckToken(token string) *shoreline.TokenData {
	return &shoreline.TokenData{UserID: m.userid, IsServer: false}
}

type (
	//common test structure
	toTest struct {
		skip       bool
		returnNone bool
		method     string
		url        string
		body       jo
		token      string
		respCode   int
		response   jo
	}
	// These two types make it easier to define blobs of json inline.
	// We don't use the types defined by the API because we want to
	// be able to test with partial data structures.
	// jo is a generic json object
	jo map[string]interface{}

	// and ja is a generic json array
	ja []interface{}
)

func TestGetStatus_StatusOk(t *testing.T) {

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull)
	hydrophone.SetHandlers("", rtr)

	hydrophone.GetStatus(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusOK)
	}

}

func TestGetStatus_StatusInternalServerError(t *testing.T) {

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophoneFails := InitApi(FAKE_CONFIG, mockStoreFails, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull)
	hydrophoneFails.SetHandlers("", rtr)

	hydrophoneFails.GetStatus(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusInternalServerError)
	}

	body, _ := ioutil.ReadAll(response.Body)

	if string(body) != `{"code":500,"reason":"Session failure"}` {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "Session failure")
	}
}

func (i *jo) deepCompare(j *jo) string {
	for k, _ := range *i {
		if reflect.DeepEqual((*i)[k], (*j)[k]) == false {
			return fmt.Sprintf("for [%s] was [%v] expected [%v] ", k, (*i)[k], (*j)[k])
		}
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////

func T_ExpectResponsablesEmpty(t *testing.T) {
	if responsableGatekeeper.HasResponses() {
		if len(responsableGatekeeper.UserInGroupResponses) > 0 {
			t.Logf("UserInGroupResponses still available")
		}
		if len(responsableGatekeeper.SetPermissionsResponses) > 0 {
			t.Logf("SetPermissionsResponses still available")
		}
		responsableGatekeeper.Reset()
		t.Fail()
	}
}

func Test_TokenUserHasRequestedPermissions_Server(t *testing.T) {
	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: true}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, requestedPermissions) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_Owner(t *testing.T) {
	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "abcdef1234", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, requestedPermissions) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_GatekeeperError(t *testing.T) {
	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{}, errors.New("ERROR")}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err == nil {
		t.Fatalf("Unexpected success")
	}
	if err.Error() != "ERROR" {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if len(permissions) != 0 {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_CompleteMismatch(t *testing.T) {
	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{"y": commonClients.Allowed, "z": commonClients.Allowed}, nil}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if len(permissions) != 0 {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_PartialMismatch(t *testing.T) {
	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{"a": commonClients.Allowed, "z": commonClients.Allowed}, nil}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, commonClients.Permissions{"a": commonClients.Allowed}) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_FullMatch(t *testing.T) {
	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}, nil}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, requestedPermissions) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}
