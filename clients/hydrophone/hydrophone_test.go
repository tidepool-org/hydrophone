package hydrophone

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mdblp/hydrophone/models"
)

func buildServer(t *testing.T, userID string, testToken string, confirmType string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		urlPath := req.URL.Path
		if strings.HasPrefix(urlPath, "/"+confirmType+"/") {
			if req.Method != "GET" && req.Method != "PUT" {
				t.Errorf("Incorrect HTTP Method [%s]", req.Method)
			} else if req.Header.Get("x-tidepool-session-token") != testToken {
				res.WriteHeader(http.StatusUnauthorized)
			} else {
				userID = strings.TrimPrefix(urlPath, "/"+confirmType+"/")
				if req.Method == "GET" {
					switch userID {
					case "authorizedWithData":
						res.WriteHeader(http.StatusOK)
						if confirmType == "invite" {
							fmt.Fprint(res, `[{"key":"key1","type":"medicalteam_invitation"}, {"key":"key2","type":"medicalteam_do_admin"}]`)
						} else {
							fmt.Fprint(res, `[{"key":"key3","type":"signup_confirmation"}]`)
						}
					case "authorizedWithoutData":
						res.WriteHeader(http.StatusNotFound)
					case "error":
						res.WriteHeader(http.StatusInternalServerError)
					}
				} else {
					switch userID {
					case "error":
						res.WriteHeader(http.StatusInternalServerError)
					default:
						res.WriteHeader(http.StatusOK)
					}
				}

			}
		} else {
			t.Errorf("Unknown path[%s]", urlPath)
		}
	}))
}

func TestGetPendingInvitations(t *testing.T) {
	testToken := "a.b.c"
	var userID string
	srvr := buildServer(t, userID, testToken, "invite")
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

	testWrongToken(t, hydrophoneClient, "invite")
	testEmptyData(t, hydrophoneClient, testToken, "invite")
	testError(t, hydrophoneClient, testToken, "invite")
}

func TestGetPendingSignup(t *testing.T) {
	testToken := "a.b.c"
	var userID string
	srvr := buildServer(t, userID, testToken, "signup")
	defer srvr.Close()

	hydrophoneClient := NewHydrophoneClientBuilder().
		WithHost(srvr.URL).
		Build()

	confirms, err := hydrophoneClient.GetPendingSignup("authorizedWithData", testToken)
	if err != nil {
		t.Errorf("Failed GetPendingSignup with error[%v]", err)
	}
	if confirms.Key != "key3" && confirms.Type != "signup_confirmation" {
		t.Errorf("Failed GetPendingSignup wrong data returned, first element: %v", confirms)
	}

	testWrongToken(t, hydrophoneClient, "signup")
	testEmptyData(t, hydrophoneClient, testToken, "signup")
	testError(t, hydrophoneClient, testToken, "signup")
}

func TestCancelSignup(t *testing.T) {
	testToken := "a.b.c"
	var userID string
	srvr := buildServer(t, userID, testToken, "signup")
	defer srvr.Close()

	hydrophoneClient := NewHydrophoneClientBuilder().
		WithHost(srvr.URL).
		Build()

	err := hydrophoneClient.CancelSignup(models.Confirmation{UserId: "randomUserId"}, testToken)
	if err != nil {
		t.Errorf("Failed CancelSignup with error[%v]", err)
	}

	err = hydrophoneClient.CancelSignup(models.Confirmation{UserId: "randomUserId"}, "wrongToken")
	if err == nil {
		t.Error("Unauthorized request should return an error")
	}

	err = hydrophoneClient.CancelSignup(models.Confirmation{UserId: "error"}, testToken)
	if err == nil {
		t.Error("Error from service should be forwarded")
	}
}

func testWrongToken(t *testing.T, hydrophoneClient *Client, confirmType string) {
	userID := "authorizedWithData"
	testToken := "wrong.token"
	confirms, err := getPending(hydrophoneClient, userID, testToken, confirmType)
	if err == nil {
		t.Error("Unauthorized request should return an error")
	}
	if !reflect.ValueOf(confirms).IsNil() {
		t.Error("When unauthorized no confirmations should be sent")
	}
}

func testEmptyData(t *testing.T, hydrophoneClient *Client, testToken string, confirmType string) {
	userID := "authorizedWithoutData"
	confirms, err := getPending(hydrophoneClient, userID, testToken, confirmType)
	if err != nil {
		t.Errorf("Failed empty test with error[%v]", err)
	}

	switch v := confirms.(type) {
	case *models.Confirmation:
		if v != nil {
			t.Error("empty test returned non nil confirmation")
		}
	case []models.Confirmation:
		length := len(v)
		if length != 0 {
			t.Errorf("empty test returned %v elements expected %v", length, 0)
		}
	default:
		t.Errorf("unknown type")
	}
}

func testError(t *testing.T, hydrophoneClient *Client, testToken string, confirmType string) {
	userID := "error"
	confirms, err := getPending(hydrophoneClient, userID, testToken, confirmType)
	if err == nil {
		t.Error("Error from service should be forwarded")
	}
	if !reflect.ValueOf(confirms).IsNil() {
		t.Error("On error no confirmations should be sent")
	}
}

func getPending(hydrophoneClient *Client, userID string, token string, confirmType string) (interface{}, error) {
	if confirmType == "signup" {
		return hydrophoneClient.GetPendingSignup(userID, token)
	} else {
		return hydrophoneClient.GetPendingInvitations(userID, token)
	}
}
