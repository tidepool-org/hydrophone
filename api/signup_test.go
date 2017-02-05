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
			token:    testing_token_uid1,
			respCode: 404,
		},
		{
			// first time you ask, it does it
			returnNone: true,
			method:     "POST",
			url:        "/send/signup/NewUserID",
			token:      testing_token_uid1,
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
			token:    testing_token_uid1,
			respCode: 403,
		},
		{
			// can't resend a signup if you didn't send it
			method:   "POST",
			url:      "/resend/signup",
			respCode: 404,
		},
		{
			// no token is all good
			method:   "POST",
			url:      "/resend/signup/email@address.org",
			respCode: 200,
		},
		{
			// you can't accept an invitation you didn't get
			returnNone: true,
			method:     "PUT",
			url:        "/accept/signup/UID2/UIDBad",
			respCode:   404,
		},
		{
			// you can accept an invitation from another user key and userid wont give match
			returnNone: true,
			method:     "PUT",
			url:        "/accept/signup/UID2/UID",
			respCode:   404,
		},
		{
			// failure - user does not yet have a password; no body
			method:   "PUT",
			url:      "/accept/signup/WithoutPassword",
			respCode: 409,
			response: jo{
				"code":   float64(409),
				"error":  float64(1001),
				"reason": "User does not have a password",
			},
		},
		{
			// failure - user does not yet have a password; password missing, but birthday correct
			method: "PUT",
			url:    "/accept/signup/WithoutPassword",
			body: jo{
				"birthday": "2016-01-01",
			},
			respCode: 409,
			response: jo{
				"code":   float64(409),
				"error":  float64(1002),
				"reason": "Password is missing",
			},
		},
		{
			// failure - user does not yet have a password; password invalid, but birthday correct
			method: "PUT",
			url:    "/accept/signup/WithoutPassword",
			body: jo{
				"password": "1234",
				"birthday": "2016-01-01",
			},
			respCode: 409,
			response: jo{
				"code":   float64(409),
				"error":  float64(1003),
				"reason": "Password specified is invalid",
			},
		},
		{
			// failure - user does not yet have a password; password valid and birthday missing
			method: "PUT",
			url:    "/accept/signup/WithoutPassword",
			body: jo{
				"password": "12345678",
			},
			respCode: 409,
			response: jo{
				"code":   float64(409),
				"error":  float64(1004),
				"reason": "Birthday is missing",
			},
		},
		{
			// failure - user does not yet have a password; password valid and birthday invalid
			method: "PUT",
			url:    "/accept/signup/WithoutPassword",
			body: jo{
				"password": "12345678",
				"birthday": "aaaaaaaa",
			},
			respCode: 409,
			response: jo{
				"code":   float64(409),
				"error":  float64(1005),
				"reason": "Birthday specified is invalid",
			},
		},
		{
			// failure - user does not yet have a password; password valid and birthday not correct
			method: "PUT",
			url:    "/accept/signup/WithoutPassword",
			body: jo{
				"password": "12345678",
				"birthday": "2015-12-31",
			},
			respCode: 409,
			response: jo{
				"code":   float64(409),
				"error":  float64(1006),
				"reason": "Birthday specified does not match patient birthday",
			},
		},
		{
			// all good - user does not yet have a password; password valid and birthday correct
			method: "PUT",
			url:    "/accept/signup/WithoutPassword",
			body: jo{
				"password": "12345678",
				"birthday": "2016-01-01",
			},
			respCode: 200,
		},
		{
			// all good
			method:   "PUT",
			url:      "/accept/signup/UID",
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
			method:   "PUT",
			url:      "/signup/UID",
			respCode: 200,
			body: jo{
				"key": "1234-xXd",
			},
		},
		{
			method:   "PUT",
			url:      "/signup/UID",
			respCode: 400,
		},
		{
			method:   "GET",
			url:      "/signup/UID",
			token:    testing_token_uid1,
			respCode: 200,
		},
		{
			//no confirmtaions
			returnNone: true,
			method:     "GET",
			url:        "/signup/UID",
			token:      testing_token_uid1,
			respCode:   404,
		},
		{
			//no token is no good
			method:   "GET",
			url:      "/signup/UID",
			respCode: 401,
		},
	}

	for idx, test := range tests {

		if test.skip {
			continue
		}

		//fresh each time
		var testRtr = mux.NewRouter()

		if test.returnNone {
			hydrophoneFindsNothing := InitApi(FAKE_CONFIG, mockStoreEmpty, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull, mockTemplates)
			hydrophoneFindsNothing.SetHandlers("", testRtr)
		} else {
			hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull, mockTemplates)
			hydrophone.SetHandlers("", testRtr)
		}

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
