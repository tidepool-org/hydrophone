package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
)

func initTestingRouterNoPerms() *mux.Router {
	testRtr := mux.NewRouter()
	hydrophone := NewApi(
		FAKE_CONFIG,
		nil,
		mockStore,
		mockNotifier,
		mock_uid1Shoreline,
		mock_NoPermsGatekeeper,
		mockMetrics,
		mockSeagull,
		nil,
		mockTemplates,
		zap.NewNop().Sugar(),
	)
	hydrophone.SetHandlers("", testRtr)
	return testRtr
}

func TestSendInvite_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()
	sendBody := &bytes.Buffer{}
	json.NewEncoder(sendBody).Encode(testJSONObject{
		"email": testing_uid2 + "@email.org",
		"permissions": testJSONObject{
			"view": testJSONObject{},
			"note": testJSONObject{},
		},
	})

	request := MustRequest(t, "POST", fmt.Sprintf("/send/invite/%s", testing_uid2), sendBody)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestGetReceivedInvitations_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request := MustRequest(t, "GET", fmt.Sprintf("/invitations/%s", testing_uid2), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestGetSentInvitations_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request := MustRequest(t, "GET", fmt.Sprintf("/invite/%s", testing_uid2), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestAcceptInvite_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request := MustRequest(t, "PUT", fmt.Sprintf("/accept/invite/%s/%s", testing_uid2, testing_uid1), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestDismissInvite_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request := MustRequest(t, "PUT", fmt.Sprintf("/dismiss/invite/%s/%s", testing_uid2, testing_uid1), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestInviteResponds(t *testing.T) {

	inviteTests := []toTest{
		{
			desc:     "can't invite without a body",
			method:   http.MethodPost,
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusBadRequest,
		},
		{
			desc:     "can't invite without permissions",
			method:   http.MethodPost,
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusBadRequest,
			body: testJSONObject{
				"email": "personToInvite@email.com",
			},
		},
		{
			desc:     "can't invite without email",
			method:   http.MethodPost,
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusBadRequest,
			body: testJSONObject{
				"email":       "",
				"permissions": testJSONObject{"view": testJSONObject{}},
			},
		},
		{
			desc:     "can't have a duplicate invite",
			method:   http.MethodPost,
			url:      fmt.Sprintf("/send/invite/%s", testing_uid2),
			token:    testing_token_uid1,
			respCode: http.StatusConflict,
			body: testJSONObject{
				"email": testing_uid2 + "@email.org",
				"permissions": testJSONObject{
					"view": testJSONObject{},
					"note": testJSONObject{},
				},
			},
		},
		{
			desc:       "invite valid if email, permissons and not a duplicate",
			returnNone: true,
			method:     http.MethodPost,
			url:        fmt.Sprintf("/send/invite/%s", testing_uid2),
			token:      testing_token_uid1,
			respCode:   http.StatusOK,
			body: testJSONObject{
				"email":       testing_uid2 + "@email.org",
				"permissions": testJSONObject{"view": testJSONObject{}},
			},
		},
		{
			desc:     "invitations gives list of our outstanding invitations",
			method:   http.MethodGet,
			url:      fmt.Sprintf("/invitations/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			response: testJSONObject{
				"invitedBy": testing_uid2,
				"permissions": testJSONObject{
					"view": testJSONObject{},
					"note": testJSONObject{},
				},
			},
		},
		{
			desc:     "request not found without the full path",
			method:   http.MethodPut,
			url:      "/accept/invite",
			token:    testing_token_uid1,
			respCode: http.StatusNotFound,
		},
		{
			desc:     "invalid request to accept an invite when user ID's not expected",
			method:   http.MethodPut,
			url:      fmt.Sprintf("/accept/invite/%s/%s", testing_uid1, "badID"),
			token:    testing_token_uid1,
			respCode: http.StatusForbidden,
			body: testJSONObject{
				"key": "careteam_invite/1234",
			},
		},
		{
			desc:     "invite will get invitations we sent",
			method:   http.MethodGet,
			url:      fmt.Sprintf("/invite/%s", testing_uid2),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			response: testJSONObject{
				"email": "personToInvite@email.com",
				"permissions": testJSONObject{
					"view": testJSONObject{},
					"note": testJSONObject{},
				},
			},
		},
		{
			desc:     "dismiss an invitation we were sent",
			method:   http.MethodPut,
			url:      fmt.Sprintf("/dismiss/invite/%s/%s", testing_uid2, testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			body: testJSONObject{
				"key": "careteam_invite/1234",
			},
		},
		{
			desc:     "delete the other invitation we sent",
			method:   http.MethodPut,
			url:      fmt.Sprintf("/%s/invited/other@youremail.com", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
		},
	}

	for idx, inviteTest := range inviteTests {
		// don't run a test if it says to skip it
		if inviteTest.skip {
			continue
		}
		var testRtr = mux.NewRouter()

		//default flow, fully authorized
		hydrophone := NewApi(
			FAKE_CONFIG,
			nil,
			mockStore,
			mockNotifier,
			mockShoreline,
			mockGatekeeper,
			mockMetrics,
			mockSeagull,
			nil,
			mockTemplates,
			zap.NewNop().Sugar(),
		)

		//testing when there is nothing to return from the store
		if inviteTest.returnNone {
			hydrophone = NewApi(
				FAKE_CONFIG,
				nil,
				mockStoreEmpty,
				mockNotifier,
				mockShoreline,
				mockGatekeeper,
				mockMetrics,
				mockSeagull,
				nil,
				mockTemplates,
				zap.NewNop().Sugar(),
			)
		}

		hydrophone.SetHandlers("", testRtr)

		var body = &bytes.Buffer{}
		// build the body only if there is one defined in the test
		if len(inviteTest.body) != 0 {
			json.NewEncoder(body).Encode(inviteTest.body)
		}
		request := MustRequest(t, inviteTest.method, inviteTest.url, body)
		if inviteTest.token != "" {
			request.Header.Set(TP_SESSION_TOKEN, testing_token)
		}
		response := httptest.NewRecorder()
		testRtr.ServeHTTP(response, request)

		if response.Code != inviteTest.respCode {
			t.Logf("TestId `%d` `%s` expected `%d` actual `%d`", idx, inviteTest.desc, inviteTest.respCode, response.Code)
			t.Fail()
		}

		if response.Body.Len() != 0 && len(inviteTest.response) != 0 {
			var result = &testJSONObject{}
			err := json.NewDecoder(response.Body).Decode(result)
			if err != nil {
				//TODO: not dealing with arrays at the moment ....
				if err.Error() != "json: cannot unmarshal array into Go value of type api.testJSONObject" {
					t.Logf("TestId `%d` `%s` errored `%s` body `%v`", idx, inviteTest.desc, err.Error(), response.Body)
					t.Fail()
				}
			}

			if cmp := result.deepCompare(&inviteTest.response); cmp != "" {
				t.Logf("TestId `%d` `%s` URL `%s` body `%s`", idx, inviteTest.desc, inviteTest.url, cmp)
				t.Fail()
			}
		}
	}
}

func TestInviteCanAddAlerting(t *testing.T) {
	mockShorelineAlerting := newtestingShorelineMock(testing_uid1, testing_uid2)
	mockStoreAlerting := newMockRecordingStore(mockStoreEmpty, "UpsertConfirmation")
	perms := map[string]commonClients.Permissions{
		key(testing_uid1, testing_uid1): {"root": commonClients.Allowed},
		key(testing_uid2, testing_uid2): {"root": commonClients.Allowed},
		key(testing_uid2, testing_uid1): {"view": commonClients.Allowed},
	}
	mockGatekeeperAlerting := newMockGatekeeperAlerting(perms)
	hydrophone := NewApi(
		FAKE_CONFIG,
		nil,
		mockStoreAlerting,
		mockNotifier,
		mockShorelineAlerting,
		mockGatekeeperAlerting,
		mockMetrics,
		mockSeagull,
		nil,
		mockTemplates,
		zap.NewNop().Sugar(),
	)
	testRtr := mux.NewRouter()
	hydrophone.SetHandlers("", testRtr)
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(map[string]interface{}{
		"email": testing_uid2 + "@email.org",
		"permissions": commonClients.Permissions{
			"view":   commonClients.Allowed,
			"follow": commonClients.Allowed,
		},
	})
	if err != nil {
		t.Fatalf("error creating test request body: %s", err)
	}
	request, err := http.NewRequest(http.MethodPost, "/send/invite/"+testing_uid1, buf)
	if err != nil {
		t.Fatalf("error creating test request: %s", err)
	}
	request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
	response := httptest.NewRecorder()

	testRtr.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status `%d` actual `%d`", http.StatusOK, response.Code)
	}

	for _, call := range mockStoreAlerting.Calls["UpsertConfirmation"] {
		conf, ok := call.(*models.Confirmation)
		if !ok {
			t.Fatalf("expected Confirmation, got %+v", call)
		}
		ctc := &models.CareTeamContext{}
		err := conf.DecodeContext(ctc)
		if err != nil {
			t.Fatalf("error decoding Confirmation Context: %s", err)
		}
		t.Logf("permissions: %+v", ctc.Permissions)
		if ctc.Permissions["view"] == nil {
			t.Fatalf("expected view permissions, got nil")
		}
		if ctc.Permissions["follow"] == nil {
			t.Fatalf("expected follow permissions, got nil")
		}
		for key := range ctc.Permissions {
			if key != "follow" && key != "view" {
				t.Fatalf("expected only follow and view, got %q", key)
			}
		}
	}

}

func TestAcceptInviteAlertsConfigRequiresFollowPerm(t *testing.T) {
	mockShorelineAlerting := newtestingShorelineMock(testing_uid1, testing_uid2)
	mockStoreAlerting := newMockRecordingStore(mockStore, "UpsertConfirmation")
	perms := map[string]commonClients.Permissions{
		key(testing_uid1, testing_uid1): {"root": commonClients.Allowed},
		key(testing_uid2, testing_uid2): {"root": commonClients.Allowed},
	}
	mockGatekeeperAlerting := newMockGatekeeperAlerting(perms)
	hydrophone := NewApi(
		FAKE_CONFIG,
		nil,
		mockStoreAlerting,
		mockNotifier,
		mockShorelineAlerting,
		mockGatekeeperAlerting,
		mockMetrics,
		mockSeagull,
		newMockAlertsClientWithFailingUpsert(),
		mockTemplates,
		zap.NewNop().Sugar(),
	)
	c := &models.Confirmation{
		Key:          testing_uid2,
		Type:         "careteam_invitation",
		Email:        testing_uid2,
		ClinicId:     "",
		CreatorId:    testing_uid1,
		Creator:      models.Creator{},
		Context:      []byte(`{"permissions":{"view":{}},"alertsConfig":{}}`),
		Created:      time.Time{},
		Modified:     time.Time{},
		Status:       "pending",
		Restrictions: &models.Restrictions{},
		UserId:       testing_uid2,
	}
	buf, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("error marshaling confirmation: %s", err)
	}
	body := bytes.NewBuffer(buf)
	testRtr := mux.NewRouter()
	hydrophone.SetHandlers("", testRtr)
	request, err := http.NewRequest(http.MethodPut, "/confirm/accept/invite/"+testing_uid2+"/"+testing_uid1, body)
	if err != nil {
		t.Fatalf("error creating test request: %s", err)
	}
	request.Header.Set(TP_SESSION_TOKEN, testing_uid2)
	response := httptest.NewRecorder()

	testRtr.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status `%d` actual `%d`", http.StatusForbidden, response.Code)
	}

	if numCalls := len(mockStoreAlerting.Calls["UpsertConfirmation"]); numCalls != 0 {
		t.Fatalf("expected 0 calls to UpsertConfirmation, got %d", numCalls)
	}
}

func TestAcceptInviteAlertsConfigOptional(t *testing.T) {
	mockShorelineAlerting := newtestingShorelineMock(testing_uid1, testing_uid2)
	mockStoreAlerting := newMockRecordingStore(mockStore, "UpsertConfirmation")
	perms := map[string]commonClients.Permissions{
		key(testing_uid1, testing_uid1): {"root": commonClients.Allowed},
		key(testing_uid2, testing_uid2): {"root": commonClients.Allowed},
	}
	mockGatekeeperAlerting := newMockGatekeeperAlerting(perms)
	hydrophone := NewApi(
		FAKE_CONFIG,
		nil,
		mockStoreAlerting,
		mockNotifier,
		mockShorelineAlerting,
		mockGatekeeperAlerting,
		mockMetrics,
		mockSeagull,
		newMockAlertsClientWithFailingUpsert(),
		mockTemplates,
		zap.NewNop().Sugar(),
	)
	c := &models.Confirmation{
		Key:          testing_uid2,
		Type:         "careteam_invitation",
		Email:        testing_uid2,
		ClinicId:     "",
		CreatorId:    testing_uid1,
		Creator:      models.Creator{},
		Context:      []byte(`{"permissions":{"view":{}}}`),
		Created:      time.Time{},
		Modified:     time.Time{},
		Status:       "pending",
		Restrictions: &models.Restrictions{},
		UserId:       testing_uid2,
	}
	buf, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("error marshaling confirmation: %s", err)
	}
	body := bytes.NewBuffer(buf)
	testRtr := mux.NewRouter()
	hydrophone.SetHandlers("", testRtr)
	request, err := http.NewRequest(http.MethodPut, "/confirm/accept/invite/"+testing_uid2+"/"+testing_uid1, body)
	if err != nil {
		t.Fatalf("error creating test request: %s", err)
	}
	request.Header.Set(TP_SESSION_TOKEN, testing_uid2)
	response := httptest.NewRecorder()

	testRtr.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status `%d` actual `%d`", http.StatusOK, response.Code)
	}

	numCalls := len(mockStoreAlerting.Calls["UpsertConfirmation"])
	if numCalls != 1 {
		t.Fatalf("expected 1 call to UpsertConfirmation, got %d", numCalls)
	}
}

func TestInviteAddingAlertingMergesPerms(t *testing.T) {
	mockShorelineAlerting := newtestingShorelineMock(testing_uid1, testing_uid2)
	mockStoreAlerting := newMockRecordingStore(mockStoreEmpty, "UpsertConfirmation")
	perms := map[string]commonClients.Permissions{
		key(testing_uid1, testing_uid1): {"root": commonClients.Allowed},
		key(testing_uid2, testing_uid2): {"root": commonClients.Allowed},
		key(testing_uid2, testing_uid1): {"view": commonClients.Allowed, "other": commonClients.Allowed},
	}
	mockGatekeeperAlerting := newMockGatekeeperAlerting(perms)
	hydrophone := NewApi(
		FAKE_CONFIG,
		nil,
		mockStoreAlerting,
		mockNotifier,
		mockShorelineAlerting,
		mockGatekeeperAlerting,
		mockMetrics,
		mockSeagull,
		nil,
		mockTemplates,
		zap.NewNop().Sugar(),
	)
	testRtr := mux.NewRouter()
	hydrophone.SetHandlers("", testRtr)
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(map[string]interface{}{
		"email": testing_uid2 + "@email.org",
		"permissions": commonClients.Permissions{
			"view":   commonClients.Allowed,
			"follow": commonClients.Allowed,
		},
	})
	if err != nil {
		t.Fatalf("error creating test request body: %s", err)
	}
	request, err := http.NewRequest(http.MethodPost, "/send/invite/"+testing_uid1, buf)
	if err != nil {
		t.Fatalf("error creating test request: %s", err)
	}
	request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
	response := httptest.NewRecorder()
	testRtr.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status `%d` actual `%d`", http.StatusOK, response.Code)
	}

	for _, call := range mockStoreAlerting.Calls["UpsertConfirmation"] {
		conf, ok := call.(*models.Confirmation)
		if !ok {
			t.Fatalf("expected Confirmation, got %+v", call)
		}
		ctc := &models.CareTeamContext{}
		err := conf.DecodeContext(ctc)
		if err != nil {
			t.Fatalf("error decoding Confirmation Context: %s", err)
		}
		t.Logf("permissions: %+v", ctc.Permissions)
		if ctc.Permissions["view"] == nil {
			t.Fatalf("expected view permissions, got nil")
		}
		if ctc.Permissions["follow"] == nil {
			t.Fatalf("expected alerting permissions, got nil")
		}
		if ctc.Permissions["other"] == nil {
			t.Fatalf("expected other permissions, got nil")
		}
		for key := range ctc.Permissions {
			if key != "follow" && key != "view" && key != "other" {
				t.Fatalf("expected only follow, view, and other, got %q", key)
			}
		}
	}
}

