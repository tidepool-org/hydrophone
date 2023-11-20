package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"go.uber.org/fx"

	clinicsClient "github.com/tidepool-org/clinic/client"
	commonClients "github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/highwater"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/hydrophone/clients"
	"github.com/tidepool-org/hydrophone/models"
	"github.com/tidepool-org/hydrophone/testutil"
	"github.com/tidepool-org/platform/alerts"
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

	MockAlertsModule = fx.Options(fx.Provide(func() AlertsClient {
		return newMockAlertsClient()
	}))

	MockMetricsModule = fx.Options(fx.Provide(func() highwater.Client {
		return highwater.NewMock()
	}))

	MockSeagullModule = fx.Options(fx.Provide(func() commonClients.Seagull {
		return commonClients.NewSeagullMock()
	}))

	MockClinicsModule = fx.Options(
		fx.Provide(func(t *testing.T) *gomock.Controller {
			return gomock.NewController(t)
		}),
		fx.Provide(func(ctrl *gomock.Controller) clinicsClient.ClientWithResponsesInterface {
			return clinicsClient.NewMockClientWithResponsesInterface(ctrl)
		}),
	)

	// MockTemplates
	MockTemplatesModule = fx.Options(fx.Supply(models.Templates{}))

	//MockNoPermsGatekeeperModule mocks gatekeeper
	MockNoPermsGatekeeperModule = fx.Options(fx.Provide(func() commonClients.Gatekeeper {
		return commonClients.NewGatekeeperMock(commonClients.Permissions{"upload": commonClients.Permission{"userid": "other-id"}}, nil)
	}))

	ResponsableGatekeeperModule = fx.Options(fx.Provide(func() commonClients.Gatekeeper {
		return NewResponsableMockGatekeeper()
	}))

	BaseModuleWithLog = func(rw io.ReadWriter) fx.Option {
		return fx.Options(
			clients.MockNotifierModule,
			MockShorelineModule,
			MockMetricsModule,
			MockSeagullModule,
			MockAlertsModule,
			MockTemplatesModule,
			MockConfigModule,
			fx.Supply(fx.Annotate(rw, fx.As(new(io.ReadWriter)))),
			fx.Provide(testutil.NewLoggerWithReadWriter),
			fx.Provide(NewApi),
			fx.Provide(mux.NewRouter),
		)
	}

	BaseModule = BaseModuleWithLog(os.Stderr)

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
		MockClinicsModule,
		fx.Supply(t),
		fx.Populate(&api),
	)

	request := MustRequest(t, "GET", "/status", nil)
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
		MockClinicsModule,
		fx.Supply(t),
		fx.Populate(&api),
	)

	request := MustRequest(t, "GET", "/status", nil)
	response := httptest.NewRecorder()

	api.IsReady(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Resp given [%d] expected [%d] ", response.Code, http.StatusInternalServerError)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading response body: %s", err)
	}

	if string(body) != `{"code":500,"reason":"store connectivity failure"}` {
		t.Fatalf("Message given [%s] expected [%s] ", string(body), "store connectivity failure")
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
		MockClinicsModule,
		fx.Supply(t),
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
		MockClinicsModule,
		fx.Supply(t),
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
		MockClinicsModule,
		fx.Supply(t),
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
		MockClinicsModule,
		fx.Supply(t),
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

func TestAddUserIDToLogger(s *testing.T) {
	s.Run("is request specific (and thread-safe)", func(t *testing.T) {
		// This test is designed to try to exacerbate thread-safety issues in
		// Api logging. Unfortunately, it can't 100% reliably produce an error
		// in the event of a race condition.
		//
		// As a result, if this test is flapping, that's a strong indicator
		// that there's a thread-safety issue. A symptom of these flaps is
		// having the test fail some number of times, then randomly pass, and
		// then stay passing. This can be caused by Go caching the test result
		// (it's not actually running it again, it just reports the previous
		// success). Use the -count=1 flag to go test to force the test to be
		// re-run.
		userIDs := []string{"foo", "bar", "baz", "quux"}
		vars := testutil.WithRotatingVar("userId", userIDs)
		ht := newHydrophoneTest(t)
		handler := ht.handlerWithSync(len(userIDs))

		logData := ht.captureLogs(func() {
			ts := ht.Server(handler, ht.Api.addUserIDToLogger, vars)
			for i := 0; i < len(userIDs); i++ {
				go ts.Client().Get(ts.URL)
			}
			ht.syncer.Sync()
		})

		for _, userID := range userIDs {
			expected := fmt.Sprintf(`"userId": "%s"`, userID)
			if strings.Count(logData, expected) != 1 {
				t.Errorf("expected 1x field %s, got:\n%s", expected, logData)
			}
		}
	})

	s.Run("includes the userId", func(t *testing.T) {
		vars := testutil.WithRotatingVars(map[string]string{"userId": "foo"})
		ht := newHydrophoneTest(t)

		logData := ht.captureLogs(func() {
			ts := ht.Server(ht.handlerLog(), ht.Api.addUserIDToLogger, vars)
			if _, err := ts.Client().Get(ts.URL); err != nil {
				t.Errorf("expected no error, got: %s", err)
			}
		})

		expected := `"userId": "foo"`
		if !strings.Contains(logData, expected) {
			t.Errorf("expected field %s, got: %s", expected, logData)
		}
	})
}

type mockAlertsClient struct{}

func newMockAlertsClient() *mockAlertsClient {
	return &mockAlertsClient{}
}

func (c *mockAlertsClient) Upsert(_ context.Context, _ *alerts.Config) error {
	return nil
}

func (c *mockAlertsClient) Delete(_ context.Context, _ *alerts.Config) error {
	return nil
}

type mockAlertsClientWithFailingUpsert struct {
	AlertsClient
}

func newMockAlertsClientWithFailingUpsert() *mockAlertsClientWithFailingUpsert {
	return &mockAlertsClientWithFailingUpsert{
		AlertsClient: newMockAlertsClient(),
	}
}

func (c *mockAlertsClientWithFailingUpsert) Upsert(_ context.Context, _ *alerts.Config) error {
	return fmt.Errorf("this should not be called")
}

// MustRequest is a helper for tests that fails the test when request creation
// fails.
func MustRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("error creating http.Request: %s", err)
	}
	return r
}

