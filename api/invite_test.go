package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mdblp/crew/store"
	"github.com/mdblp/go-common/clients/auth"
	"github.com/mdblp/hydrophone/templates"
	"github.com/mdblp/shoreline/token"
	"github.com/stretchr/testify/mock"
)

func initTestingRouterNoPerms() *mux.Router {
	testRtr := mux.NewRouter()
	mockAuth.ExpectedCalls = nil
	hydrophone := InitApi(
		FAKE_CONFIG,
		mockStoreEmpty,
		mockNotifier,
		mock_uid1Shoreline,
		mockPerms,
		mockAuth,
		mockSeagull,
		mockPortal,
		mockTemplates,
		logger,
	)
	hydrophone.SetHandlers("", testRtr)
	return testRtr
}

func TestInvitesLocales(t *testing.T) {
	tests := []toTest{
		//When the invitee is not a member yet:
		{
			desc:       "Caregiver invite to a non registered person, expect DE Language",
			returnNone: true,
			method:     http.MethodPost,
			url:        fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:      testing_token_uid1,
			respCode:   http.StatusOK,
			body: testJSONObject{
				"email":       "doesnotexist@myemail.com",
				"permissions": testJSONObject{"view": testJSONObject{}},
			},
			customHeaders: map[string]string{
				"x-tidepool-language": "de",
			},
			emailSubject: "Einladung Diabetes Care Team",
		},
		{
			desc:       "Medical team invite to a non registered person, expect DE Language",
			method:     "POST",
			returnNone: true,
			url:        "/send/team/invite",
			respCode:   200,
			token:      testing_token_uid2,
			body: testJSONObject{
				"email":  "doesnotexist@myemail.com",
				"teamId": "123456",
				"role":   "member",
			},
			customHeaders: map[string]string{
				"x-tidepool-language": "de",
			},
			emailSubject: "Einladung zur Teilnahme an einem Betreuungsteam",
		},
		// WHen the member is already a user
		{
			desc:       "Caregiver invite to an existing user, expect user's language",
			returnNone: true,
			method:     http.MethodPost,
			url:        fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:      testing_token_uid1,
			respCode:   http.StatusOK,
			body: testJSONObject{
				"email":       "caregiver@myemail.com",
				"permissions": testJSONObject{"view": testJSONObject{}},
			},
			customHeaders: map[string]string{
				"x-tidepool-language": "de",
			},
			emailSubject: "Invitation patient",
		},
		{
			method:     "POST",
			returnNone: true,
			url:        "/send/team/invite",
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "hcpMember@myemail.com",
				"teamId": "123456",
				"role":   "member",
			},
			customHeaders: map[string]string{
				"x-tidepool-language": "de",
			},
			emailSubject: "Invitación para unirse a un equipo de atención",
		},
	}
	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	for idx, test := range tests {
		var testRtr = initTestingTeamRouter(test.returnNone)
		mockSeagull.SetMockNextCollectionCall(testing_uid1+"preferences", `{"displayLanguageCode":"fr"}`, nil)
		mockSeagull.SetMockNextCollectionCall(testing_uid2+"preferences", `{"displayLanguageCode":"es"}`, nil)
		// mock user preference:
		// mockSeagull.SetMockNextCollectionCall(testing_uid2+"@email.org"+"preferences", `{"DisplayLanguage": "de"}`, nil)
		teams1 := []store.Team{}
		membersAccepted := store.Member{
			UserID:           testing_token_uid2,
			TeamID:           "123456",
			InvitationStatus: "accepted",
		}

		mockPerms.SetMockNextCall(testing_token_uid1, teams1, nil)
		mockPerms.SetMockNextCall(testing_token_uid2, teams1, nil)
		mockPerms.SetMockNextCall(testing_token_uid1+testing_uid2, &membersAccepted, nil)
		mockPerms.SetMockNextCall(testing_token_uid1, []store.DataShare{}, nil)
		var body = &bytes.Buffer{}
		if len(test.body) != 0 {
			json.NewEncoder(body).Encode(test.body)
		}
		request, _ := http.NewRequest(test.method, test.url, body)
		if test.token != "" {
			request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
		}
		if test.customHeaders != nil {
			for header, value := range test.customHeaders {
				request.Header.Set(header, value)
			}
		}
		mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
		response := httptest.NewRecorder()
		testRtr.ServeHTTP(response, request)

		if response.Code != test.respCode {
			t.Fatalf("Test %d url: '%s'\nNon-expected status code %d (expected %d):\n\tbody: %v",
				idx, test.url, response.Code, test.respCode, response.Body)
		}
		t.Logf("Test %d url: '%s'\nExpected status code %d (expected %d):\n\tbody: %v",
			idx, test.url, response.Code, test.respCode, response.Body)

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
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestSendInvite_ToAnother_Patient_Should_Respond_MethodNotAllowed(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()
	sendBody := &bytes.Buffer{}
	json.NewEncoder(sendBody).Encode(testJSONObject{
		"email": "patient.team@myemail.com",
		"permissions": testJSONObject{
			"view": testJSONObject{},
			"note": testJSONObject{},
		},
	})

	request, _ := http.NewRequest("POST", fmt.Sprintf("/send/invite/%s", testing_uid1), sendBody)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Logf("expected %d actual %d", http.StatusMethodNotAllowed, response.Code)
		t.Fail()
	}
}