// mockRecordingStore can record the arguments passed to its methods.
//
// These arguments can be checked for testing.
type mockRecordingStore struct {
	clients.StoreClient
	Calls map[string][]interface{}
}

func newMockRecordingStore(store clients.StoreClient, calls ...string) *mockRecordingStore {
	callsToRecord := map[string][]interface{}{}
	for _, call := range calls {
		callsToRecord[call] = []interface{}{}
	}
	return &mockRecordingStore{
		StoreClient: store,
		Calls:       callsToRecord,
	}
}

func (r *mockRecordingStore) UpsertConfirmation(ctx context.Context, confirmation *models.Confirmation) error {
	if recordings := r.Calls["UpsertConfirmation"]; recordings != nil {
		r.Calls["UpsertConfirmation"] = append(recordings, confirmation)
		return nil
	}
	return r.StoreClient.UpsertConfirmation(ctx, confirmation)
}

// mockGatekeeperAlerting extends GatekeeperMock with permissions for multiple
// mocked users.
type mockGatekeeperAlerting struct {
	*commonClients.GatekeeperMock
	perms map[string]commonClients.Permissions
}

func newMockGatekeeperAlerting(perms map[string]commonClients.Permissions) *mockGatekeeperAlerting {
	return &mockGatekeeperAlerting{
		GatekeeperMock: commonClients.NewGatekeeperMock(nil, nil),
		perms:          perms,
	}
}