// hydrophoneTest bundles useful scaffolding for hydrophone tests.
type hydrophoneTest struct {
	*testing.T
	Api    *Api
	logBuf *bytes.Buffer
	syncer *syncer
}

// newHydrophoneTest handles creating the scaffolding for testing a hydrophone
// handler.
func newHydrophoneTest(t *testing.T) *hydrophoneTest {
	var api *Api

	logBuf := &bytes.Buffer{}
	fx.New(
		clients.MockStoreFailsModule,
		MockGatekeeperModule,
		BaseModuleWithLog(logBuf),
		MockClinicsModule,
		fx.Supply(t),
		fx.Populate(&api),
	)

	return &hydrophoneTest{
		T:      t,
		Api:    api,
		logBuf: logBuf,
	}
}

// Server provides a test server, with the provided middleware applied to each
// request.
func (ht *hydrophoneTest) Server(h http.Handler, middleware ...mux.MiddlewareFunc) *httptest.Server {
	var combined http.Handler = h
	for _, m := range middleware {
		combined = m(combined)
	}
	ts := httptest.NewServer(combined)
	ht.Cleanup(ts.Close)
	return ts
}

// handlerOK just responds with a bare 200 header.
func (ht *hydrophoneTest) handlerOK() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// handlerLog simply logs "test"
func (ht *hydrophoneTest) handlerLog() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ht.Api.logger(r.Context()).Info("test")
		ht.handlerOK().ServeHTTP(w, r)
	})
}

// handlerWithSync will cause the test server to wait until size requests have
// been made before allowing any of them to finish.
func (ht *hydrophoneTest) handlerWithSync(size int) http.Handler {
	ht.syncer = newSyncer(size)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ht.handlerLog().ServeHTTP(w, r)
		ht.syncer.Wait()
	})
}

// captureLogs returns the output written to the logs from within f.
func (ht *hydrophoneTest) captureLogs(f func()) string {
	ht.Helper()
	prev := ht.logBuf.Len()
	f()
	return ht.logBuf.String()[prev:]
}

// syncer synchonizes two goroutines.
//
// It can be useful to produce race conditions.
type syncer struct {
	size    int
	waiting chan struct{}
	done    chan struct{}

	mu sync.Mutex
}

func newSyncer(size int) *syncer {
	return &syncer{
		size:    size,
		waiting: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (s *syncer) Wait() {
	<-s.waiting
	<-s.done
}

func (s *syncer) Sync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < s.size; i++ {
		s.waiting <- struct{}{}
	}
	close(s.done)
}
