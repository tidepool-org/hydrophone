package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mdblp/hydrophone/templates"
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
			// no language header -> default to EN
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/NewUserID",
			token:        testing_token_uid1,
			emailSubject: "Verify your email address",
			respCode:     200,
		},
		{
			// testing language preferences
			// follow standard header -> EN
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/EnglishUserID",
			token:        testing_token_uid1,
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"Accept-Language": "en",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// follow standard header -> FR
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/FrenchUserID",
			token:        testing_token_uid1,
			emailSubject: "Vérification de votre adresse email",
			customHeaders: map[string]string{
				"Accept-Language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// standard header not supported language -> EN
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/FrenchUserID",
			token:        testing_token_uid1,
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"Accept-Language": "gr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// follow custom header -> FR
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/FrenchUserID",
			token:        testing_token_uid1,
			emailSubject: "Vérification de votre adresse email",
			customHeaders: map[string]string{
				"x-tidepool-language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header not supported language -> EN
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/FrenchUserID",
			token:        testing_token_uid1,
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"x-tidepool-language": "gr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header takes precedence over standard -> FR
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/FrenchUserID",
			token:        testing_token_uid1,
			emailSubject: "Vérification de votre adresse email",
			customHeaders: map[string]string{
				"Accept-Language": "en", "x-tidepool-language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header takes precedence over standard but language does not exist -> EN
			returnNone:   true,
			method:       "POST",
			url:          "/send/signup/FrenchUserID",
			token:        testing_token_uid1,
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"Accept-Language": "fr", "x-tidepool-language": "gr",
			},
			respCode: 200,
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
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Verify your email address",
			respCode:     200,
		},
		{
			// testing language preferences
			// follow standard header -> EN
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"Accept-Language": "en",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// follow standard header -> FR
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Vérification de votre adresse email",
			customHeaders: map[string]string{
				"Accept-Language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// standard header not supported language -> EN
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"Accept-Language": "gr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// follow custom header -> FR
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Vérification de votre adresse email",
			customHeaders: map[string]string{
				"x-tidepool-language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header not supported language -> EN
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"x-tidepool-language": "gr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header takes precedence over standard -> FR
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Vérification de votre adresse email",
			customHeaders: map[string]string{
				"Accept-Language": "en", "x-tidepool-language": "fr",
			},
			respCode: 200,
		},
		{
			// testing language preferences
			// custom header takes precedence over standard but language does not exist -> EN
			method:       "POST",
			url:          "/resend/signup/email.resend@address.org",
			emailSubject: "Verify your email address",
			customHeaders: map[string]string{
				"Accept-Language": "fr", "x-tidepool-language": "gr",
			},
			respCode: 200,
		},
		{
			// testing too many resends
			method:   "POST",
			url:      "/resend/signup/test.ResendCounterMax.CreatedRecent",
			respCode: 403,
		},
		{
			// you can't accept an invitation you didn't get
			returnNone: true,
			method:     "PUT",
			url:        "/accept/signup/UID2/UIDBad",
			respCode:   404,
		},
		{
			// you can't accept an invitation from another user key and userid wont give match
			returnNone: true,
			method:     "PUT",
			url:        "/accept/signup/UID2/UID",
			respCode:   404,
		},
		{
			// all good
			method:   "PUT",
			url:      "/accept/signup/signupkey",
			respCode: 200,
		},
		{
			method:   "PUT",
			url:      "/dismiss/signup/UID",
			respCode: 200,
			body: testJSONObject{
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
			body: testJSONObject{
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

	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	for idx, test := range tests {

		if test.skip {
			continue
		}

		//fresh each time
		var testRtr = mux.NewRouter()
		mockSeagull.SetMockNextCollectionCall("NewUserID"+"profile", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall("EnglishUserID"+"profile", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall("FrenchUserID"+"profile", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall("email.resend@address.org"+"profile", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall("profile", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall("WithoutPassword"+"profile", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall("UID"+"profile", `{"Something":"anit no thing"}`, nil)

		if test.returnNone {
			hydrophoneFindsNothing := InitApi(FAKE_CONFIG, mockStoreEmpty, mockNotifier, mockShoreline, mockPerms, mockSeagull, mockPortal, mockTemplates, logger)
			hydrophoneFindsNothing.SetHandlers("", testRtr)
		} else {
			hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockPerms, mockSeagull, mockPortal, mockTemplates, logger)
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
}
