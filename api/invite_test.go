package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestInviteResponds(t *testing.T) {

	tests := []toTest{
		{
			// can't invite without a body
			method:   "POST",
			url:      "/send/invite/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 400,
		},
		{
			// can't invite without permissions
			method:   "POST",
			url:      "/send/invite/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 400,
			body:     jo{"email": "personToInvite@email.com"},
		},
		{
			// can't invite without email
			method:   "POST",
			url:      "/send/invite/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 400,
			body: jo{
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// if dup invite
			method:   "POST",
			url:      "/send/invite/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 409,
			body: jo{
				"email": "personToInvite@email.com",
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
			url:        "/send/invite/UID",
			token:      TOKEN_FOR_UID1,
			respCode:   200,
			body: jo{
				"email": "otherToInvite@email.com",
				"permissions": jo{
					"view": jo{},
					"note": jo{},
				},
			},
		},
		{
			// we should get a list of our outstanding invitations
			method:   "GET",
			url:      "/invitations/UID",
			token:    TOKEN_FOR_UID1,
			respCode: http.StatusOK,
			response: jo{
				"invitedBy": "UID",
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
			token:    TOKEN_FOR_UID1,
			respCode: http.StatusNotFound,
		},
		{
			// no token
			method:   "PUT",
			url:      "/accept/invite/UID2/UID",
			respCode: http.StatusUnauthorized,
		},
		{
			// we can accept an invitation we did get
			method:   "PUT",
			url:      "/accept/invite/UID1/UID",
			token:    TOKEN_FOR_UID1,
			respCode: http.StatusOK,
			body: jo{
				"key": "careteam_invite/1234",
			},
		},
		{
			// get invitations we sent
			method:   "GET",
			url:      "/invite/UID2",
			token:    TOKEN_FOR_UID1,
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
			url:      "/dismiss/invite/UID2/UID",
			token:    TOKEN_FOR_UID1,
			respCode: http.StatusOK,
			body: jo{
				"key": "careteam_invite/1234",
			},
		},
		{
			// delete the other invitation we sent
			method:   "PUT",
			url:      "/UID/invited/other@youremail.com",
			token:    TOKEN_FOR_UID1,
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

		if test.returnNone {
			hydrophoneFindsNothing := InitApi(FAKE_CONFIG, mockStoreEmpty, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull)
			hydrophoneFindsNothing.SetHandlers("", testRtr)
		} else {
			hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull)
			hydrophone.SetHandlers("", testRtr)
		}

		var body = &bytes.Buffer{}
		// build the body only if there is one defined in the test
		if len(test.body) != 0 {
			json.NewEncoder(body).Encode(test.body)
		}
		request, _ := http.NewRequest(test.method, test.url, body)
		if test.token != "" {
			request.Header.Set(TP_SESSION_TOKEN, FAKE_TOKEN)
		}
		response := httptest.NewRecorder()
		testRtr.ServeHTTP(response, request)

		if response.Code != test.respCode {
			t.Fatalf("Test %d url: '%s'\nNon-expected status code %d (expected %d):\n\tbody: %v",
				idx, test.url, response.Code, test.respCode, response.Body)
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
