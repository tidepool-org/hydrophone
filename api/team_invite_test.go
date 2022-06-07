package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/mdblp/crew/store"
	"github.com/mdblp/hydrophone/templates"
	"github.com/mdblp/shoreline/token"
	"github.com/stretchr/testify/mock"
)

func initTestingTeamRouter(returnNone bool) *mux.Router {
	//fresh each time
	var testRtr = mux.NewRouter()

	// Init mock data
	remoteMonitored := true
	notRemoteMonitored := false
	token1 := "00000"
	teams1 := []store.Team{}
	members := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "1",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
	}
	membersSetMemberRole := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamSetMemberRole",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid2,
			TeamID:           "teamSetMemberRole",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid5,
			TeamID:           "teamSetMemberRole",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
	}
	membersSetAdminRole := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamSetAdminRole",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid3,
			TeamID:           "teamSetAdminRole",
			Role:             "member",
			InvitationStatus: "accepted",
		},
	}
	membersAlready := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamAlreadyMember",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid3,
			TeamID:           "teamAlreadyMember",
			Role:             "member",
			InvitationStatus: "accepted",
		},
	}
	team123456 := store.Team{
		Name:        "Led Zep",
		Description: "Fake Team",
		Members:     members,
		ID:          "123456",
	}
	teamSetMemberRole := store.Team{
		Name:        "Led Zep",
		Description: "Fake Team",
		Members:     membersSetMemberRole,
		ID:          "123456",
	}
	teamSetAdminRole := store.Team{
		Name:        "Led Zep",
		Description: "Fake Team",
		Members:     membersSetAdminRole,
		ID:          "123456",
	}
	teamAlreadyMember := store.Team{
		Name:        "team already member",
		Description: "Fake Team",
		Members:     membersAlready,
		ID:          "teamAlreadyMember",
		RemotePatientMonitoring: &store.TeamMonitoring{
			Enabled: &remoteMonitored,
		},
	}
	teamDeleteMember := store.Team{
		Name:        "team already member",
		Description: "Fake Team",
		Members:     membersAlready,
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
	membersMonitoringTeam := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamMonitoring",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
	}

	patientsMonitoringTeam := []store.Patient{
		{
			UserID:           testing_uid_patient1,
			TeamID:           "teamMonitoring",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid1,
			TeamID:           "teamMonitoring",
			InvitationStatus: "accepted",
		},
		{
			UserID:           testing_uid_patient2,
			TeamID:           "teamMonitoring",
			InvitationStatus: "pending",
		},
	}

	teamMonitoring := store.Team{
		Name:        "team monitoring",
		Description: "team monitoring",
		Members:     membersMonitoringTeam,
		ID:          "teamMonitoring",
		RemotePatientMonitoring: &store.TeamMonitoring{
			Enabled: &remoteMonitored,
		},
	}

	membersMonitoringTeamNotAdmin := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamMonitoring",
			Role:             "member",
			InvitationStatus: "accepted",
		},
	}

	teamMonitoringNotAdmin := store.Team{
		Name:        "team monitoring",
		Description: "team monitoring",
		Members:     membersMonitoringTeamNotAdmin,
		ID:          "teamMonitoring",
		RemotePatientMonitoring: &store.TeamMonitoring{
			Enabled: &remoteMonitored,
		},
	}

	teamMonitoringNotMonitored := store.Team{
		Name:        "team monitoring",
		Description: "team monitoring",
		Members:     membersMonitoringTeam,
		ID:          "teamMonitoring",
		RemotePatientMonitoring: &store.TeamMonitoring{
			Enabled: &notRemoteMonitored,
		},
	}

	membersMonitoringTeamNotMember := []store.Member{
		{
			UserID:           testing_uid1,
			TeamID:           "teamMonitoring",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
	}

	teamMonitoringNotMember := store.Team{
		Name:        "teamMonitoringNotMember",
		Description: "teamMonitoringNotMember",
		Members:     membersMonitoringTeamNotMember,
		ID:          "teamMonitoringNotMember",
		RemotePatientMonitoring: &store.TeamMonitoring{
			Enabled: &remoteMonitored,
		},
	}

	member_uid3 := store.Member{
		TeamID:           "1",
		InvitationStatus: "pending",
	}

	patient_uid4 := store.Patient{
		UserID:           testing_uid4,
		TeamID:           "123456",
		InvitationStatus: "pending",
	}

	patient_dup := store.Patient{
		UserID:           testing_uid4,
		TeamID:           "teamAlreadyMember",
		InvitationStatus: "pending",
	}

	member_dismissed := store.Member{
		UserID:           testing_uid4,
		TeamID:           "123456",
		Role:             "member",
		InvitationStatus: "pending",
	}
	patient_dismissed := store.Patient{
		UserID:           testing_uid4,
		TeamID:           "123456",
		InvitationStatus: "pending",
	}

	mockPerms.SetMockNextCall(token1, teams1, nil)
	mockPerms.SetMockNextCall(testing_token_uid1, teams1, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"123456", &team123456, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+testing_uid3, &member_uid3, nil) // Used in teamSetAdminRole too

	mockPerms.SetMockNextCall(testing_token_uid1+"NotInAnyTeam", nil, fmt.Errorf("Member not found"))
	mockPerms.SetMockNextCall(testing_token_uid1+"teamSetMemberRole", &teamSetMemberRole, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+testing_uid2, &membersSetMemberRole[1], nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamSetAdminRole", &teamSetAdminRole, nil)

	mockPerms.SetMockNextCall(testing_token_uid1+"teamAlreadyMember", &teamAlreadyMember, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamAlreadyMember", []store.Patient{patient_dup}, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamInvitePatient", []store.Patient{}, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"123456", []store.Patient{}, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamMonitoring", patientsMonitoringTeam, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamMonitoringEmpty", []store.Patient{}, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamMonitoringNotMember", &teamMonitoringNotMember, nil)
	mockPerms.SetMockNextCall("GetTeamPatients"+testing_token_uid1+"teamMonitoringNotMember", []store.Patient{}, nil)

	mockPerms.SetMockNextCall(testing_token_uid1+"teamInvitePatient", &teamAddPatientAsMember, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDeleteMember", &teamDeleteMember, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+testing_uid1, &membersDismissInvite_uid1, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDismissInvite", &teamDismissInvite, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamMonitoring", &teamMonitoring, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamMonitoringNotAdmin", &teamMonitoringNotAdmin, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamMonitoringNotMonitored", &teamMonitoringNotMonitored, nil)

	mockPerms.SetMockNextCall(testing_token_uid1+"key.to.be.dismissed", &member_dismissed, nil)
	mockPerms.SetMockNextCall("UpdatePatient"+testing_token_uid1+"patient.key.to.be.dismissed", &patient_dismissed, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDismissInviteAsAdmin", &teamDismissInviteAsAdmin, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"teamDismissInvitePatient", &teamDismissInvitePatient, nil)

	mockPerms.SetMockNextCall("AddPatient"+testing_token_uid1+testing_uid4, &patient_uid4, nil)

	mockPerms.On(
		"UpdatePatientMonitoringWithContext", mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)
	mockPerms.On(
		"GetPatientMonitoring", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(&store.Patient{}, nil)

	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamMonitoringEmpty").
		Return(nil, errors.New("Team doesn't exist"))
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamAlreadyMember").
		Return(&teamAlreadyMember, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamInvitePatient").
		Return(&teamAddPatientAsMember, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamDeleteMember").
		Return(&teamDeleteMember, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamDismissInvite").
		Return(&teamDismissInvite, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamMonitoring").
		Return(&teamMonitoring, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamMonitoringNotAdmin").
		Return(&teamMonitoringNotAdmin, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamMonitoringNotMonitored").
		Return(&teamMonitoringNotMonitored, nil)
	mockPerms.On("GetTeamWithContext", mock.Anything, mock.Anything, "teamMonitoringNotMember").
		Return(&teamMonitoringNotMember, nil)

	mockShoreline.On("TokenProvide").Return("ok")

	mockSeagull.SetMockNextCollectionCall(testing_uid1+"profile", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall("patient.team@myemail.com"+"profile", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid1+"preferences", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid3+"preferences", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid4+"preferences", `{"Something":"anit no thing"}`, nil)
	mockSeagull.SetMockNextCollectionCall(testing_uid_patient1+"preferences", `{"Something":"anit no thing"}`, nil)

	hydrophone := InitApi(
		FAKE_CONFIG,
		mockStore,
		mockNotifier,
		mock_uid1Shoreline,
		mockPerms,
		mockAuth,
		mockSeagull,
		mockPortal,
		mockTemplates,
		logger,
	)
	if returnNone {
		hydrophone = InitApi(
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

	}
	hydrophone.SetHandlers("", testRtr)
	return testRtr
}

func initTests() []toTest {
	monitoringEnd := time.Now().Add(2 * time.Hour)
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
		// returns a 400 when everything goes wrong to set member role
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid5),
			respCode: http.StatusBadRequest,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  testing_uid5 + "@example.com",
				"teamId": "NotInAnyTeam",
				"role":   "member",
			},
		},
		// returns a 200 when everything goes well to set member role
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid2),
			respCode: 200,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  testing_uid2 + "@example.com",
				"teamId": "teamSetMemberRole",
				"role":   "member",
			},
		},
		// returns a 200 when everything goes well to set admin role
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid3),
			respCode: 200,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  testing_uid3 + "@example.com",
				"teamId": "teamSetAdminRole",
				"role":   "admin",
			},
		},
		// returns 400 when the member do not exists
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid5),
			respCode: http.StatusBadRequest,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  testing_uid5 + "@example.com",
				"teamId": "teamSetAdminRole",
				"role":   "admin",
			},
		},
		// returns a 409 when the member already has this role
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid3),
			respCode: http.StatusConflict,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  testing_uid3 + "@example.com",
				"teamId": "teamSetAdminRole",
				"role":   "member",
			},
		},
		// returns a 500 when the update role failed in crew
		{
			method:   "PUT",
			url:      fmt.Sprintf("/send/team/role/%s", testing_uid5),
			respCode: http.StatusInternalServerError,
			token:    testing_token_uid1,
			body: testJSONObject{
				"email":  testing_uid5 + "@example.com",
				"teamId": "teamSetMemberRole",
				"role":   "member",
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
				"email":  "doesnotexist@myemail.com",
				"teamId": "123456",
				"role":   "patient",
			},
		},
		// returns a 403 when account of the hcp does not exist
		{
			method:     "POST",
			url:        "/send/team/invite",
			returnNone: true,
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"email":  "doesnotexist@myemail.com",
				"teamId": "123456",
				"role":   "member",
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
		{
			method:   "POST",
			url:      "/send/team/monitoring/teamAlreadyMember/UID123",
			respCode: 409,
			token:    testing_token_uid1,
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
			response: testJSONObject{
				"code":   float64(409),
				"error":  float64(1001),
				"reason": statusExistingInviteMessage,
			},
		},
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoring/" + testing_uid_patient1,
			returnNone: true,
			respCode:   200,
			token:      testing_token_uid1,
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
		},
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoring/" + testing_uid_patient2,
			returnNone: false,
			respCode:   409,
			token:      testing_token_uid1,
			response: testJSONObject{
				"code":   float64(409),
				"error":  float64(1001),
				"reason": statusExistingInviteMessage,
			},
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
		},
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoringNotAdmin/" + testing_uid_patient2,
			returnNone: true,
			respCode:   401,
			token:      testing_token_uid1,
			response: testJSONObject{
				"code":   float64(401),
				"error":  float64(1001),
				"reason": STATUS_NOT_ADMIN,
			},
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
		},
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoringNotMonitored/" + testing_uid_patient1,
			returnNone: true,
			respCode:   400,
			token:      testing_token_uid1,
			response: testJSONObject{
				"code":   float64(400),
				"error":  float64(1001),
				"reason": STATUS_NOT_TEAM_MONITORING,
			},
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
		},
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoring/doesnotexist@myemail.com",
			returnNone: true,
			respCode:   400,
			token:      testing_token_uid1,
			response: testJSONObject{
				"code":   float64(400),
				"error":  float64(1001),
				"reason": STATUS_ERR_FINDING_USER,
			},
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
		},
		// STATUS_ERR_FINDING_TEAM
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoringEmpty/" + testing_uid_patient1,
			returnNone: true,
			respCode:   400,
			token:      testing_token_uid1,
			response: testJSONObject{
				"code":   float64(400),
				"error":  float64(1001),
				"reason": STATUS_ERR_FINDING_TEAM,
			},
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
			},
		},
		// STATUS_ERR_PATIENT_NOT_MBR
		{
			method:     "POST",
			url:        "/send/team/monitoring/teamMonitoringNotMember/" + testing_uid_patient3,
			returnNone: true,
			respCode:   500,
			token:      testing_token_uid1,
			response: testJSONObject{
				"code":   float64(500),
				"error":  float64(1001),
				"reason": STATUS_ERR_PATIENT_NOT_MBR,
			},
			body: testJSONObject{
				"monitoringEnd": monitoringEnd,
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
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})

	for idx, test := range tests {
		var testRtr = initTestingTeamRouter(test.returnNone)
		var body = &bytes.Buffer{}
		if len(test.body) != 0 {
			json.NewEncoder(body).Encode(test.body)
		}
		request, _ := http.NewRequest(test.method, test.url, body)
		if test.token != "" {
			request.Header.Set(TP_SESSION_TOKEN, test.token)
		}
		if test.customHeaders != nil {
			for header, value := range test.customHeaders {
				request.Header.Set(header, value)
			}
		}
		response := httptest.NewRecorder()
		testRtr.ServeHTTP(response, request)

		if response.Code != test.respCode {
			t.Fatalf("Test %d url: %s '%s'\nNon-expected status code %d (expected %d):\n\tbody: %v",
				idx, test.method, test.url, response.Code, test.respCode, response.Body)
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
		testJSONObject{
			"teamId": "abcdef",
		},
		testJSONObject{
			"teamId": "abcdef",
			"email":  testing_uid2 + "@email.org",
			"role":   "god",
		},
		testJSONObject{},
	)
	return bodies
}

