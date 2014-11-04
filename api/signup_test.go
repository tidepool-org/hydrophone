package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestSignupResponds(t *testing.T) {

	tests := []toTest{
		{
			// if you leave off the userid, it fails
			method:   "POST",
			url:      "/send/signup",
			token:    TOKEN_FOR_UID1,
			respCode: 404,
		},
		{
			// first time you ask, it does it
			returnNone: true,
			method:     "POST",
			url:        "/send/signup/NewUserID",
			token:      TOKEN_FOR_UID1,
			respCode:   200,
		},
		{
			// need a token
			returnNone: true,
			method:     "POST",
			url:        "/send/signup/NewUserID",
			respCode:   401,
		},
		{
			// second time you ask, it fails with a limit
			method:   "POST",
			url:      "/send/signup/NewUserID",
			token:    TOKEN_FOR_UID1,
			respCode: 403,
		},
		{
			// can't resend a signup if you didn't send it
			method:   "POST",
			url:      "/resend/signup",
			token:    TOKEN_FOR_UID1,
			respCode: 404,
		},
		{
			// no token is no good
			method:   "POST",
			url:      "/resend/signup/UID",
			respCode: 401,
		},
		{
			// but you can resend a valid one
			method:   "POST",
			url:      "/resend/signup/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 200,
		},
		{
			// no token is no good
			method:   "PUT",
			url:      "/accept/signup/UID2/UID",
			respCode: 401,
		},
		{
			// you can't accept an invitation you didn't get
			returnNone: true,
			method:     "PUT",
			url:        "/accept/signup/UID2/UIDBad",
			token:      TOKEN_FOR_UID2,
			respCode:   404,
		},
		{
			// you can accept an invitation from another user key and userid wont give match
			returnNone: true,
			method:     "PUT",
			url:        "/accept/signup/UID2/UID",
			token:      TOKEN_FOR_UID2,
			respCode:   404,
		},
		{
			// all good
			method:   "PUT",
			url:      "/accept/signup/UID2/UIDBad",
			token:    TOKEN_FOR_UID2,
			respCode: 200,
		},
		{
			method:   "PUT",
			url:      "/dismiss/signup/UID",
			respCode: 200,
			body: jo{
				"key": "1234-xXd",
			},
		},
		{
			//when no key
			method:   "PUT",
			url:      "/dismiss/signup/UID",
			respCode: 400,
		},
		{
			skip:     true,
			method:   "GET",
			url:      "/signup/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 200,
		},
		{
			skip:     true,
			method:   "DELETE",
			url:      "/signup/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 200,
		},
	}

	for idx, test := range tests {

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
