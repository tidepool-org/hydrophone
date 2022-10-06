package api

// import (
// 	"bytes"
// 	"encoding/json"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"testing"

// 	"github.com/gorilla/mux"
// 	"github.com/mdblp/go-common/clients/status"
// 	"github.com/mdblp/hydrophone/templates"
// 	. "github.com/mdblp/seagull/schema"
// 	"github.com/mdblp/shoreline/token"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// )

// func Test_SendNotification(t *testing.T) {
// 	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
// 	if found {
// 		FAKE_CONFIG.I18nTemplatesPath = templatesPath
// 	}
// 	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

// 	mockAuth.ExpectedCalls = nil
// 	mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: true})
// 	mockShoreline.On("TokenProvide").Return(testing_token_uid1)
// 	var testRtr = mux.NewRouter()
// 	mockSeagull.On("GetCollections", "sd454fgrgr84dfg", []string{"preferences", "profile"}).Return(&SeagullDocument{Preferences: &Preferences{}}, nil)
// 	hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockPerms, mockAuth, mockSeagull, mockPortal, mockTemplates, logger)
// 	hydrophone.SetHandlers("", testRtr)
// 	payload := map[string]string{
// 		"id":            "123456789",
// 		"code":          "987321456",
// 		"patientEmail":  "moi@dbl.de",
// 		"prescriptorId": "sd454fgrgr84dfg",
// 		"product":       "DBLG2",
// 	}
// 	var body = &bytes.Buffer{}

// 	t.Run("Message OK", func(t *testing.T) {

// 		json.NewEncoder(body).Encode(payload)
// 		request, _ := http.NewRequest("POST", "/notifications/submit_app_prescription", body)
// 		request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
// 		response := httptest.NewRecorder()
// 		testRtr.ServeHTTP(response, request)

// 		assert.Equal(t, response.Code, 200)
// 	})
// 	t.Run("Wrong topic", func(t *testing.T) {
// 		request, _ := http.NewRequest("POST", "/notifications/not_exist", &bytes.Buffer{})
// 		request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
// 		response := httptest.NewRecorder()
// 		testRtr.ServeHTTP(response, request)
// 		assert.Equal(t, response.Code, 400)
// 		verifyResultBody(t, response.Body, &status.StatusError{Status: status.NewStatus(http.StatusBadRequest, STATUS_WRONG_NOTIFICATION_TOPIC)})
// 	})
// 	t.Run("Wrong Body", func(t *testing.T) {
// 		request, _ := http.NewRequest("POST", "/notifications/submit_app_prescription", &bytes.Buffer{})
// 		request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
// 		response := httptest.NewRecorder()
// 		testRtr.ServeHTTP(response, request)

// 		assert.Equal(t, response.Code, 400)
// 	})
// 	t.Run("Incomplete Request", func(t *testing.T) {

// 		json.NewEncoder(body).Encode(payload)
// 		request, _ := http.NewRequest("POST", "/notifications/submit_app_prescription", body)
// 		request.Header.Set(TP_SESSION_TOKEN, testing_token_uid1)
// 		response := httptest.NewRecorder()
// 		testRtr.ServeHTTP(response, request)

// 		assert.Equal(t, response.Code, 400)
// 	})
// 	t.Run("Not a server request", func(t *testing.T) {
// 		mockAuth.ExpectedCalls = nil
// 		mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false})
// 		request, _ := http.NewRequest("POST", "/notifications/submit_app_prescription", &bytes.Buffer{})
// 		request.Header.Set(TP_SESSION_TOKEN, testing_token_hcp)
// 		response := httptest.NewRecorder()
// 		testRtr.ServeHTTP(response, request)
// 		assert.Equal(t, 403, response.Code)
// 		verifyResultBody(t, response.Body, &status.StatusError{Status: status.NewStatus(http.StatusForbidden, STATUS_UNAUTHORIZED)})
// 	})
// }

// func verifyResultBody(t *testing.T, body *bytes.Buffer, expected *status.StatusError) {
// 	if body.Len() != 0 {
// 		var result = &status.StatusError{}
// 		err := json.NewDecoder(body).Decode(result)
// 		if err != nil {
// 			t.Errorf(err.Error())
// 		}

// 		assert.EqualValues(t, expected, result)
// 	}
// }