func sendTeamInvite(method, path string, t *testing.T) {
	tstRtr := initTestingRouterNoPerms()
	wrongBodies := initWrongBodies()
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
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

func TestSendTeamInvite_WrongBody(t *testing.T) {
	sendTeamInvite("POST", "/send/team/invite", t)
}

func TestDeleteTeamInvite_WrongBody(t *testing.T) {
	sendTeamInvite("DELETE", "/send/team/leave/UID0000", t)
}

func TestUpdateTeamRole_WrongBody(t *testing.T) {
	sendTeamInvite("PUT", "/send/team/role/UID0000", t)
}

func TestUpdateTeamRole_NoToken(t *testing.T) {
	tstRtr := initTestingRouterNoPerms()
	body := &bytes.Buffer{}
	json.NewEncoder(body).Encode(testJSONObject{})
	mockAuth.On("Authenticate", mock.Anything).Return(nil)
	request, _ := http.NewRequest("PUT", "/send/team/role/UID0000", body)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Logf("expected %d actual %d", http.StatusUnauthorized, response.Code)
		t.Fail()
	}
}

func TestUpdateTeamRole_InvalidBody(t *testing.T) {
	tstRtr := initTestingRouterNoPerms()
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
	body := strings.NewReader("[Invalid JSON]")

	request, _ := http.NewRequest("PUT", "/send/team/role/UID0000", body)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Logf("expected %d actual %d", http.StatusBadRequest, response.Code)
		t.Fail()
	}
	expectedBody := fmt.Sprintf("{\"code\":%d,\"reason\":\"%s\"}", http.StatusBadRequest, STATUS_ERR_DECODING_INVITE)
	responseBody := response.Body.String()
	if responseBody != expectedBody {
		t.Fatalf("expected body '%s' receive '%s'", expectedBody, responseBody)
	}
}