func (g *mockGatekeeperAlerting) UserInGroup(userID string, groupID string) (commonClients.Permissions, error) {
	if perms, ok := g.perms[key(userID, groupID)]; ok {
		return perms, nil
	}
	return g.GatekeeperMock.UserInGroup(userID, groupID)
}

// key is helper for generating mockGatekeeperAlerting map keys.
func key(userID, groupID string) string {
	return userID + ":" + groupID
}

func TestInviteBodyParsing(t *testing.T) {
	rawBody := []byte(`{
  "alertsConfig": {
    "urgentLow": {
      "threshold": {
        "units": "mg/dL",
        "value": 100
      },
      "repeat": 0,
      "enabled": true
    }
  },
  "permissions": {
    "view": {}
  },
  "email": "foo@example.com",
  "nickname": "whatthefoo?"
}`)
	ib := &inviteBody{}
	err := json.Unmarshal(rawBody, ib)
	if err != nil {
		t.Fatalf("expected nil, got %+v", err)
	}

	if ib.Email != "foo@example.com" {
		t.Fatalf("expected foo@example.com, got %q", ib.Email)
	}

	if ib.Permissions == nil {
		t.Fatalf("expected permissions, got nil")
	} else if ib.Permissions["view"] == nil {
		t.Fatalf("expected view permissions, got nil")
	}

	if ib.AlertsConfig == nil {
		t.Fatalf("expected alerts config, got nil")
	} else if ib.AlertsConfig.UrgentLow.Threshold.Value != 100 {
		t.Fatalf("expected urgent low threshold of 100, got %f", ib.AlertsConfig.UrgentLow.Threshold.Value)
	} else if !ib.AlertsConfig.UrgentLow.Enabled {
		t.Fatalf("expected true, got %t", ib.AlertsConfig.UrgentLow.Enabled)
	}

	if ib.Nickname == nil || *ib.Nickname != "whatthefoo?" {
		t.Fatalf("expected whatthefoo?, got %+v", ib.Nickname)
	}
}
