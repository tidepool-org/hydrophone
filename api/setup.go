package api

import (
	"github.com/gorilla/mux"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/clients"
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
		ServerSecret: "shhh! don't tell",
	}

	/*
	 * basics setup
	 */
	rtr            = mux.NewRouter()
	mockNotifier   = clients.NewMockNotifier()
	mockShoreline  = shoreline.NewMock(testing_token)
	mockGatekeeper = commonClients.NewGatekeeperMock(nil, &status.StatusError{Status: status.NewStatus(500, "Unable to parse response.")})
	mockMetrics    = highwater.NewMock()
	mockSeagull    = commonClients.NewSeagullMock()
	mockTemplates  = models.Templates{}

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

	mock_uid1Shoreline = newtestingShorelingMock(testing_uid1)

	responsableGatekeeper = NewResponsableMockGatekeeper()
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
