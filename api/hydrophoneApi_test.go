package api

import (
	"./../clients"
	"./../models"
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	MAKE_IT_FAIL = true
	FAKE_TOKEN   = "a.fake.token.to.use.in.tests"
)

var (
	NO_PARAMS = map[string]string{}

	FAKE_CONFIG = Config{
		ServerSecret: "shhh! don't tell",
	}
	/*
	 * users and tokens
	 */
	SRVR_TOKEN, _ = models.NewSessionToken(sverTknData, FAKE_CONFIG.Secret)
	/*
	 * basics setup
	 */
	rtr          = mux.NewRouter()
	mockNotifier = clients.NewMockNotifier()
	/*
	 * expected path
	 */
	mockStore  = clients.NewMockStoreClient(FAKE_CONFIG.Salt, false, false)
	hydrophone = InitApi(FAKE_CONFIG, mockStore, mockNotifier)
	/*
	 * failure path
	 */
	mockStoreFails  = clients.NewMockStoreClient(FAKE_CONFIG.Salt, false, MAKE_IT_FAIL)
	hydrophoneFails = InitApi(FAKE_CONFIG, mockStoreFails, mockNotifier)
)

func TestGetStatus_StatusOk(t *testing.T) {

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	hydrophone.SetHandlers("", rtr)

	hydrophone.GetStatus(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Resp given [%s] expected [%s] ", response.Code, http.StatusOK)
	}

}

func TestGetStatus_StatusInternalServerError(t *testing.T) {

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

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

func TestEmailAddress_StatusUnauthorized_WhenNoToken(t *testing.T) {
	request, _ := http.NewRequest("PUT", "/email", nil)
	response := httptest.NewRecorder()

	hydrophone.SetHandlers("", rtr)

	hydrophone.EmailAddress(response, request, NO_PARAMS)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("Non-expected status code%v:\n\tbody: %v", http.StatusUnauthorized, response.Code)
	}
}

func TestEmailAddress_StatusBadRequest_WhenNoVariablesPassed(t *testing.T) {
	request, _ := http.NewRequest("PUT", "/email", nil)
	request.Header.Set(TP_SESSION_TOKEN, FAKE_TOKEN)
	response := httptest.NewRecorder()

	hydrophone.SetHandlers("", rtr)

	hydrophone.EmailAddress(response, request, NO_PARAMS)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("Non-expected status code%v:\n\tbody: %v", http.StatusBadRequest, response.Code)
	}
}

func TestEmailAddress_StatusOK(t *testing.T) {
	request, _ := http.NewRequest("POST", "/email", nil)
	request.Header.Set(TP_SESSION_TOKEN, FAKE_TOKEN)
	response := httptest.NewRecorder()

	hydrophone.SetHandlers("", rtr)

	hydrophone.EmailAddress(response, request, map[string]string{"type": "password", "address": "test@user.org"})

	if response.Code != http.StatusOK {
		t.Fatalf("Non-expected status code%v:\n\tbody: %v", http.StatusNotImplemented, response.Code)
	}
}
