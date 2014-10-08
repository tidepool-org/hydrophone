package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"./../clients"
	"./../models"

	"github.com/gorilla/mux"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

const (
	MAKE_IT_FAIL   = true
	RETURN_NOTHING = true
	FAKE_TOKEN     = "a.fake.token.to.use.in.tests"
	TOKEN_FOR_UID1 = "a.fake.token.for.uid.1"
	TOKEN_FOR_UID2 = "a.fake.token.for.uid.2"
)

var (
	NO_PARAMS = map[string]string{}

	FAKE_CONFIG = Config{
		ServerSecret: "shhh! don't tell",
		Templates: &models.TemplateConfig{
			PasswordReset:  `{{define "reset_test"}} {{ .UserId }} {{ .Key }} {{end}}{{template "reset_test" .}}`,
			CareteamInvite: `{{define "invite_test"}} {{ .UserId }} {{ .Key }} {{end}}{{template "invite_test" .}}`,
			Confirmation:   `{{define "confirm_test"}} {{ .UserId }} {{ .Key }} {{end}}{{template "confirm_test" .}}`,
		},
	}
	/*
	 * basics setup
	 */
	rtr            = mux.NewRouter()
	mockNotifier   = clients.NewMockNotifier()
	mockShoreline  = shoreline.NewMock(FAKE_TOKEN)
	mockGatekeeper = commonClients.NewGatekeeperMock(nil, nil)
	mockMetrics    = highwater.NewMock()
	mockSeagull    = commonClients.NewSeagullMock(`{}`, nil)
	/*
	 * stores
	 */
	mockStore      = clients.NewMockStoreClient(false, false)
	mockStoreEmpty = clients.NewMockStoreClient(RETURN_NOTHING, false)
	mockStoreFails = clients.NewMockStoreClient(false, MAKE_IT_FAIL)
)

func TestGetStatus_StatusOk(t *testing.T) {

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull)
	hydrophone.SetHandlers("", rtr)

	hydrophone.GetStatus(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Resp given [%s] expected [%s] ", response.Code, http.StatusOK)
	}

}

func TestGetStatus_StatusInternalServerError(t *testing.T) {

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophoneFails := InitApi(FAKE_CONFIG, mockStoreFails, mockNotifier, mockShoreline, mockGatekeeper, mockMetrics, mockSeagull)
	hydrophoneFails.SetHandlers("", rtr)

	hydrophoneFails.GetStatus(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Resp given [%s] expected [%s] ", response.Code, http.StatusInternalServerError)
	}

	body, _ := ioutil.ReadAll(response.Body)

	if string(body) != "Session failure" {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "Session failure")
	}
}

// These two types make it easier to define blobs of json inline.
// We don't use the types defined by the API because we want to
// be able to test with partial data structures.
// jo is a generic json object
type jo map[string]interface{}

// and ja is a generic json array
type ja []interface{}

func (i *jo) deepCompare(j *jo) string {
	for k, _ := range *i {
		if (*i)[k] != (*j)[k] {
			return fmt.Sprintf("Failed comparing field %s", k)
		}
	}
	return ""
}

func TestAddressResponds(t *testing.T) {

	type toTest struct {
		skip       bool
		returnNone bool
		method     string
		url        string
		body       jo
		token      string
		respCode   int
		response   jo
	}

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
			url:      "/invitations/me@myemail.com",
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
			respCode: http.StatusNoContent,
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
		{
			// if you leave off the userid, it fails
			skip:     true,
			method:   "POST",
			url:      "/send/signup",
			token:    TOKEN_FOR_UID1,
			respCode: 404,
		},
		{
			// first time you ask, it does it
			skip:     true,
			method:   "POST",
			url:      "/send/signup/NewUserID",
			token:    TOKEN_FOR_UID1,
			respCode: 200,
		},
		{
			// second time you ask, it fails with a limit
			skip:     true,
			method:   "POST",
			url:      "/send/signup/NewUserID",
			token:    TOKEN_FOR_UID1,
			respCode: 403,
		},
		{
			// can't resend a signup if you didn't send it
			skip:     true,
			method:   "POST",
			url:      "/resend/signup/BadUID",
			token:    TOKEN_FOR_UID1,
			respCode: 404,
		},
		{
			// but you can resend a valid one
			skip:     true,
			method:   "POST",
			url:      "/resend/signup/UID",
			token:    TOKEN_FOR_UID1,
			respCode: 200,
		},
		{
			// you can't accept an invitation you didn't get
			skip:     true,
			method:   "PUT",
			url:      "/accept/signup/UID2/UIDBad",
			token:    TOKEN_FOR_UID2,
			respCode: 200,
		},
		{
			// you can accept an invitation from another user
			skip:     true,
			method:   "PUT",
			url:      "/accept/signup/UID2/UID",
			token:    TOKEN_FOR_UID2,
			respCode: 200,
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
			method:   "PUT",
			url:      "/dismiss/signup/UID",
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
		{
			// always returns a 200 if properly formed
			skip:     true,
			method:   "POST",
			url:      "/send/forgot/me@myemail.com",
			respCode: 200,
		},
		{
			skip:     true,
			method:   "PUT",
			url:      "/accept/forgot",
			token:    TOKEN_FOR_UID1,
			respCode: 200,
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
				log.Printf("Err decoding nonempty response body: %v\n", err)
				return
			}

			if cmp := result.deepCompare(&test.response); cmp != "" {
				t.Fatalf("Test %d url: '%s'\n\t%s\n", idx, test.url, cmp)
			}
		}
	}
}
