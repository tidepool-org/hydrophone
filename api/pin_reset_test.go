package api

import (
	"bytes"
	"encoding/json"
	"errors"
	orcaSchema "github.com/mdblp/orca/schema"
	"github.com/mdblp/tide-whisperer-v2/v2/client/tidewhisperer"
	tide "github.com/mdblp/tide-whisperer-v2/v2/schema"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mdblp/hydrophone/templates"
	. "github.com/mdblp/seagull/schema"
	"github.com/mdblp/shoreline/token"
	"github.com/stretchr/testify/mock"
)

type (
	pinResetTest struct {
		test            toTest
		patientSettings *tide.SettingsResult
		portalErr       bool // bools are false by default
	}
)

func beforeTests() {
	// for these tests series, we do not consider server token for shoreline
	mockShoreline.On("TokenProvide").Return(testing_token)
	mockShoreline.IsServer = false
	mockAuth.ExpectedCalls = nil
}

func afterTests() {
	// for these tests series, we do not consider server token for shoreline
	mockShoreline.IsServer = true
}

func TestPinResetResponds(t *testing.T) {
	pinResetTests := []pinResetTest{
		{
			// if you leave off the /{userid}, it goes 404 (not found)
			test: toTest{
				returnNone: true,
				method:     "POST",
				url:        "/send/pin-reset",
				token:      testing_token_uid1,
				respCode:   404,
			},
		},
		{
			// need a token or get 401 (unauthorized)
			test: toTest{
				returnNone: true,
				method:     "POST",
				url:        "/send/pin-reset/" + testing_uid1,
				respCode:   401,
			},
		},
		{
			// wrong/inexisting user goes 400 (bad request)
			test: toTest{

				returnNone: true,
				method:     "POST",
				url:        "/send/pin-reset/NotFound",
				token:      testing_token_uid1,
				respCode:   400,
			},
		},
		{
			// if portal will return an error, it goes 500
			test: toTest{
				returnNone: true,
				method:     "POST",
				url:        "/send/pin-reset/" + testing_uid1,
				token:      testing_token_uid1,
				respCode:   500,
			},
			portalErr: true,
		},
		{
			// if the patient's IMEI is blank it goes 500 (cannot generate TOTP)
			test: toTest{
				returnNone: true,
				method:     "POST",
				url:        "/send/pin-reset/" + testing_uid1,
				token:      testing_token_uid1,
				respCode:   500,
			},
			patientSettings: &tide.SettingsResult{
				TimedCurrentSettings: orcaSchema.TimedCurrentSettings{
					CurrentSettings: orcaSchema.CurrentSettings{
						Device: &orcaSchema.Device{
							Imei: "",
						},
					},
				},
			},
		},
		{
			// testing too many pin reset sends
			test: toTest{
				method:                     "POST",
				url:                        "/send/pin-reset/" + testing_uid1,
				token:                      testing_token_uid1,
				respCode:                   403,
				counterLatestConfirmations: 11,
			},
		},
		{
			// everything OK goes 200
			test: toTest{
				returnNone: true,
				method:     "POST",
				url:        "/send/pin-reset/" + testing_uid1,
				token:      testing_token_uid1,
				respCode:   200,
			},
			patientSettings: &tide.SettingsResult{
				TimedCurrentSettings: orcaSchema.TimedCurrentSettings{
					CurrentSettings: orcaSchema.CurrentSettings{
						Device: &orcaSchema.Device{
							Imei: "123456789012345",
						},
					},
				},
			},
		},
	}

	templatesPath, found := os.LookupEnv("TEMPLATE_PATH")
	if found {
		FAKE_CONFIG.I18nTemplatesPath = templatesPath
	}
	mockTemplates, _ = templates.New(FAKE_CONFIG.I18nTemplatesPath, mockLocalizer)

	beforeTests()

	for idx, pinResetTest := range pinResetTests {
		medicalDataMock = tidewhisperer.NewMock()

		// don't run a test if it says to skip it
		if pinResetTest.test.skip {
			continue
		}
		mockAuth.ExpectedCalls = nil
		if pinResetTest.test.token == "" {
			mockAuth.On("Authenticate", mock.Anything).Return(nil)
		} else {
			mockAuth.On("Authenticate", mock.Anything).Return(&token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"})
		}

		var testRtr = mux.NewRouter()
		// if the token is not provided, shoreline will consider the requester as unauthorized
		if pinResetTest.test.token == "" {
			mockShoreline.Unauthorized = true
		} else {
			mockShoreline.Unauthorized = false
		}

		// Mock an error from portal if need be
		if pinResetTest.portalErr == true {
			medicalDataMock.On("GetSettings", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("fatal error"))
		}

		// Mock a fake Patient config in portal if need be
		if pinResetTest.patientSettings != nil {
			medicalDataMock.On("GetSettings", mock.Anything, mock.Anything, mock.Anything).Return(pinResetTest.patientSettings, nil)
		}
		mockSeagull.On("GetCollections", testing_uid1, []string{"preferences"}).Return(&SeagullDocument{Preferences: &Preferences{}}, nil)

		//testing when there is nothing to return from the store
		if pinResetTest.test.returnNone {
			mockStoreEmpty.CounterLatestConfirmations = pinResetTest.test.counterLatestConfirmations
			hydrophoneFindsNothing := InitApi(FAKE_CONFIG, mockStoreEmpty, mockNotifier, mockShoreline, mockPerms, mockAuth, mockSeagull, medicalDataMock, mockTemplates, logger)
			hydrophoneFindsNothing.SetHandlers("", testRtr)
		} else {
			mockStore.CounterLatestConfirmations = pinResetTest.test.counterLatestConfirmations
			hydrophone := InitApi(FAKE_CONFIG, mockStore, mockNotifier, mockShoreline, mockPerms, mockAuth, mockSeagull, medicalDataMock, mockTemplates, logger)
			hydrophone.SetHandlers("", testRtr)
		}

		var body = &bytes.Buffer{}
		// build the body only if there is one defined in the test
		if len(pinResetTest.test.body) != 0 {
			json.NewEncoder(body).Encode(pinResetTest.test.body)
		}
		request, _ := http.NewRequest(pinResetTest.test.method, pinResetTest.test.url, body)
		if pinResetTest.test.token != "" {
			request.Header.Set(TP_SESSION_TOKEN, pinResetTest.test.token)
		}
		response := httptest.NewRecorder()
		testRtr.ServeHTTP(response, request)

		if response.Code != pinResetTest.test.respCode {
			t.Logf("TestId `%d` `%s` expected `%d` actual `%d`", idx, pinResetTest.test.desc, pinResetTest.test.respCode, response.Code)
			t.Fail()
		}

		if response.Body.Len() != 0 && len(pinResetTest.test.response) != 0 {
			var result = &testJSONObject{}
			err := json.NewDecoder(response.Body).Decode(result)
			if err != nil {
				//TODO: not dealing with arrays at the moment ....
				if err.Error() != "json: cannot unmarshal array into Go value of type api.testJSONObject" {
					t.Logf("TestId `%d` `%s` errored `%s` body `%v`", idx, pinResetTest.test.desc, err.Error(), response.Body)
					t.Fail()
				}
			}

			if cmp := result.deepCompare(&pinResetTest.test.response); cmp != "" {
				t.Logf("TestId `%d` `%s` URL `%s` body `%s`", idx, pinResetTest.test.desc, pinResetTest.test.url, cmp)
				t.Fail()
			}
		}
	}

	afterTests()
}
