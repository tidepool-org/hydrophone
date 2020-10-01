package api

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
	"go.uber.org/fx"

	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
)

var (

	// MockConfigModule
	MockConfigModule = fx.Options(fx.Supply(Config{
		ServerSecret: "shhh! don't tell",
	}))

	MockShorelineModule = fx.Options(fx.Provide(func() shoreline.Client { return shoreline.NewMock(testing_token) }))

	MockGatekeeperModule = fx.Options(fx.Provide(func() commonClients.Gatekeeper {
		return commonClients.NewGatekeeperMock(nil, &status.StatusError{Status: status.NewStatus(500, "Unable to parse response.")})
	}))

	MockMetricsModule = fx.Options(fx.Provide(func() highwater.Client {
		return highwater.NewMock()
	}))

	MockSeagullModule = fx.Options(fx.Provide(func() commonClients.Seagull {
		return commonClients.NewSeagullMock()
	}))

	// MockTemplates
	MockTemplatesModule = fx.Options(fx.Supply(models.Templates{}))

	//MockNoPermsGatekeeperModule mocks gatekeeper
	MockNoPermsGatekeeperModule = fx.Options(fx.Provide(func() commonClients.Gatekeeper {
		return commonClients.NewGatekeeperMock(commonClients.Permissions{"upload": commonClients.Permission{"userid": "other-id"}}, nil)
	}))

	ResponsableGatekeeperModule = fx.Options(fx.Provide(func() commonClients.Gatekeeper {
		return NewResponsableMockGatekeeper()
	}))

	BaseModule = fx.Options(
		clients.MockNotifierModule,
		MockShorelineModule,
		MockMetricsModule,
		MockSeagullModule,
		MockTemplatesModule,
		MockConfigModule,
		fx.Provide(NewApi),
		fx.Provide(mux.NewRouter),
	)

	ResponableModule = fx.Options(
		clients.MockStoreFailsModule,
		ResponsableGatekeeperModule,
		BaseModule,
	)
)

func TestGetStatus_StatusOk(t *testing.T) {

	var api Api
	_ = fx.New(
		clients.MockStoreModule,
		MockGatekeeperModule,
		BaseModule,
		fx.Populate(&api),
	)

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()
	api.IsAlive(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusOK)
	}
}

func TestGetStatus_StatusInternalServerError(t *testing.T) {

	var api *Api
	_ = fx.New(
		clients.MockStoreFailsModule,
		MockGatekeeperModule,
		BaseModule,
		fx.Populate(&api),
	)

	request, _ := http.NewRequest("GET", "/status", nil)
	response := httptest.NewRecorder()

	api.IsReady(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusInternalServerError)
	}

	body, _ := ioutil.ReadAll(response.Body)

	if string(body) != `{"code":500,"reason":"Session failure"}` {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "Session failure")
	}
}

func (i *testJSONObject) deepCompare(j *testJSONObject) string {
	for k := range *i {
		if reflect.DeepEqual((*i)[k], (*j)[k]) == false {
			return fmt.Sprintf("`%s` expected `%v` actual `%v` ", k, (*j)[k], (*i)[k])
		}
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////

func T_ExpectResponsablesEmpty(t *testing.T) {

	var gk commonClients.Gatekeeper
	_ = fx.New(ResponsableGatekeeperModule,
		fx.Populate(&gk),
	)
	responsableGatekeeper := gk.(*ResponsableMockGatekeeper)

	if responsableGatekeeper.HasResponses() {
		if len(responsableGatekeeper.UserInGroupResponses) > 0 {
			t.Logf("UserInGroupResponses still available")
		}
		if len(responsableGatekeeper.UsersInGroupResponses) > 0 {
			t.Logf("UsersInGroupResponses still available")
		}
		if len(responsableGatekeeper.SetPermissionsResponses) > 0 {
			t.Logf("SetPermissionsResponses still available")
		}
		responsableGatekeeper.Reset()
		t.Fail()
	}
}

func Test_TokenUserHasRequestedPermissions_Server(t *testing.T) {

	var responsableHydrophone *Api
	fx.New(
		ResponableModule,
		fx.Populate(&responsableHydrophone),
	)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: true}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, requestedPermissions) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_Owner(t *testing.T) {
	var responsableHydrophone *Api
	fx.New(
		ResponableModule,
		fx.Populate(&responsableHydrophone),
	)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "abcdef1234", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, requestedPermissions) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_GatekeeperError(t *testing.T) {
	var responsableHydrophone *Api
	var gk commonClients.Gatekeeper
	fx.New(
		ResponableModule,
		fx.Populate(&responsableHydrophone),
		fx.Populate(&gk),
	)
	responsableGatekeeper := gk.(*ResponsableMockGatekeeper)

	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{}, errors.New("ERROR")}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err == nil {
		t.Fatalf("Unexpected success")
	}
	if err.Error() != "ERROR" {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if len(permissions) != 0 {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_CompleteMismatch(t *testing.T) {
	var responsableHydrophone *Api
	var gk commonClients.Gatekeeper
	fx.New(
		ResponableModule,
		fx.Populate(&responsableHydrophone),
		fx.Populate(&gk),
	)
	responsableGatekeeper := gk.(*ResponsableMockGatekeeper)

	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{"y": commonClients.Allowed, "z": commonClients.Allowed}, nil}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if len(permissions) != 0 {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_PartialMismatch(t *testing.T) {
	var responsableHydrophone *Api
	var gk commonClients.Gatekeeper
	fx.New(
		ResponableModule,
		fx.Populate(&responsableHydrophone),
		fx.Populate(&gk),
	)
	responsableGatekeeper := gk.(*ResponsableMockGatekeeper)

	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{"a": commonClients.Allowed, "z": commonClients.Allowed}, nil}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, commonClients.Permissions{"a": commonClients.Allowed}) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}

func Test_TokenUserHasRequestedPermissions_FullMatch(t *testing.T) {
	var responsableHydrophone *Api
	var gk commonClients.Gatekeeper
	fx.New(
		ResponableModule,
		fx.Populate(&responsableHydrophone),
		fx.Populate(&gk),
	)
	responsableGatekeeper := gk.(*ResponsableMockGatekeeper)
	responsableGatekeeper.UserInGroupResponses = []PermissionsResponse{{commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}, nil}}
	defer T_ExpectResponsablesEmpty(t)

	tokenData := &shoreline.TokenData{UserID: "abcdef1234", IsServer: false}
	requestedPermissions := commonClients.Permissions{"a": commonClients.Allowed, "b": commonClients.Allowed}
	permissions, err := responsableHydrophone.tokenUserHasRequestedPermissions(tokenData, "1234567890", requestedPermissions)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !reflect.DeepEqual(permissions, requestedPermissions) {
		t.Fatalf("Unexpected permissions returned: %#v", permissions)
	}
}
