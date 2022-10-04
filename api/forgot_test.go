package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mdblp/go-common/clients/auth"
	"github.com/mdblp/hydrophone/templates"
	"github.com/mdblp/seagull/schema"
	"github.com/mdblp/shoreline/token"
	"github.com/stretchr/testify/mock"
)

func TestForgotResponds(t *testing.T) {

	tests := []toTest{
		{
			// return 400 if the email is not provided
			method:   "POST",
			url:      "/send/forgot/",
			respCode: 404,
		},
		{
			// always returns a 200 if properly formed
			// no language header -> default to EN
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Password reset instructions",
			respCode:     200,
		},
		{
			// testing language preferences
			// follow standard header -> EN
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Password reset instructions",
			customHeaders: map[string]string{
				"Accept-Language": "en",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// follow standard header -> FR
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Réinitialisation du mot de passe",
			customHeaders: map[string]string{
				"Accept-Language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// standard header not supported language -> EN
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Password reset instructions",
			customHeaders: map[string]string{
				"Accept-Language": "gr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// follow custom header -> FR
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Réinitialisation du mot de passe",
			customHeaders: map[string]string{
				"x-tidepool-language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header not supported language -> EN
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Password reset instructions",
			customHeaders: map[string]string{
				"x-tidepool-language": "gr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header takes precedence over standard -> FR
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Réinitialisation du mot de passe",
			customHeaders: map[string]string{
				"Accept-Language": "en", "x-tidepool-language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header takes precedence over standard but language does not exist -> EN
			method:       "POST",
			url:          "/send/forgot/me@myemail.com",
			emailSubject: "Password reset instructions",
			customHeaders: map[string]string{
				"Accept-Language": "en", "x-tidepool-language": "gr",
			},
			respCode: 200,
		},
		{
			// always returns a 200 for patient
			// with shortKey
			method:   "POST",
			url:      "/send/forgot/patient@myemail.com",
			respCode: 200,
		},
		{
			// always returns a 200 for patient
			// with info
			method:   "POST",
			url:      "/send/forgot/patient@myemail.com?info=ok",
			respCode: 200,
		},
		{
			// returns a 403 for clinician
			method:   "POST",
			url:      "/send/forgot/clinic@myemail.com",
			respCode: 403,
		},
		{
			// returns a 403 for caregivers
			method:   "POST",
			url:      "/send/forgot/caregiver@myemail.com",
			respCode: 403,
		},
		{
			// always returns a 200 if properly formed
			returnNone: true,
			method:     "POST",
			url:        "/send/forgot/me@myemail.com",
			respCode:   200,
		},
		{
			// testing too many forgot email sent
			method:                     "POST",
			url:                        "/send/forgot/patient@myemail.com",
			respCode:                   403,
			counterLatestConfirmations: 11,
		},
		{
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 400,
			body: testJSONObject{
				"key":      "1234_aK3yxxx123",
				"email":    "me@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
		{
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 200,
			body: testJSONObject{
				"shortkey": "12345678",
				"email":    "patient@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
		// returns a 403 for caregivers
		{
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 403,
			body: testJSONObject{
				"shortkey": "12345678",
				"email":    "caregiver@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
		{
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 404,
			body: testJSONObject{
				"shortkey": "00000000",
				"email":    "expired@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
		{
			method:   "PUT",
			url:      "/accept/forgot",
			respCode: 404,
			body: testJSONObject{
				"shortkey": "11111111",
				"email":    "doesnotexist@myemail.com",
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
				"shortkey": "1234_no_match",
				"email":    "me@myemail.com",
				"password": "myN3wpa55w0rd",
			},
		},
		{
			// no key in the payload
			returnNone: true,
			method:     "PUT",
			url:        "/accept/forgot",
			respCode:   400,
			body: testJSONObject{
				"email": "patient@myemail.com",
			},
		},
	}

	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{IsServer: false})

	for idx, test := range tests {

		//fresh each time
		var testRtr = mux.NewRouter()

		mockSeagull.On("GetCollections", "me@myemail.com", []string{"preferences"}).Return(&schema.SeagullDocument{}, nil)
		mockSeagull.On("GetCollections", "patient@myemail.com", []string{"preferences"}).Return(&schema.SeagullDocument{}, nil)
		mockSeagull.On("GetCollections", "clinic@myemail.com", []string{"preferences"}).Return(&schema.SeagullDocument{}, nil)
		mockSeagull.On("GetCollections", "expires@myemail.com", []string{"preferences"}).Return(&schema.SeagullDocument{}, nil)
		mockShoreline.On("TokenProvide").Return(testing_token)

		if test.returnNone {
			mockStoreEmpty.CounterLatestConfirmations = test.counterLatestConfirmations
			hydrophoneFindsNothing := InitApi(FAKE_CONFIG, mockStoreEmpty, mockNotifier, mockShoreline, mockPerms, mockAuth, mockSeagull, mockPortal, mockTemplates, logger)
			hydrophoneFindsNothing.SetHandlers("", testRtr)
		} else {
			mockStore.CounterLatestConfirmations = test.counterLatestConfirmations
			hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockPerms, mockAuth, mockSeagull, mockPortal, mockTemplates, logger)
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
		if test.customHeaders != nil {
			for header, value := range test.customHeaders {
				request.Header.Set(header, value)
			}
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

		if test.emailSubject != "" {
			if emailSubjectSent := mockNotifier.GetLastEmailSubject(); emailSubjectSent != test.emailSubject {
				t.Fatalf("Test %d url: '%s'\nNon-expected email subject %s (expected %s)",
					idx, test.url, emailSubjectSent, test.emailSubject)
			}
		}
	}
	mockAuth = auth.NewMock()
}
