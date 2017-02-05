package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
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
	json.NewEncoder(sendBody).Encode(jo{
		"email": testing_uid2 + "@email.org",
		"permissions": jo{
			"view": jo{},
			"note": jo{},
		},
	})

	request, _ := http.NewRequest("POST", fmt.Sprintf("/send/invite/%s", testing_uid2), sendBody)
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

	request, _ := http.NewRequest("GET", fmt.Sprintf("/invitations/%s", testing_uid2), nil)
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

	request, _ := http.NewRequest("GET", fmt.Sprintf("/invite/%s", testing_uid2), nil)
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

	request, _ := http.NewRequest("PUT", fmt.Sprintf("/accept/invite/%s/%s", testing_uid2, testing_uid1), nil)
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

	request, _ := http.NewRequest("PUT", fmt.Sprintf("/dismiss/invite/%s/%s", testing_uid2, testing_uid1), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestInviteResponds(t *testing.T) {

	tests := []toTest{
		{
			// can't invite without a body
			method:   "POST",
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: 400,
		},
		{
			// can't invite without permissions
			method:   "POST",
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: 400,
			body:     jo{"email": "personToInvite@email.com"},
		},
		{
			// can't invite without email
			method:   "POST",
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusBadRequest,
			body: jo{
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// if duplicate invite
			method:   "POST",
			url:      fmt.Sprintf("/send/invite/%s", testing_uid2),
			token:    testing_token_uid1,
			respCode: http.StatusConflict,
			body: jo{
				"email": testing_uid2 + "@email.org",
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// but if you have them all, it should work
			returnNone: true,
			method:     "POST",
			url:        fmt.Sprintf("/send/invite/%s", testing_uid2),
			token:      testing_token_uid1,
			respCode:   http.StatusOK,
			body: jo{
				"email": testing_uid2 + "@email.org",
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// we should get a list of our outstanding invitations
			method:   "GET",
			url:      fmt.Sprintf("/invitations/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			response: jo{
				"invitedBy": testing_uid2,
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// not found without the full path
			method:   "PUT",
			url:      "/accept/invite",
			token:    testing_token_uid1,
			respCode: http.StatusNotFound,
		},
		{
			// we can accept an invitation we did get
			method:   "PUT",
			url:      fmt.Sprintf("/accept/invite/%s/%s", testing_uid1, testing_uid2),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			body: jo{
				"key": "careteam_invite/1234",
			},
		},
		{
			// get invitations we sent
			method:   "GET",
			url:      fmt.Sprintf("/invite/%s", testing_uid2),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			response: jo{
				"email": "personToInvite@email.com",
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// dismiss an invitation we were sent
			method:   "PUT",
			url:      fmt.Sprintf("/dismiss/invite/%s/%s", testing_uid2, testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			body: jo{
				"key": "careteam_invite/1234",
			},
		},
		{
			// delete the other invitation we sent
			method:   "PUT",
			url:      fmt.Sprintf("%s/invited/other@youremail.com", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusOK,
		},
	}

	for idx, test := range tests {
		// don't run a test if it says to skip it
		if test.skip {
			continue
		}
		//fresh each time
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
		if test.returnNone {
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
		if len(test.body) != 0 {
			json.NewEncoder(body).Encode(test.body)
		}
		request, _ := http.NewRequest(test.method, test.url, body)
		if test.token != "" {
			request.Header.Set(TP_SESSION_TOKEN, testing_token)
		}
		response := httptest.NewRecorder()
		testRtr.ServeHTTP(response, request)

		if response.Code != test.respCode {
			t.Logf("TestId %d expected %d actual %d", idx, test.respCode, response.Code)
			t.Fail()
		}

		if response.Body.Len() != 0 && len(test.response) != 0 {
			// compare bodies by comparing the unmarshalled JSON results
			var result = &jo{}

			if err := json.NewDecoder(response.Body).Decode(result); err != nil {
				t.Logf("Err decoding nonempty response body: [%v]\n [%v]\n", err, response.Body)
				return
			}

			if cmp := result.deepCompare(&test.response); cmp != "" {
				t.Fatalf("Test %d url: '%s'\n\t%s\n", idx, test.url, cmp)
			}
		}
	}
}
