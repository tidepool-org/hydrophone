package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/tidepool-org/hydrophone/testutil"
)

func TestForgotResponds(t *testing.T) {

	tests := []toTest{
		{
			// always returns a 200 if properly formed
			method:   "POST",
			url:      "/send/forgot/me@myemail.com",
			respCode: 200,
		},
		{
			// always returns a 200 if properly formed
			returnNone: true,
			method:     "POST",
			url:        "/send/forgot/me@myemail.com",
			respCode:   200,
		},
		{
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 200,
			body: testJSONObject{
				"key":      "1234_aK3yxxx123",
				"email":    "me@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
		{
			//no data given
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 400,
		},
		{
			//no match found
			returnNone: true,
			method:     "PUT",
			url:        "/accept/forgot",
			respCode:   404,
			body: testJSONObject{
				"key":      "1234_no_match",
				"email":    "me@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
	}

	logger := testutil.NewLogger(t)
	for idx, test := range tests {

		//fresh each time
		var testRtr = mux.NewRouter()

		store := mockStore
		if test.returnNone {
			store = mockStoreEmpty
		}

		hydrophone := NewApi(FAKE_CONFIG, nil, store, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull, nil, mockTemplates, logger)
		hydrophone.SetHandlers("", testRtr)

		var body = &bytes.Buffer{}
		// build the body only if there is one defined in the test
		if len(test.body) != 0 {
			json.NewEncoder(body).Encode(test.body)
		}
		request := MustRequest(t, test.method, test.url, body)
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
			var result = &testJSONObject{}

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