func TestGetReceivedInvitations_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()

	request, _ := http.NewRequest("GET", fmt.Sprintf("/invitations/%s", testing_uid2), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
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
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestAcceptInvite_NoPerms(t *testing.T) {

	tstRtr := initTestingRouterNoPerms()
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
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
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
	request, _ := http.NewRequest("PUT", fmt.Sprintf("/dismiss/invite/%s/%s", testing_uid2, testing_uid1), nil)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestCaregiverInvite(t *testing.T) {

	inviteTests := []toTest{
		{
			desc:     "can't invite without a body",
			method:   http.MethodPost,
			url:      fmt.Sprintf("/send/invite/%s", testing_uid1),
			token:    testing_token_uid1,
			respCode: http.StatusBadRequest,
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
				"email":       testing_uid2 + "hcp@email.org",
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

	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	for idx, inviteTest := range inviteTests {
		// don't run a test if it says to skip it
		if inviteTest.skip {
			continue
		}
		var testRtr = mux.NewRouter()

		mockSeagull.SetMockNextCollectionCall("personToInvite@email.com"+"preferences", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall(testing_uid1+"@email.org"+"preferences", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall(testing_uid2+"@email.org"+"preferences", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall(testing_uid2+"hcp@email.org"+"preferences", `{"Something":"anit no thing"}`, nil)
		mockShoreline.On("TokenProvide").Return(testing_token)
		mockAuth = auth.NewMock()

		teams1 := []store.Team{}
		membersAccepted := store.Member{
			UserID:           testing_uid1,
			TeamID:           "123.456.789",
			InvitationStatus: "accepted",
		}

		mockPerms.SetMockNextCall(testing_token_uid1, teams1, nil)
		mockPerms.SetMockNextCall(testing_token+"123.456.789", &membersAccepted, nil)

		//default flow, fully authorized
		hydrophone := InitApi(
			FAKE_CONFIG,
			mockStore,
			mockNotifier,
			mockShoreline,
			mockPerms,
			mockAuth,
			mockSeagull,
			mockPortal,
			mockTemplates,
			logger,
		)

		//testing when there is nothing to return from the store
		if inviteTest.returnNone {
			hydrophone = InitApi(
				FAKE_CONFIG,
				mockStoreEmpty,
				mockNotifier,
				mockShoreline,
				mockPerms,
				mockAuth,
				mockSeagull,
				mockPortal,
				mockTemplates,
				logger,
			)
		}
		// testing when returning errors
		if inviteTest.doBad {
			hydrophone = InitApi(
				FAKE_CONFIG,
				mockStoreFails,
				mockNotifier,
				mockShoreline,
				mockPerms,
				mockAuth,
				mockSeagull,
				mockPortal,
				mockTemplates,
				logger,
			)
		}
		mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: true})
		hydrophone.SetHandlers("", testRtr)

		var body = &bytes.Buffer{}
		// build the body only if there is one defined in the test
		if len(inviteTest.body) != 0 {
			json.NewEncoder(body).Encode(inviteTest.body)
		}
		request, _ := http.NewRequest(inviteTest.method, inviteTest.url, body)
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
