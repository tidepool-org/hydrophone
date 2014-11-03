package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
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
			PasswordReset:  `{{define "reset_test"}} {{ .Email }} {{ .Key }} {{end}}{{template "reset_test" .}}`,
			CareteamInvite: `{{define "invite_test"}} {{ .CareteamName }} {{ .Key }} {{end}}{{template "invite_test" .}}`,
			Confirmation:   `{{define "confirm_test"}} {{ .UserId }} {{ .Key }} {{end}}{{template "confirm_test" .}}`,
		},
		InviteTimeoutDays: 7,
		ResetTimeoutDays:  7,
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

type (
	//common test structure
	toTest struct {
		skip       bool
		returnNone bool
		method     string
		url        string
		body       jo
		token      string
		respCode   int
		response   jo
	}
	// These two types make it easier to define blobs of json inline.
	// We don't use the types defined by the API because we want to
	// be able to test with partial data structures.
	// jo is a generic json object
	jo map[string]interface{}

	// and ja is a generic json array
	ja []interface{}
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

	if string(body) != `{"code":500,"reason":"Session failure"}` {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "Session failure")
	}
}

func (i *jo) deepCompare(j *jo) string {
	for k, _ := range *i {
		if reflect.DeepEqual((*i)[k], (*j)[k]) == false {
			return fmt.Sprintf("for [%s] was [%v] expected [%v] ", k, (*i)[k], (*j)[k])
		}
	}
	return ""
}
