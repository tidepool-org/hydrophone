package api

import (
	"fmt"
	"strings"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"

	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
)

const (
	make_store_return_nothing = true

	testing_token = "a.fake.token.to.use.in.tests"

	testing_token_uid1 = "a.fake.token.for.uid.1"
	testing_uid1       = "UID123"

	testing_uid2 = "UID999"
)

var (
	NO_PARAMS = map[string]string{}

	FAKE_CONFIG = Config{
		ServerSecret: "shhh! don't tell",
	}

	/*
	 * basics setup
	 */
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

	/*
	 * users permissons scenarios
	 */
	mock_NoPermsGatekeeper = commonClients.NewGatekeeperMock(commonClients.Permissions{"upload": commonClients.Permission{"userid": "other-id"}}, nil)

	mock_uid1Shoreline = newtestingShorelineMock(testing_uid1)
)

// In an effort to mock shoreline so that we can return the token we wish
type testingShorelineMock struct{ userIDs []string }

func (m *testingShorelineMock) CreateCustodialUserForClinic(clinicId string, userData shoreline.CustodialUserData, token string) (*shoreline.UserData, error) {
	panic("Not Implemented")
}

func (m *testingShorelineMock) DeleteUserSessions(userID, token string) error {
	panic("Not Implemented")
}

func newtestingShorelineMock(userIDs ...string) *testingShorelineMock {
	return &testingShorelineMock{userIDs: userIDs}
}

func (m *testingShorelineMock) Start() error { return nil }
func (m *testingShorelineMock) Close()       {}
func (m *testingShorelineMock) Login(userID, password string) (*shoreline.UserData, string, error) {
	for _, mockedUserID := range m.userIDs {
		if mockedUserID == userID {
			return m.mockUser(mockedUserID), "", nil
		}
	}
	return nil, "", fmt.Errorf("userID not mocked: %q", userID)
}

func (m *testingShorelineMock) mockUser(userID string) *shoreline.UserData {
	return &shoreline.UserData{
		UserID:   userID,
		Emails:   []string{userID + "@email.org"},
		Username: userID + "@email.org",
	}
}

func (m *testingShorelineMock) Signup(userID, password, email string) (*shoreline.UserData, error) {
	for _, mockedUserID := range m.userIDs {
		if mockedUserID == userID {
			return m.mockUser(userID), nil
		}
	}
	return nil, fmt.Errorf("userID not mocked: %q", userID)
}

func (m *testingShorelineMock) TokenProvide() string { return testing_token }

func (m *testingShorelineMock) GetUser(userID, token string) (*shoreline.UserData, error) {
	if prefix, _, found := strings.Cut(userID, "@"); found {
		userID = prefix
	}
	for _, mockedUserID := range m.userIDs {
		if mockedUserID == userID {
			return m.mockUser(userID), nil
		}
	}
	return nil, fmt.Errorf("userID not mocked: %q", userID)
}

func (m *testingShorelineMock) UpdateUser(userID string, userUpdate shoreline.UserUpdate, token string) error {
	return nil
}

func (m *testingShorelineMock) CheckToken(token string) *shoreline.TokenData {
	for _, mockedUserID := range m.userIDs {
		if mockedUserID == token || (token == testing_token_uid1 && mockedUserID == testing_uid1) {
			return &shoreline.TokenData{UserID: mockedUserID, IsServer: false}
		}
	}
	return nil
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
)
