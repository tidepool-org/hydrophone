package api

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gorilla/mux"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/clients/version"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/localize"
	"github.com/tidepool-org/hydrophone/models"
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
		ServerSecret:      "shhh! don't tell",
		I18nTemplatesPath: "../templates",
	}
	/*
	 * basics setup
	 */
	rtr            = mux.NewRouter()
	mockNotifier   = clients.NewMockNotifier()
	mockShoreline  = shoreline.NewMock(testing_token)
	mockGatekeeper = commonClients.NewGatekeeperMock(nil, &status.StatusError{status.NewStatus(500, "Unable to parse response.")})

	mockSeagull = commonClients.NewSeagullMock()

	mockTemplates = models.Templates{}

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

	responsableGatekeeper = NewResponsableMockGatekeeper()
	responsableHydrophone = InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, responsableGatekeeper, mockSeagull, mockTemplates)

	mockLocalizer, _ = localize.NewI18nLocalizer("../templates/locales")
)

// In an effort to mock shoreline so that we can return the token we wish
type testingShorelingMock struct{ userid string }

func newtestingShorelingMock(userid string) *testingShorelingMock {
	return &testingShorelingMock{userid: userid}
}

func (m *testingShorelingMock) Start() error { return nil }
func (m *testingShorelingMock) Close()       { return }
func (m *testingShorelingMock) Login(username, password string) (*shoreline.UserData, string, error) {
	return &shoreline.UserData{UserID: m.userid, Emails: []string{m.userid + "@email.org"}, Username: m.userid + "@email.org"}, "", nil
}
func (m *testingShorelingMock) Signup(username, password, email string) (*shoreline.UserData, error) {
	return &shoreline.UserData{UserID: m.userid, Emails: []string{m.userid + "@email.org"}, Username: m.userid + "@email.org"}, nil
}
func (m *testingShorelingMock) TokenProvide() string { return testing_token }
func (m *testingShorelingMock) GetUser(userID, token string) (*shoreline.UserData, error) {
	return &shoreline.UserData{UserID: m.userid, Emails: []string{m.userid + "@email.org"}, Username: m.userid + "@email.org"}, nil
}
func (m *testingShorelingMock) UpdateUser(userID string, userUpdate shoreline.UserUpdate, token string) error {
	return nil
}
func (m *testingShorelingMock) CheckToken(token string) *shoreline.TokenData {
	return &shoreline.TokenData{UserID: m.userid, IsServer: false}
}

type (
	//common test structure
	toTest struct {
		desc       string
		skip       bool
		returnNone bool
		method     string
		url        string
		body       testJSONObject
		token      string
		respCode   int
		response   testJSONObject
	}
	// These two types make it easier to define blobs of json inline.
	// We don't use the types defined by the API because we want to
	// be able to test with partial data structures.
	// testJSONObject is a generic json object
	testJSONObject map[string]interface{}

	// and ja is a generic json array
	ja []interface{}
)

func TestGetStatus_StatusOk(t *testing.T) {

	version.ReleaseNumber = "1.2.3"
	version.FullCommit = "e0c73b95646559e9a3696d41711e918398d557fb"

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockGatekeeper, mockSeagull, mockTemplates)
	hydrophone.SetHandlers("", rtr)

	hydrophone.GetStatus(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusOK)
	}

	body, _ := ioutil.ReadAll(response.Body)

	if string(body) != `{"status":{"code":200,"reason":"OK"},"version":"1.2.3+e0c73b95646559e9a3696d41711e918398d557fb"}` {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "OK")
	}

}

func TestGetStatus_StatusInternalServerError(t *testing.T) {

	version.ReleaseNumber = "1.2.3"
	version.FullCommit = "e0c73b95646559e9a3696d41711e918398d557fb"

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophoneFails := InitApi(FAKE_CONFIG, mockStoreFails, mockNotifier, mockShoreline, mockGatekeeper, mockSeagull, mockTemplates)
	hydrophoneFails.SetHandlers("", rtr)

	hydrophoneFails.GetStatus(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusInternalServerError)
	}

	body, _ := ioutil.ReadAll(response.Body)

	if string(body) != `{"status":{"code":500,"reason":"Session failure"},"version":"1.2.3+e0c73b95646559e9a3696d41711e918398d557fb"}` {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "Session failure")
	}
}

func (i *testJSONObject) deepCompare(j *testJSONObject) string {
	for k := range *i {
		if reflect.DeepEqual((*i)[k], (*j)[k]) == false {
			return fmt.Sprintf("`%s` expected `%v` actual `%v` ", k, (*j)[k], (*i)[k])
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
		if len(responsableGatekeeper.UsersInGroupResponses) > 0 {
			t.Logf("UsersInGroupResponses still available")
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
