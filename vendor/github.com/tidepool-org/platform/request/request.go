package request

import (
	"context"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"

	"github.com/tidepool-org/platform/errors"
)

const (
	HTTPHeaderTraceRequest = "X-Tidepool-Trace-Request"
	HTTPHeaderTraceSession = "X-Tidepool-Trace-Session"
)

func CopyTrace(ctx context.Context, req *http.Request) error {
	if ctx == nil {
		return errors.New("context is missing")
	}
	if req == nil {
		return errors.New("request is missing")
	}

	if traceRequest := TraceRequestFromContext(ctx); traceRequest != "" {
		req.Header.Add(HTTPHeaderTraceRequest, traceRequest)
	}
	if traceSession := TraceSessionFromContext(ctx); traceSession != "" {
		req.Header.Add(HTTPHeaderTraceSession, traceSession)
	}

	return nil
}

func IsStatusCodeSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode <= 299
}

func IsStatusCodeRedirection(statusCode int) bool {
	return statusCode >= 300 && statusCode <= 399
}

func IsStatusCodeClientError(statusCode int) bool {
	return statusCode >= 400 && statusCode <= 499
}

type Method string

const (
	MethodServiceSecret   Method = "service secret"
	MethodAccessToken     Method = "access token"
	MethodSessionToken    Method = "session token"
	MethodRestrictedToken Method = "restricted token"
)

type Details interface {
	Method() Method

	IsService() bool
	IsUser() bool
	UserID() string

	HasToken() bool
	Token() string
}

func NewDetails(method Method, userID string, token string) Details {
	return &details{
		method: method,
		userID: userID,
		token:  token,
	}
}

type details struct {
	method Method
	userID string
	token  string
}

func (d *details) Method() Method {
	return d.method
}

func (d *details) IsService() bool {
	return d.method == MethodServiceSecret || (d.method == MethodSessionToken && d.userID == "")
}

func (d *details) IsUser() bool {
	return !d.IsService()
}

func (d *details) UserID() string {
	return d.userID
}

func (d *details) HasToken() bool {
	return d.method != MethodServiceSecret
}

func (d *details) Token() string {
	return d.token
}

func DecodeRequestPathParameter(req *rest.Request, key string, validator func(value string) bool) (string, error) {
	if req == nil {
		return "", errors.New("request is missing")
	}

	value, ok := req.PathParams[key]
	if !ok || value == "" {
		return "", ErrorParameterMissing(key)
	} else if validator != nil && !validator(value) {
		return "", ErrorParameterInvalid(key)
	}
	return value, nil
}

func DecodeOptionalRequestPathParameter(req *rest.Request, key string, validator func(value string) bool) (*string, error) {
	if req == nil {
		return nil, errors.New("request is missing")
	}

	value, ok := req.PathParams[key]
	if !ok || value == "" {
		return nil, nil
	} else if validator != nil && !validator(value) {
		return nil, ErrorParameterInvalid(key)
	}
	return &value, nil
}

type contextKey string

const detailsContextKey contextKey = "details"

func NewContextWithDetails(ctx context.Context, details Details) context.Context {
	return context.WithValue(ctx, detailsContextKey, details)
}

func DetailsFromContext(ctx context.Context) Details {
	if ctx != nil {
		if details, ok := ctx.Value(detailsContextKey).(Details); ok {
			return details
		}
	}
	return nil
}

const traceRequestContextKey contextKey = "trace-request"

func NewContextWithTraceRequest(ctx context.Context, traceRequest string) context.Context {
	return context.WithValue(ctx, traceRequestContextKey, traceRequest)
}

func TraceRequestFromContext(ctx context.Context) string {
	if ctx != nil {
		if traceRequest, ok := ctx.Value(traceRequestContextKey).(string); ok {
			return traceRequest
		}
	}
	return ""
}

const traceSessionContextKey contextKey = "trace-session"

func NewContextWithTraceSession(ctx context.Context, traceSession string) context.Context {
	return context.WithValue(ctx, traceSessionContextKey, traceSession)
}

func TraceSessionFromContext(ctx context.Context) string {
	if ctx != nil {
		if traceSession, ok := ctx.Value(traceSessionContextKey).(string); ok {
			return traceSession
		}
	}
	return ""
}

const contextErrorContextKey contextKey = "context-error"

type ContextError struct {
	err error
}

func NewContextError() *ContextError {
	return &ContextError{}
}

func (c *ContextError) Get() error {
	return c.err
}

func (c *ContextError) Set(err error) {
	c.err = err
}

func NewContextWithContextError(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextErrorContextKey, NewContextError())
}

func ContextErrorFromContext(ctx context.Context) *ContextError {
	if ctx != nil {
		if contextError, ok := ctx.Value(contextErrorContextKey).(*ContextError); ok {
			return contextError
		}
	}
	return nil
}

func GetErrorFromContext(ctx context.Context) error {
	if contextError := ContextErrorFromContext(ctx); contextError != nil {
		return contextError.Get()
	}
	return nil
}

func SetErrorToContext(ctx context.Context, err error) {
	if contextError := ContextErrorFromContext(ctx); contextError != nil {
		contextError.Set(err)
	}
}