func TestUpdateTeamRole_NoinviteeID(t *testing.T) {
	tstRtr := initTestingRouterNoPerms()
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
	body := &bytes.Buffer{}
	json.NewEncoder(body).Encode(testJSONObject{})

	request, _ := http.NewRequest("PUT", "/send/team/role/", body)
	request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
	response := httptest.NewRecorder()
	tstRtr.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Logf("expected %d actual %d", http.StatusNotFound, response.Code)
		t.Fail()
	}
}

func TestAcceptTeamInvite(t *testing.T) {

	inviteTests := []toTest{
		{
			desc:     "valid request to accept a team invite",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_hcp,
			respCode: http.StatusOK,
			body: testJSONObject{
				"key": "medicalteam.invite.member",
			},
		},
		{
			desc:     "valid request to accept a team invite for a patient",
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
			desc:     "valid request to accept a team invite",
			method:   http.MethodPut,
			url:      "/accept/team/invite",
			token:    testing_token_caregiver,
			respCode: http.StatusForbidden,
			body: testJSONObject{
				"key": "medicalteam.invite.member",
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

	mockAuth := NewAuthMock(testing_uid1)

	for idx, inviteTest := range inviteTests {
		// don't run a test if it says to skip it
		if inviteTest.skip {
			continue
		}
		var testRtr = mux.NewRouter()

		mockSeagull.SetMockNextCollectionCall(testing_uid1+"@email.org"+"preferences", `{"Something":"anit no thing"}`, nil)
		mockSeagull.SetMockNextCollectionCall(testing_uid2+"@email.org"+"preferences", `{"Something":"anit no thing"}`, nil)

		teams1 := []store.Team{}
		membersAccepted := store.Member{
			UserID:           testing_uid1,
			TeamID:           "123456",
			InvitationStatus: "accepted",
		}
		patientsAccepted := store.Patient{
			UserID:           testing_uid1,
			TeamID:           "123456",
			InvitationStatus: "accepted",
		}

		mockPerms.SetMockNextCall(testing_token, teams1, nil)
		mockPerms.SetMockNextCall(testing_token+testing_uid1, &membersAccepted, nil)
		mockPerms.SetMockNextCall("UpdatePatient"+testing_token_uid1+testing_uid1, &patientsAccepted, nil)
		mockPerms.On(
			"UpdatePatientMonitoringWithContext", mock.Anything, mock.Anything, mock.Anything,
		).Return(nil, nil)
		mockPerms.On(
			"GetPatientMonitoring", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(&store.Patient{}, nil)

		//default flow, fully authorized
		hydrophone := InitApi(
			FAKE_CONFIG,
			mockStore,
			mockNotifier,
			mock_uid1Shoreline,
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
				mock_uid1Shoreline,
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
				mock_uid1Shoreline,
				mockPerms,
				mockAuth,
				mockSeagull,
				mockPortal,
				mockTemplates,
				logger,
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
			request.Header.Set(TP_SESSION_TOKEN, inviteTest.token)
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

func TestAcceptMonitoringInvite(t *testing.T) {

	inviteTests := []toTest{
		{
			desc:     "valid request to accept a monitoring invite",
			method:   http.MethodPut,
			url:      "/accept/team/monitoring/123456/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusOK,
		},
		// Forbidden request on already accepted invitation
		{
			desc:     "forbidden request to accept a monitoring invite already accepted",
			method:   http.MethodPut,
			url:      "/accept/team/monitoring/accepted/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusForbidden,
		},
		// Forbidden request on declined invitation
		{
			desc:     "forbidden request to accept a monitoring invite already declined",
			method:   http.MethodPut,
			url:      "/accept/team/monitoring/declined/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusForbidden,
		},
		// Forbidden request on expired invitation
		{
			desc:     "forbidden request to accept a monitoring invite that is expired",
			method:   http.MethodPut,
			url:      "/accept/team/monitoring/expired/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusConflict,
		},
		// Wrong user to access an invitation
		{
			desc:     "valid request to accept a monitoring invite",
			method:   http.MethodPut,
			url:      "/accept/team/monitoring/declined/UnauthorizedUserID",
			token:    testing_token_uid2,
			respCode: http.StatusForbidden,
			response: testJSONObject{
				"code":   float64(403),
				"error":  float64(1001),
				"reason": STATUS_UNAUTHORIZED,
			},
		},
		{
			desc:     "Store return bad",
			method:   http.MethodPut,
			url:      "/accept/team/monitoring/123456/" + testing_uid1,
			token:    testing_token_uid1,
			doBad:    true,
			respCode: http.StatusInternalServerError,
		},
		{
			desc:       "Store return None",
			method:     http.MethodPut,
			url:        "/accept/team/monitoring/123456/" + testing_uid1,
			token:      testing_token_uid1,
			returnNone: true,
			respCode:   http.StatusNotFound,
		},
	}

	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	teams1 := []store.Team{}
	membersAccepted := store.Member{
		UserID:           testing_uid1,
		TeamID:           "123456",
		InvitationStatus: "accepted",
	}
	mockPerms.SetMockNextCall(testing_token, teams1, nil)
	mockPerms.SetMockNextCall(testing_token+testing_uid1, &membersAccepted, nil)
	mockPerms.SetMockNextCall(testing_token_uid1+"123456", &store.Patient{}, nil)
	mockAuth := NewAuthMock(testing_uid1)
	mockPerms.On(
		"GetPatientMonitoring", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(&store.Patient{}, nil)

	for idx, inviteTest := range inviteTests {
		// don't run a test if it says to skip it
		if inviteTest.skip {
			continue
		}
		var testRtr = mux.NewRouter()

		//default flow, fully authorized
		hydrophone := InitApi(
			FAKE_CONFIG,
			mockStore,
			mockNotifier,
			mock_uid1Shoreline,
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
				mock_uid1Shoreline,
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
				mock_uid1Shoreline,
				mockPerms,
				mockAuth,
				mockSeagull,
				mockPortal,
				mockTemplates,
				logger,
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
			request.Header.Set(TP_SESSION_TOKEN, inviteTest.token)
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

func TestDismissMonitoringInvite(t *testing.T) {

	inviteTests := []toTest{
		{
			desc:     "valid request to dismiss a monitoring invite",
			method:   http.MethodPut,
			url:      "/dismiss/team/monitoring/dismiss.team/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusOK,
		},
		{
			desc:     "request on non existing invite",
			method:   http.MethodPut,
			url:      "/dismiss/team/monitoring/not.found/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusNotFound,
			response: testJSONObject{
				"code":   float64(404),
				"error":  float64(1001),
				"reason": statusInviteNotFoundMessage,
			},
		},
		// Forbidden request on user not a member
		{
			desc:     "valid request to dismiss a monitoring invite from team admin",
			method:   http.MethodPut,
			url:      "/dismiss/team/monitoring/dismiss.team/" + testing_uid1,
			token:    testing_token_hcp2,
			respCode: http.StatusOK,
		},
		{
			desc:     "forbidden request dismiss a monitoring invite from team admin",
			method:   http.MethodPut,
			url:      "/dismiss/team/monitoring/dismiss.team.not.admin/" + testing_uid1,
			token:    testing_token_hcp2,
			respCode: http.StatusUnauthorized,
		},
		{
			desc:     "Store returns error",
			method:   http.MethodPut,
			url:      "/dismiss/team/monitoring/dismiss.team/" + testing_uid1,
			token:    testing_token_uid1,
			respCode: http.StatusInternalServerError,
			doBad:    true,
		},
		{
			desc:       "Store returns not found",
			method:     http.MethodPut,
			url:        "/dismiss/team/monitoring/dismiss.team/" + testing_uid1,
			token:      testing_token_uid1,
			respCode:   http.StatusNotFound,
			returnNone: true,
		},
	}

	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	//patient := store.Patient{}
	members := []store.Member{
		{
			UserID:           testing_token_hcp2,
			TeamID:           "dismiss.team",
			Role:             "admin",
			InvitationStatus: "accepted",
		},
	}
	notAdmin := []store.Member{
		{
			UserID:           testing_token_hcp2,
			TeamID:           "dismiss.team",
			Role:             "member",
			InvitationStatus: "accepted",
		},
	}
	team := store.Team{
		Name:        "Dismiss team monitoring",
		Description: "Dismiss team monitoring",
		Members:     members,
		ID:          "dismiss.team",
	}
	teamNotAdmin := store.Team{
		Name:        "Dismiss team monitoring",
		Description: "Dismiss team monitoring",
		Members:     notAdmin,
		ID:          "dismiss.team.not.admin",
	}
	//mockPerms.SetMockNextCall(testing_token_uid1+"dismiss.team", &patient, nil)
	mockPerms.SetMockNextCall(testing_token_hcp2+"dismiss.team", &team, nil)
	mockPerms.SetMockNextCall(testing_token_hcp2+"dismiss.team.not.admin", &teamNotAdmin, nil)

	mockPerms.On(
		"UpdatePatientMonitoringWithContext", mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)
	mockPerms.On(
		"GetPatientMonitoring", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(&store.Patient{}, nil)

	for idx, inviteTest := range inviteTests {
		mockAuth := NewAuthMock(inviteTest.token)
		// don't run a test if it says to skip it
		if inviteTest.skip {
			continue
		}
		var testRtr = mux.NewRouter()

		//default flow, fully authorized
		hydrophone := InitApi(
			FAKE_CONFIG,
			mockStore,
			mockNotifier,
			mock_uid1Shoreline,
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
				mock_uid1Shoreline,
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
				mock_uid1Shoreline,
				mockPerms,
				mockAuth,
				mockSeagull,
				mockPortal,
				mockTemplates,
				logger,
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
			request.Header.Set(TP_SESSION_TOKEN, inviteTest.token)
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

func TestSendMonitoringTeamInvite(t *testing.T) {
	tstRtr := initTestingRouterNoPerms()
	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "hcp"})

	// Malformed Requests
	wrongBodies := []testJSONObject{
		{"notMonitoringEnd": time.Now().Add(24 * time.Hour)},
		{"monitoringEnd": "AZERTY"},
	}

	for _, wrongBody := range wrongBodies {
		body := &bytes.Buffer{}
		json.NewEncoder(body).Encode(wrongBody)
		path := "/send/team/monitoring/TEAMID/USERID"
		request, _ := http.NewRequest("POST", path, body)
		request.Header.Set(TP_SESSION_TOKEN, testing_uid1)
		response := httptest.NewRecorder()
		tstRtr.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Logf("expected %d actual %d", http.StatusBadRequest, response.Code)
			t.Fail()
		}
	}

}
