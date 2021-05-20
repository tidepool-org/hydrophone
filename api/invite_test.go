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
	"github.com/tidepool-org/hydrophone/templates"
)

func initTestingRouterNoPerms() *mux.Router {
	testRtr := mux.NewRouter()
	hydrophone := InitApi(
		FAKE_CONFIG,
		mockStore,
		mockNotifier,
		mock_uid1Shoreline,
		mockPerms,
		mockSeagull,
		mockPortal,
		mockTemplates,
	)
	hydrophone.SetHandlers("", testRtr)
	return testRtr
}

func initTestingTeamRouter(returnNone bool) *mux.Router {
	//fresh each time
	var testRtr = mux.NewRouter()

	// Init mock data
	token1 := "00000"
	teams1 := []store.Team{}
	members := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "1",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           "4567",
			TeamID:           "2",
			Role:             "patient",
			InvitationStatus: "pending",
		},
	}
	membersAddAdminRole := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamAddAdminRole",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid3,
			TeamID:           "teamAddAdminRole",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid4,
			TeamID:           "teamAlreadyMember",
			Role:             "patient",
			InvitationStatus: "accepted",
		},
	}
	team123456 := store.Team{
		Name:        "Led Zep",
		Description: "Fake Team",
		Members:     members,
		ID:          "123456",
	}
	teamAddAdminRole := store.Team{
		Name:        "Led Zep",
		Description: "Fake Team",
		Members:     membersAddAdminRole,
		ID:          "123456",
	}
	teamAlreadyMember := store.Team{
		Name:        "team already member",
		Description: "Fake Team",
		Members:     membersAddAdminRole,
		ID:          "teamAlreadyMember",
	}
	teamDeleteMember := store.Team{
		Name:        "team already member",
		Description: "Fake Team",
		Members:     membersAddAdminRole,
		ID:          "teamDeleteMember",
	}
	teamAddPatientAsMember := store.Team{
		Name:        "team add a patient as a member",
		Description: "Fake Team",
		Members:     members,
		ID:          "teamInvitePatient",
	}
	membersDismissInvite := []store.Member{
		{
			UserID:           testing_uid3,
			TeamID:           "teamDismissInvite",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid1,
			TeamID:           "teamDismissInvite",
			InvitationStatus: "pending",
		},
	}
	membersDismissInvite_uid1 := store.Member{
		UserID:           testing_uid1,
		TeamID:           "teamDismissInvite",
		InvitationStatus: "pending",
	}
	teamDismissInvite := store.Team{
		Name:        "team dismiss invite",
		Description: "Fake Team",
		Members:     membersDismissInvite,
		ID:          "teamDismissInvite",
	}
	membersDismissInviteAsAdmin := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamDismissInvite",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           "UIDpending",
			TeamID:           "teamDismissInvite",
			InvitationStatus: "pending",
		},
	}
	teamDismissInviteAsAdmin := store.Team{
		Name:        "team dismiss invite",
		Description: "Fake Team",
		Members:     membersDismissInviteAsAdmin,
		ID:          "teamDismissInviteAsAdmin",
	}
	teamDismissInvitePatient := store.Team{
		Name:        "team dismiss invite",
		Description: "Fake Team",
		Members:     membersDismissInviteAsAdmin,
		ID:          "teamDismissInvitePatient",
	}

	member_uid3 := store.Member{
		TeamID:           "1",
		InvitationStatus: "pending",
	}

	member_uid4 := store.Member{
		UserID:           testing_uid4,
		TeamID:           "123456",
		Role:             "patient",
		InvitationStatus: "pending",
	}

	member_dup := store.Member{
		UserID:           testing_uid4,
		TeamID:           "teamAlreadyMember",
		Role:             "patient",
		InvitationStatus: "pending",
	}

	member_dismissed := store.Member{
		UserID:           testing_uid4,
		TeamID:           "123456",
		Role:             "member",
		InvitationStatus: "pending",
	}
	patient_dismissed := store.Member{
		UserID:           testing_uid4,
		TeamID:           "123456",
		Role:             "patient",
		InvitationStatus: "pending",
	}

	mockPerms.SetMockNextCall(token1, teams1, nil)
	mockPerms.SetMockNextCall(testing_token_uid1, teams1, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"123456", &team123456, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+testing_uid3, &member_uid3, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamAddAdminRole", &teamAddAdminRole, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamAlreadyMember", &teamAlreadyMember, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamAlreadyMember", []store.Member{member_dup}, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamInvitePatient", []store.Member{}, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"123456", []store.Member{}, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamInvitePatient", &teamAddPatientAsMember, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDeleteMember", &teamDeleteMember, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+testing_uid1, &membersDismissInvite_uid1, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDismissInvite", &teamDismissInvite, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"key.to.be.dismissed", &member_dismissed, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"patient.key.to.be.dismissed", &patient_dismissed, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDismissInviteAsAdmin", &teamDismissInviteAsAdmin, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDismissInvitePatient", &teamDismissInvitePatient, nil)

	mockPerms.SetMockNextCall(testing_token_uid1+testing_uid4, &member_uid4, nil)

	mockSeagull.SetMockNextCollectionCall(testing_uid1+"profile", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall("patient.team@myemail.com"+"profile", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid1+"preferences", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid3+"preferences", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid4+"preferences", `{"Something":"anit no thing"}`, nil)

	hydrophone := InitApi(
		FAKE_CONFIG,
		mockStore,
		mockNotifier,
		mock_uid1Shoreline,
		mockPerms,
		mockSeagull,
		mockPortal,
		mockTemplates,
	)
	if returnNone {
		hydrophone = InitApi(
			FAKE_CONFIG,
			mockStoreEmpty,
			mockNotifier,
			mock_uid1Shoreline,
			mockPerms,
			mockSeagull,
			mockPortal,
			mockTemplates,
		)

	}
	hydrophone.SetHandlers("", testRtr)
	return testRtr
}

func initTests() []toTest {

	tests := []toTest{
		// returns a 200 when everything goes well
		{
			method:     "POST",
			returnNone: true,
			url:        "/send/team/invite",
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "me2@myemail.com",
				"teamId": "123456",
			},
		},
		// returns a 400 when body is not well formed
		{
			method:   "POST",
			url:      "/send/team/invite",
			respCode: 400,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email": "me2@myemail.com",
			},
		},
		// returns a 409 when user is already a member
		{
			method:   "POST",
			url:      "/send/team/invite",
			respCode: 409,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  "me2@myemail.com",
				"teamId": "teamAlreadyMember",
			},
		},
		// returns a 405 when the invited member is a patient
		{
			method:     "POST",
			url:        "/send/team/invite",
			returnNone: true,
			respCode:   405,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "patient.team@myemail.com",
				"teamId": "teamInvitePatient",
			},
		},
		// returns a 200 when everything goes well to add an admin role
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid3),
			respCode: 200,
			token:    testing_token_uid1,
			body: testJSONObject{
				"user":   testing_uid3,
				"teamId": "teamAddAdminRole",
				"role":   "admin",
			},
		},
		// returns a 200 when everything goes well to delete a member
		{
			method:   "DELETE",
			url:      fmt.Sprintf("/send/team/leave/%s", testing_uid3),
			respCode: 200,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  "me2@myemail.com",
				"teamId": "teamAlreadyMember",
			},
		},
		// returns a 400 when body is not well formed to delete a member
		{
			method:   "DELETE",
			url:      fmt.Sprintf("/send/team/leave/%s", testing_uid3),
			respCode: 400,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email": "me2@myemail.com",
			},
		},
		// returns a 400 when body is not well formed to delete a member
		{
			method:   "DELETE",
			url:      fmt.Sprintf("/send/team/leave/%s", testing_uid3),
			respCode: 400,
			token:    testing_token_uid1,
			body: testJSONObject{
				"teamId": "teamAlreadyMember",
			},
		},
		// returns a 200 when dismiss a team invite for yourself
		{
			method:     "PUT",
			returnNone: false,
			url:        fmt.Sprintf("/dismiss/team/invite/%s", "teamDismissInvite"),
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"key": "key.to.be.dismissed",
			},
		},
		// returns a 200 when dismiss a team invite as Admin
		{
			method:     "PUT",
			returnNone: false,
			url:        fmt.Sprintf("/dismiss/team/invite/%s", "teamDismissInviteAsAdmin"),
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"key": "key.to.be.dismissed",
			},
		},
		// returns a 400 when dismiss a team invite of non existing team as Admin
		{
			method:     "PUT",
			returnNone: false,
			url:        fmt.Sprintf("/dismiss/team/invite/%s", "nonExistingTeam"),
			respCode:   400,
			token:      testing_token_uid1,
			body: testJSONObject{
				"key": "key.to.be.dismissed",
			},
		},
		// returns a 200 when dismiss a team invite as Admin
		{
			method:     "PUT",
			returnNone: false,
			url:        fmt.Sprintf("/dismiss/team/invite/%s", "teamDismissInvitePatient"),
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"key": "patient.key.to.be.dismissed",
			},
		},
		// returns a 200 when everything goes well
		{
			method:     "POST",
			returnNone: true,
			url:        "/send/team/invite",
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "patient.team@myemail.com",
				"teamId": "123456",
				"role":   "patient",
			},
		},
		// returns a 400 when body is not well formed
		{
			method:   "POST",
			url:      "/send/team/invite",
			respCode: 400,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email": "patient.team@myemail.com",
				"role":  "patient",
			},
		},
		// returns a 400 when body is not well formed
		{
			method:   "POST",
			url:      "/send/team/invite",
			respCode: 400,
			token:    testing_token_uid1,
			body: testJSONObject{
				"teamId": "123456",
				"role":   "patient",
			},
		},
		// returns a 403 when account of the patient does not exist
		{
			method:     "POST",
			url:        "/send/team/invite",
			returnNone: true,
			respCode:   403,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "patient.doesnotexist@myemail.com",
				"teamId": "123456",
				"role":   "patient",
			},
		},
		// returns a 409 when user is already a member
		{
			method:     "POST",
			returnNone: true,
			url:        "/send/team/invite",
			respCode:   409,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "patient.team@myemail.com",
				"teamId": "teamAlreadyMember",
				"role":   "patient",
			},
			response: testJSONObject{
				"code":   float64(409),
				"error":  float64(1001),
				"reason": statusExistingMemberMessage,
			},
		},
		// returns a 409 when there is already an invite
		{
			method:   "POST",
			url:      "/send/team/invite",
			respCode: 409,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  "patient.team@myemail.com",
				"teamId": "teamAlreadyMember",
				"role":   "patient",
			},
			response: testJSONObject{
				"code":   float64(409),
				"error":  float64(1001),
				"reason": statusExistingInviteMessage,
			},
		},
	}
	return tests
}

