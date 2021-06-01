package hydrophone

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetPendingInvitations(t *testing.T) {
	testToken := "a.b.c"
	var userID string
	srvr := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		urlPath := req.URL.Path
		if strings.HasPrefix(urlPath, "/invite/") {
			if req.Method != "GET" {
				t.Errorf("Incorrect HTTP Method [%s]", req.Method)
			} else if req.Header.Get("x-tidepool-session-token") != testToken {
				res.WriteHeader(http.StatusUnauthorized)
			} else {
				userID = strings.TrimPrefix(urlPath, "/invite/")
				switch userID {
				case "authorizedWithData":
					res.WriteHeader(http.StatusOK)
					fmt.Fprint(res, `[{"key":"key1","type":"medicalteam_invitation"}, {"key":"key2","type":"medicalteam_do_admin"}]`)
				case "authorizedWithoutData":
					res.WriteHeader(http.StatusNotFound)
				case "error":
					res.WriteHeader(http.StatusInternalServerError)
				}
			}
		} else {
			t.Errorf("Unknown path[%s]", urlPath)
		}
	}))
	defer srvr.Close()

	hydrophoneClient := NewHydrophoneClientBuilder().
		WithHost(srvr.URL).
		Build()

	confirms, err := hydrophoneClient.GetPendingInvitations("authorizedWithData", testToken)
	if err != nil {
		t.Errorf("Failed GetPendingInvitations with error[%v]", err)
	}
	if len(confirms) != 2 {
		t.Errorf("Failed GetPendingInvitations returned %v elements expected %v", len(confirms), 2)
	}
	if confirms[0].Key != "key1" && confirms[0].Type != "medicalteam_invitation" {
		t.Errorf("Failed GetPendingInvitations wrong data returned, first element: %v", confirms[0])
	}
	if confirms[1].Key != "key2" && confirms[0].Type != "medicalteam_do_admin" {
		t.Errorf("Failed GetPendingInvitations wrong data returned, second element: %v", confirms[1])
	}

	confirms, err = hydrophoneClient.GetPendingInvitations("authorizedWithData", "wrong.token")
	if err == nil {
		t.Error("Failed GetPendingInvitations unauthorized request should return an error")
	}
	if confirms != nil {
		t.Error("Failed GetPendingInvitations when unauthorized no confirmations should be sent")
	}

	confirms, err = hydrophoneClient.GetPendingInvitations("authorizedWithoutData", testToken)
	if err != nil {
		t.Errorf("Failed GetPendingInvitations with error[%v]", err)
	}
	if len(confirms) != 0 {
		t.Errorf("Failed GetPendingInvitations returned %v elements expected %v", len(confirms), 0)
	}

	confirms, err = hydrophoneClient.GetPendingInvitations("error", testToken)
	if err == nil {
		t.Error("Failed GetPendingInvitations error from service should be forwarded")
	}
	if confirms != nil {
		t.Error("Failed GetPendingInvitations on error no confirmations should be sent")
	}
}
