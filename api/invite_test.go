package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/tidepool-org/go-common/tokens"
)

func initTestingRouterNoPerms() *mux.Router {
	testRtr := mux.NewRouter()
	hydrophone := InitApi(
		FAKE_CONFIG,
		mockStore,
		mockNotifier,
		mock_uid1Shoreline,
		mock_NoPermsGatekeeper,
		mockMetrics,
		mockSeagull,
		mockTemplates,
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

	request, _ := http.NewRequest("POST", fmt.Sprintf("/send/invite/%s", testing_uid2), sendBody)
	request.Header.Set(tokens.TidepoolSessionTokenName, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestGetReceivedInvitations_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request, _ := http.NewRequest("GET", fmt.Sprintf("/invitations/%s", testing_uid2), nil)
	request.Header.Set(tokens.TidepoolSessionTokenName, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestGetSentInvitations_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request, _ := http.NewRequest("GET", fmt.Sprintf("/invite/%s", testing_uid2), nil)
	request.Header.Set(tokens.TidepoolSessionTokenName, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestAcceptInvite_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request, _ := http.NewRequest("PUT", fmt.Sprintf("/accept/invite/%s/%s", testing_uid2, testing_uid1), nil)
	request.Header.Set(tokens.TidepoolSessionTokenName, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestDismissInvite_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request, _ := http.NewRequest("PUT", fmt.Sprintf("/dismiss/invite/%s/%s", testing_uid2, testing_uid1), nil)
	request.Header.Set(tokens.TidepoolSessionTokenName, testing_uid1)
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
		hydrophone := InitApi(
			FAKE_CONFIG,
			mockStore,
			mockNotifier,
			mockShoreline,
			mockGatekeeper,
			mockMetrics,
			mockSeagull,
			mockTemplates,
		)

		//testing when there is nothing to return from the store
		if inviteTest.returnNone {
			hydrophone = InitApi(
				FAKE_CONFIG,
				mockStoreEmpty,
				mockNotifier,
				mockShoreline,
				mockGatekeeper,
				mockMetrics,
				mockSeagull,
				mockTemplates,
			)
		}

		hydrophone.SetHandlers("", testRtr)

		var body = &bytes.Buffer{}
		// build the body only if there is one defined in the test
		if len(inviteTest.body) != 0 {
			json.NewEncoder(body).Encode(inviteTest.body)
		}
		request, _ := http.NewRequest(inviteTest.method, inviteTest.url, body)
		if inviteTest.token != "" {
			request.Header.Set(tokens.TidepoolSessionTokenName, testing_token)
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