func TestTeam(t *testing.T) {
	tests := initTests()
	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	for idx, test := range tests {
		var testRtr = initTestingTeamRouter(test.returnNone)
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

func initWrongBodies() []testJSONObject {
	var bodies []testJSONObject
	bodies = append(
		bodies,
		testJSONObject{
			"email":   testing_uid2 + "@email.org",
			"teamId":  "",
			"isAdmin": "true",
		},
		testJSONObject{
			"email":   testing_uid2 + "@email.org",
			"teamId":  "123456",
			"isAdmin": "",
		},
		testJSONObject{
			"email": testing_uid2 + "@email.org",
		},
		testJSONObject{},
	)
	return bodies
}

func sendTeamInvite(method, path string, t *testing.T) {
	tstRtr := initTestingRouterNoPerms()
	wrongBodies := initWrongBodies()
	for i := 0; i < len(wrongBodies); i++ {
		body := &bytes.Buffer{}
		json.NewEncoder(body).Encode(wrongBodies[i])

		request, _ := http.NewRequest(method, path, body)
		request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
		response := httptest.NewRecorder()
		tstRtr.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Logf("expected %d actual %d", http.StatusBadRequest, response.Code)
			t.Fail()
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
		{
			desc:     "valid request to accept an team invite",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			body: testJSONObject{
				"key": "medicalteam.invite.member",
			},
		},
		{
			desc:     "valid request to accept an team invite for a patient",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_uid1,
			respCode: http.StatusOK,
			body: testJSONObject{
				"key":  "medicalteam.invite.patient",
				"role": "patient",
			},
		},
		{
			desc:     "not authorized request to accept a team invite",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_uid1,
			respCode: http.StatusForbidden,
			body: testJSONObject{
				"key": "medicalteam.invite.wrong.member",
			},
		},
		{
			desc:     "invitation does not exist",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_uid1,
			respCode: http.StatusForbidden,
			body: testJSONObject{
				"key": "invalid.key",
			},
		},
		{
			desc:     "invalid invitation",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_uid1,
			respCode: http.StatusNotFound,
			body: testJSONObject{
				"key": "key.does.not.exist",
			},
		},
		{
			desc:     "Any invite no key",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_uid1,
			respCode: http.StatusBadRequest,
		},
		{
			desc:   "Any invite invalid key",
			method: http.MethodPut,
			url:    "/accept/team/invite",
			token:  testing_token_uid1,
			body: testJSONObject{
				"key": "any.invite.invalid.key",
			},
			respCode: http.StatusNotFound,
		},
		{
			desc:   "Any invite already completed",
			method: http.MethodPut,
			url:    "/accept/team/invite",
			token:  testing_token_uid1,
			body: testJSONObject{
				"key": "any.invite.completed.key",
			},
			respCode: http.StatusForbidden,
		},
		{
			desc:   "Error getting invite",
			doBad:  true,
			method: http.MethodPut,
			url:    "/accept/team/invite",
			token:  testing_token_uid1,
			body: testJSONObject{
				"key": "foo",
			},
			respCode: http.StatusInternalServerError,
		},
		{
			desc:   "Any invite not a valid type",
			method: http.MethodPut,
			url:    "/accept/team/invite",
			token:  testing_token_uid1,
			body: testJSONObject{
				"key": "invite.wrong.type",
			},
			respCode: http.StatusForbidden,
		},
		{
			desc:   "Any valid invite do admin",
			method: http.MethodPut,
			url:    "/accept/team/invite",
			token:  testing_token_uid1,
			body: testJSONObject{
				"key": "any.invite.pending.do.admin",
			},
			respCode: http.StatusOK,
		},
		{
			desc:   "Any valid invite remove",
			method: http.MethodPut,
			url:    "/accept/team/invite",
			token:  testing_token_uid1,
			body: testJSONObject{
				"key": "any.invite.pending.remove",
			},
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
			mockSeagull,
			mockPortal,
			mockTemplates,
		)

		//testing when there is nothing to return from the store
		if inviteTest.returnNone {
			hydrophone = InitApi(
				FAKE_CONFIG,
				mockStoreEmpty,
				mockNotifier,
				mockShoreline,
				mockPerms,
				mockSeagull,
				mockPortal,
				mockTemplates,
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
				mockSeagull,
				mockPortal,
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

func TestSendTeamInvite_WrongBody(t *testing.T) {

	sendTeamInvite("POST", "/send/team/invite", t)
}

func TestSendTeamInvite_NoTeam(t *testing.T) {

	sendTeamInvite("POST", "/send/team/invite", t)
}

func TestUpdateTeamRole_WrongBody(t *testing.T) {

	sendTeamInvite("PUT", "/send/team/role/UID0000", t)
}

func TestDeleteTeamInvite_WrongBody(t *testing.T) {

	sendTeamInvite("DELETE", "/send/team/leave/UID0000", t)
}
