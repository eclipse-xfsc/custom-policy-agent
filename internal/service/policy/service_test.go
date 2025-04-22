package policy_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	goapolicy "github.com/eclipse-xfsc/custom-policy-agent/gen/policy"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/header"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy/policyfakes"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	ptr "github.com/eclipse-xfsc/microservice-core-go/pkg/ptr"
)

func TestNew(t *testing.T) {
	svc := policy.New(context.Background(), nil, nil, nil, nil, "hostname.com", false, 10*time.Second, http.DefaultClient, zap.NewNop())
	assert.Implements(t, (*goapolicy.Service)(nil), svc)
}

// testReq prepares test request to be used in tests
func testReq() *goapolicy.EvaluateRequest {
	input := map[string]interface{}{"msg": "yes"}
	var body interface{} = input

	return &goapolicy.EvaluateRequest{
		Repository: "policies",
		Group:      "testgroup",
		PolicyName: "example",
		Version:    "1.0",
		Input:      &body,
		TTL:        ptr.Int(30),
	}
}

func TestService_Evaluate(t *testing.T) {
	testPolicy := &storage.Policy{
		Repository: "policies",
		Filename:   "policy.rego",
		Name:       "example",
		Group:      "testgroup",
		Version:    "1.0",
		Rego:       `package testgroup.example default allow = false allow { input.msg == "yes" }`,
		Locked:     false,
		LastUpdate: time.Now(),
	}

	// prepare test policy source code for the case when policy result must contain only the
	// value of a blank variable assignment
	testPolicyBlankAssignment := `package testgroup.example _ = {"hello":"world"}`

	// prepare test policy using static json data during evaluation
	testPolicyWithStaticData := `package testgroup.example default allow = false allow { data.msg == "hello world" }`

	// prepare test policy accessing headers during evaluation
	testPolicyAccessingHeaders := `package testgroup.example token := external.http.header("Authorization")`

	// prepare test request with empty body
	testEmptyReq := func() *goapolicy.EvaluateRequest {
		var body interface{}

		return &goapolicy.EvaluateRequest{
			Repository: "policies",
			Group:      "testgroup",
			PolicyName: "example",
			Version:    "1.0",
			Input:      &body,
			TTL:        ptr.Int(30),
		}
	}

	// prepare http.Request for tests
	httpReq := func() *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "my-token")
		return req
	}

	// prepare context containing headers
	ctxWithHeaders := func() context.Context {
		ctx := header.ToContext(context.Background(), httpReq())
		return ctx
	}

	tests := []struct {
		// test input
		name      string
		ctx       context.Context
		req       *goapolicy.EvaluateRequest
		storage   policy.Storage
		regocache policy.RegoCache
		cache     policy.Cache
		// expected result
		res     *goapolicy.EvaluateResult
		errkind errors.Kind
		errtext string
	}{
		{
			name: "policy is found in policyCache",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return testPolicy, true
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"allow": true},
			},
		},
		{
			name: "policy is not found",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New(errors.NotFound)
				},
			},
			res:     nil,
			errkind: errors.NotFound,
			errtext: "not found",
		},
		{
			name: "error getting policy from storage",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New("some error")
				},
			},
			res:     nil,
			errkind: errors.Unknown,
			errtext: "some error",
		},
		{
			name: "policy is locked",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: true}, nil
				},
			},
			res:     nil,
			errkind: errors.Forbidden,
			errtext: "policy is locked",
		},
		{
			name: "policy is found in storage and isn't locked",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return testPolicy, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"allow": true},
			},
		},
		{
			name: "policy is executed successfully, but storing the result in cache fails",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return testPolicy, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return errors.New("some error")
				},
			},
			errkind: errors.Unknown,
			errtext: "error storing policy result in cache",
		},
		{
			name: "policy with blank variable assignment is evaluated successfully",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository: "policies",
						Name:       "example",
						Group:      "testgroup",
						Version:    "1.0",
						Rego:       testPolicyBlankAssignment,
						Locked:     false,
						LastUpdate: time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"hello": "world"},
			},
		},
		{
			name: "policy is evaluated successfully with TTL sent in the request headers",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository: "policies",
						Name:       "example",
						Group:      "testgroup",
						Version:    "1.0",
						Rego:       testPolicyBlankAssignment,
						Locked:     false,
						LastUpdate: time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"hello": "world"},
			},
		},
		{
			name: "policy using static json data is evaluated successfully",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository: "policies",
						Name:       "example",
						Group:      "testgroup",
						Version:    "1.0",
						Rego:       testPolicyWithStaticData,
						Data:       `{"msg": "hello world"}`,
						Locked:     false,
						LastUpdate: time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"allow": true},
			},
		},
		{
			name: "policy accessing headers is evaluated successfully",
			ctx:  ctxWithHeaders(),
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository: "policies",
						Name:       "example",
						Group:      "testgroup",
						Version:    "1.0",
						Rego:       testPolicyAccessingHeaders,
						Locked:     false,
						LastUpdate: time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"token": "my-token"},
			},
		},
		{
			name: "policy with empty input is evaluated successfully",
			ctx:  ctxWithHeaders(),
			req:  testEmptyReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return testPolicy, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			res: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"allow": false},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				test.regocache,
				test.cache,
				nil,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)
			ctx := context.Background()
			if test.ctx != nil {
				ctx = test.ctx
			}
			res, err := svc.Evaluate(ctx, test.req)
			if err == nil {
				assert.Empty(t, test.errtext)
				assert.NotNil(t, res)

				assert.Equal(t, test.res.Result, res.Result)
				assert.NotEmpty(t, res.ETag)
			} else {
				e, ok := err.(*errors.Error)
				assert.True(t, ok)

				assert.Contains(t, e.Error(), test.errtext)
				assert.Equal(t, test.errkind, e.Kind)
				assert.Equal(t, test.res, res)
			}
		})
	}
}

func TestService_Validate(t *testing.T) {
	// prepare basic JSON schema
	jsonSchema := `
		{
		  "type": "object",
		  "properties": {
			"foo": {
			  "type": "string",
			  "minLength": 5
			}
		  },
		  "required": [
			"foo"
		  ]
		}
	`
	// prepare schema with specified $schema property
	jsonSchemaWithSchemaProperty := `
		{
		  "$schema": "http://json-schema.org/draft-04/schema#",
		  "type": "object",
		  "properties": {
			"foo": {
			  "type": "string",
			  "minLength": 5
			}
		  },
		  "required": [
			"foo"
		  ]
		}
	`

	tests := []struct {
		name      string
		req       *goapolicy.EvaluateRequest
		storage   policy.Storage
		regocache policy.RegoCache
		cache     policy.Cache
		// expected result
		evalRes *goapolicy.EvaluateResult
		errkind errors.Kind
		errtext string
	}{
		{
			name: "output validation schema is empty",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository:   "policies",
						Name:         "example",
						Group:        "testgroup",
						Version:      "1.0",
						Rego:         `package testgroup.example _ = {"hello":"world"}`,
						Locked:       false,
						OutputSchema: "",
						LastUpdate:   time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			errtext: "validation schema for policy output is not found",
			errkind: errors.BadRequest,
		},
		{
			name: "output validation schema is invalid JSON schema",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository:   "policies",
						Name:         "example",
						Group:        "testgroup",
						Version:      "1.0",
						Rego:         `package testgroup.example _ = {"hello":"world"}`,
						Locked:       false,
						OutputSchema: "invalid JSON schema",
						LastUpdate:   time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			errtext: "error compiling output validation schema",
			errkind: errors.Unknown,
		},
		{
			name: "policy output schema validation fails",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository:   "policies",
						Name:         "example",
						Group:        "testgroup",
						Version:      "1.0",
						Rego:         `package testgroup.example _ = {"foo":"bar"}`,
						Locked:       false,
						OutputSchema: jsonSchema,
						LastUpdate:   time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			errtext: "policy output schema validation failed",
			errkind: errors.Unknown,
		},
		{
			name: "policy output validation is successful",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository:   "policies",
						Name:         "example",
						Group:        "testgroup",
						Version:      "1.0",
						Rego:         `package testgroup.example _ = {"foo":"barbaz"}`,
						Locked:       false,
						OutputSchema: jsonSchema,
						LastUpdate:   time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			evalRes: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"foo": "barbaz"},
			},
		},
		{
			name: "policy output validation using explicit schema draft version is successful ",
			req:  testReq(),
			regocache: &policyfakes.FakeRegoCache{
				GetStub: func(key string) (*storage.Policy, bool) {
					return nil, false
				},
			},
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						Repository:   "policies",
						Name:         "example",
						Group:        "testgroup",
						Version:      "1.0",
						Rego:         `package testgroup.example _ = {"foo":"barbaz"}`,
						Locked:       false,
						OutputSchema: jsonSchemaWithSchemaProperty,
						LastUpdate:   time.Now(),
					}, nil
				},
			},
			cache: &policyfakes.FakeCache{
				SetStub: func(ctx context.Context, s string, s2 string, s3 string, bytes []byte, i int) error {
					return nil
				},
			},
			evalRes: &goapolicy.EvaluateResult{
				Result: map[string]interface{}{"foo": "barbaz"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				test.regocache,
				test.cache,
				nil,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)

			res, err := svc.Validate(context.Background(), test.req)
			if err == nil {
				assert.Empty(t, test.errtext)
				assert.NotNil(t, res)

				assert.Equal(t, test.evalRes.Result, res.Result)
				assert.NotEmpty(t, res.ETag)
			} else {
				e, ok := err.(*errors.Error)
				assert.True(t, ok)

				assert.Contains(t, e.Error(), test.errtext)
				assert.Equal(t, test.errkind, e.Kind)
				assert.Equal(t, test.evalRes, res)
			}
		})
	}
}

func TestService_Lock(t *testing.T) {
	// prepare test request to be used in tests
	testReq := func() *goapolicy.LockRequest {
		return &goapolicy.LockRequest{
			Group:      "testgroup",
			PolicyName: "example",
			Version:    "1.0",
		}
	}

	tests := []struct {
		name    string
		req     *goapolicy.LockRequest
		storage policy.Storage

		errkind errors.Kind
		errtext string
	}{
		{
			name: "policy is not found",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New(errors.NotFound)
				},
			},
			errkind: errors.NotFound,
			errtext: "not found",
		},
		{
			name: "error getting policy from storage",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New("some error")
				},
			},
			errkind: errors.Unknown,
			errtext: "some error",
		},
		{
			name: "policy is already locked",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: true}, nil
				},
			},
			errkind: errors.Forbidden,
			errtext: "policy is already locked",
		},
		{
			name: "fail to lock policy",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: false}, nil
				},
				SetPolicyLockStub: func(ctx context.Context, repository, name, group, version string, lock bool) error {
					return errors.New(errors.Internal, "error locking policy")
				},
			},
			errkind: errors.Internal,
			errtext: "error locking policy",
		},
		{
			name: "policy is locked successfully",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: false}, nil
				},
				SetPolicyLockStub: func(ctx context.Context, repository, name, group, version string, lock bool) error {
					return nil
				},
			},
			errtext: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				nil,
				nil,
				nil,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)
			err := svc.Lock(context.Background(), test.req)
			if err == nil {
				assert.Empty(t, test.errtext)
			} else {
				e, ok := err.(*errors.Error)
				assert.True(t, ok)

				assert.Contains(t, e.Error(), test.errtext)
				assert.Equal(t, test.errkind, e.Kind)
			}
		})
	}
}

func TestService_Unlock(t *testing.T) {
	// prepare test request to be used in tests
	testReq := func() *goapolicy.UnlockRequest {
		return &goapolicy.UnlockRequest{
			Repository: "policies",
			Group:      "testgroup",
			PolicyName: "example",
			Version:    "1.0",
		}
	}

	tests := []struct {
		name    string
		req     *goapolicy.UnlockRequest
		storage policy.Storage

		errkind errors.Kind
		errtext string
	}{
		{
			name: "policy is not found",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New(errors.NotFound)
				},
			},
			errkind: errors.NotFound,
			errtext: "not found",
		},
		{
			name: "error getting policy from storage",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New("some error")
				},
			},
			errkind: errors.Unknown,
			errtext: "some error",
		},
		{
			name: "policy is unlocked",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: false}, nil
				},
			},
			errkind: errors.Forbidden,
			errtext: "policy is unlocked",
		},
		{
			name: "fail to unlock policy",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: true}, nil
				},
				SetPolicyLockStub: func(ctx context.Context, repository, name, group, version string, lock bool) error {
					return errors.New(errors.Internal, "error unlocking policy")
				},
			},
			errkind: errors.Internal,
			errtext: "error unlocking policy",
		},
		{
			name: "policy is unlocked successfully",
			req:  testReq(),
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{Locked: true}, nil
				},
				SetPolicyLockStub: func(ctx context.Context, repository, name, group, version string, lock bool) error {
					return nil
				},
			},
			errtext: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				nil,
				nil,
				nil,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)
			err := svc.Unlock(context.Background(), test.req)
			if err == nil {
				assert.Empty(t, test.errtext)
			} else {
				e, ok := err.(*errors.Error)
				assert.True(t, ok)

				assert.Contains(t, e.Error(), test.errtext)
				assert.Equal(t, test.errkind, e.Kind)
			}
		})
	}
}

func TestService_ListPolicies(t *testing.T) {
	boolTrue := true
	boolFalse := false
	tests := []struct {
		name    string
		storage policy.Storage
		request *goapolicy.PoliciesRequest

		response *goapolicy.PoliciesResult
		errText  string
	}{
		{
			name: "storage return error",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return nil, fmt.Errorf("some error")
			}},
			request: &goapolicy.PoliciesRequest{
				Locked:     ptr.Bool(false),
				Rego:       ptr.Bool(false),
				Data:       ptr.Bool(false),
				DataConfig: ptr.Bool(false),
			},

			errText: "some error",
		},
		{
			name: "request without errors and any additional request parameter return all policies",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return []*storage.Policy{{
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some Data",
					DataConfig: "data config",
					Locked:     false,
					LastUpdate: time.Time{},
				}, {
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some data",
					DataConfig: "data config",
					Locked:     true,
					LastUpdate: time.Time{},
				}}, nil
			}},
			request: &goapolicy.PoliciesRequest{},

			response: &goapolicy.PoliciesResult{
				Policies: []*goapolicy.Policy{
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Locked:     false,
						LastUpdate: time.Time{}.Unix(),
					},
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Locked:     true,
						LastUpdate: time.Time{}.Unix(),
					},
				},
			},
		},
		{
			name: "request with only locked parameter equal to true returns only locked policies",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return []*storage.Policy{{
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some data",
					DataConfig: "data config",
					Locked:     true,
					LastUpdate: time.Time{},
				}}, nil
			}},
			request: &goapolicy.PoliciesRequest{
				Locked: &boolTrue,
			},

			response: &goapolicy.PoliciesResult{
				Policies: []*goapolicy.Policy{
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Locked:     true,
						LastUpdate: time.Time{}.Unix(),
					},
				},
			},
		},
		{
			name: "request with only locked parameter equal to false returns only unlocked policies",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return []*storage.Policy{{
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some data",
					DataConfig: "data config",
					Locked:     false,
					LastUpdate: time.Time{},
				}}, nil
			}},
			request: &goapolicy.PoliciesRequest{
				Locked: &boolFalse,
			},

			response: &goapolicy.PoliciesResult{
				Policies: []*goapolicy.Policy{
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Locked:     false,
						LastUpdate: time.Time{}.Unix(),
					},
				},
			},
		},
		{
			name: "request with all additional params set to true and policy name is blank",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return []*storage.Policy{{
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some data",
					DataConfig: "data config",
					Locked:     true,
					LastUpdate: time.Time{},
				}}, nil
			}},
			request: &goapolicy.PoliciesRequest{
				Locked:     ptr.Bool(true),
				PolicyName: ptr.String(""),
				Rego:       &boolTrue,
				Data:       &boolTrue,
				DataConfig: &boolTrue,
			},

			response: &goapolicy.PoliciesResult{
				Policies: []*goapolicy.Policy{
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Rego:       ptr.String("some rego"),
						Data:       ptr.String("some data"),
						DataConfig: ptr.String("data config"),
						Locked:     true,
						LastUpdate: time.Time{}.Unix(),
					},
				},
			},
		},
		{
			name: "request with all additional params set to false and policy name is blank",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return []*storage.Policy{{
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some data",
					DataConfig: "data config",
					Locked:     false,
					LastUpdate: time.Time{},
				}}, nil
			}},
			request: &goapolicy.PoliciesRequest{
				Locked:     ptr.Bool(false),
				PolicyName: ptr.String(""),
				Rego:       &boolFalse,
				Data:       &boolFalse,
				DataConfig: &boolFalse,
			},

			response: &goapolicy.PoliciesResult{
				Policies: []*goapolicy.Policy{
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Locked:     false,
						LastUpdate: time.Time{}.Unix(),
					},
				},
			},
		},
		{
			name: "request with policy name filter return every policy which contains the policy name",
			storage: &policyfakes.FakeStorage{GetPoliciesStub: func(ctx context.Context, b *bool, s *string) ([]*storage.Policy, error) {
				return []*storage.Policy{{
					Repository: "policies",
					Name:       "example",
					Group:      "example",
					Version:    "example",
					Rego:       "some rego",
					Data:       "some data",
					DataConfig: "data config",
					Locked:     false,
					LastUpdate: time.Time{},
				}}, nil
			}},
			request: &goapolicy.PoliciesRequest{
				PolicyName: ptr.String("exam"),
			},

			response: &goapolicy.PoliciesResult{
				Policies: []*goapolicy.Policy{
					{
						Repository: "policies",
						PolicyName: "example",
						Group:      "example",
						Version:    "example",
						Locked:     false,
						LastUpdate: time.Time{}.Unix(),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				nil,
				nil,
				nil,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)
			result, err := svc.ListPolicies(context.Background(), test.request)

			if test.errText != "" {
				assert.ErrorContains(t, err, test.errText)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, result, test.response)
			}
		})
	}
}

func TestService_SubscribeForPolicyChange(t *testing.T) {
	tests := []struct {
		name    string
		storage policy.Storage
		request *goapolicy.SubscribeRequest

		errText string
	}{
		{
			name: "policy doesn't exist",
			storage: &policyfakes.FakeStorage{PolicyStub: func(ctx context.Context, s1, s2, s3, s4 string) (*storage.Policy, error) {
				return nil, errors.New(errors.NotFound, "not found")
			}},
			request: &goapolicy.SubscribeRequest{
				WebhookURL: "http://some.url/example",
				Subscriber: "Subscriber Name",
				Repository: "policy repo",
				PolicyName: "policy name",
				Group:      "policy group",
				Version:    "policy version",
			},

			errText: "not found",
		},
		{
			name: "subscriber already exist",
			storage: &policyfakes.FakeStorage{SubscriberStub: func(ctx context.Context, s1, s2, s3, s4, s5, s6 string) (*storage.Subscriber, error) {
				return &storage.Subscriber{Name: "subscriber"}, nil
			}},
			request: &goapolicy.SubscribeRequest{
				WebhookURL: "http://some.url/example",
				Subscriber: "Subscriber Name",
				Repository: "policy repo",
				PolicyName: "policy name",
				Group:      "policy group",
				Version:    "policy version",
			},

			errText: "subscriber already exist",
		},
		{
			name: "error retrieving subscriber",
			storage: &policyfakes.FakeStorage{SubscriberStub: func(ctx context.Context, s1, s2, s3, s4, s5, s6 string) (*storage.Subscriber, error) {
				return nil, errors.New("some error")
			}},
			request: &goapolicy.SubscribeRequest{
				WebhookURL: "http://some.url/example",
				Subscriber: "Subscriber Name",
				Repository: "policy repo",
				PolicyName: "policy name",
				Group:      "policy group",
				Version:    "policy version",
			},

			errText: "some error",
		},
		{
			name: "error while creating subscriber",
			storage: &policyfakes.FakeStorage{CreateSubscriberStub: func(ctx context.Context, s *storage.Subscriber) (*storage.Subscriber, error) {
				return nil, fmt.Errorf("some error")
			}},
			request: &goapolicy.SubscribeRequest{
				WebhookURL: "http://some.url/example",
				Subscriber: "Subscriber Name",
				Repository: "policy repo",
				PolicyName: "policy name",
				Group:      "policy group",
				Version:    "policy version",
			},

			errText: "some error",
		},
		{
			name: "subscriber is created successfully",
			storage: &policyfakes.FakeStorage{CreateSubscriberStub: func(ctx context.Context, s *storage.Subscriber) (*storage.Subscriber, error) {
				return &storage.Subscriber{
					Name:             "Subscriber Name",
					WebhookURL:       "http://some.url/example",
					PolicyRepository: "policy repo",
					PolicyName:       "policy name",
					PolicyGroup:      "policy group",
					PolicyVersion:    "policy version",
					CreatedAt:        time.Time{},
					UpdatedAt:        time.Time{},
				}, nil
			}},
			request: &goapolicy.SubscribeRequest{
				WebhookURL: "http://some.url/example",
				Subscriber: "Subscriber Name",
				Repository: "policy repo",
				PolicyName: "policy name",
				Group:      "policy group",
				Version:    "policy version",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				nil,
				nil,
				nil,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)
			res, err := svc.SubscribeForPolicyChange(context.Background(), test.request)
			if test.errText != "" {
				assert.ErrorContains(t, err, test.errText)
				assert.Nil(t, res)
			} else {
				assert.NotNil(t, res)
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_ExportBundleError(t *testing.T) {
	t.Run("policy not found in storage", func(t *testing.T) {
		storage := &policyfakes.FakeStorage{
			PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
				return nil, errors.New(errors.NotFound, "policy not found")
			},
		}
		svc := policy.New(
			context.Background(),
			storage,
			nil,
			nil,
			nil,
			"https://policyservice.com",
			false,
			10*time.Second,
			http.DefaultClient,
			zap.NewNop(),
		)
		res, reader, err := svc.ExportBundle(context.Background(), &goapolicy.ExportBundleRequest{})
		assert.Nil(t, res)
		assert.Nil(t, reader)
		require.Error(t, err)
		assert.ErrorContains(t, err, "policy not found")
		e, ok := err.(*errors.Error)
		assert.True(t, ok)
		assert.True(t, errors.Is(errors.NotFound, e))
	})

	t.Run("error getting policy from storage", func(t *testing.T) {
		storage := &policyfakes.FakeStorage{
			PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
				return nil, errors.New("unexpected error")
			},
		}
		svc := policy.New(
			context.Background(),
			storage,
			nil,
			nil,
			nil,
			"https://policyservice.com",
			false,
			10*time.Second,
			http.DefaultClient,
			zap.NewNop(),
		)
		res, reader, err := svc.ExportBundle(context.Background(), &goapolicy.ExportBundleRequest{})
		assert.Nil(t, res)
		assert.Nil(t, reader)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unexpected error")
		e, ok := err.(*errors.Error)
		assert.True(t, ok)
		assert.True(t, errors.Is(errors.Unknown, e))
	})

	t.Run("undefined policy export configuration", func(t *testing.T) {
		storage := &policyfakes.FakeStorage{
			PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
				return &storage.Policy{
					Repository: "myrepo",
					Name:       "myname",
					Group:      "mygroup",
					Version:    "1.52",
					Rego:       "package test",
					Data:       `{"key":"value"}`,
					DataConfig: `{"new":"value"}`,
					Locked:     false,
					LastUpdate: time.Date(2023, 10, 8, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}

		svc := policy.New(
			context.Background(),
			storage,
			nil,
			nil,
			nil,
			"https://policyservice.com",
			false,
			10*time.Second,
			http.DefaultClient,
			zap.NewNop(),
		)
		res, reader, err := svc.ExportBundle(context.Background(), &goapolicy.ExportBundleRequest{})
		assert.Nil(t, res)
		assert.Nil(t, reader)
		require.Error(t, err)
		assert.ErrorContains(t, err, "policy export configuration is not defined")
	})

	t.Run("error making signature", func(t *testing.T) {
		storage := &policyfakes.FakeStorage{
			PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
				return &storage.Policy{
					Repository:   "myrepo",
					Name:         "myname",
					Group:        "mygroup",
					Version:      "1.52",
					Rego:         "package test",
					Data:         `{"key":"value"}`,
					DataConfig:   `{"new":"value"}`,
					ExportConfig: `{"namespace":"transit","key":"key1"}`,
					Locked:       false,
					LastUpdate:   time.Date(2023, 10, 8, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}

		signer := &policyfakes.FakeSigner{
			SignStub: func(ctx context.Context, namespace, key string, data []byte) ([]byte, error) {
				return nil, fmt.Errorf("error signing data")
			},
		}

		svc := policy.New(
			context.Background(),
			storage,
			nil,
			nil,
			signer,
			"https://policyservice.com",
			false,
			10*time.Second,
			http.DefaultClient,
			zap.NewNop(),
		)
		res, reader, err := svc.ExportBundle(context.Background(), &goapolicy.ExportBundleRequest{})
		assert.Nil(t, res)
		assert.Nil(t, reader)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error signing data")
	})
}

func TestService_ExportBundleSuccess(t *testing.T) {
	storage := &policyfakes.FakeStorage{
		PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
			return &storage.Policy{
				Repository:   "myrepo",
				Name:         "myname",
				Group:        "mygroup",
				Version:      "1.52",
				Rego:         "package test",
				Data:         `{"key":"value"}`,
				DataConfig:   `{"new":"value"}`,
				ExportConfig: `{"namespace":"transit","key":"key1"}`,
				Locked:       false,
				LastUpdate:   time.Date(2023, 10, 8, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	signer := &policyfakes.FakeSigner{
		SignStub: func(ctx context.Context, namespace, key string, data []byte) ([]byte, error) {
			return []byte("signature"), nil
		},
	}

	svc := policy.New(
		context.Background(),
		storage,
		nil,
		nil,
		signer,
		"https://policyservice.com",
		false,
		10*time.Second,
		http.DefaultClient,
		zap.NewNop(),
	)
	res, reader, err := svc.ExportBundle(context.Background(), &goapolicy.ExportBundleRequest{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, reader)

	assert.Equal(t, "application/zip", res.ContentType)
	assert.Equal(t, `attachment; filename="myrepo_mygroup_myname_1.52.zip"`, res.ContentDisposition)
	assert.NotZero(t, res.ContentLength)

	archive, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NotNil(t, archive)

	r, err := zip.NewReader(bytes.NewReader(archive), int64(res.ContentLength))
	require.NoError(t, err)
	require.NotNil(t, r)

	// check if policy_bundle.zip is present
	require.NotNil(t, r.File[0])
	require.Equal(t, policy.BundleFilename, r.File[0].Name)

	// check if policy_bundle.jws is present
	require.NotNil(t, r.File[1])
	require.Equal(t, policy.BundleSignatureFilename, r.File[1].Name)

	// check if signature matches the returned value from signer
	reader, err = r.File[1].Open()
	require.NoError(t, err)
	require.NotNil(t, reader)
	sig, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NotNil(t, sig)

	assert.Equal(t, []byte("signature"), sig)
}

func TestService_PolicyPublicKey(t *testing.T) {
	tests := []struct {
		name    string
		storage policy.Storage
		signer  policy.Signer
		key     any
		errtext string
		errkind errors.Kind
	}{
		{
			name: "policy not found",
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New(errors.NotFound, "policy not found")
				},
			},
			errtext: "policy not found",
			errkind: errors.NotFound,
		},
		{
			name: "failed to get policy from storage",
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return nil, errors.New("some internal error")
				},
			},
			errtext: "some internal error",
			errkind: errors.Unknown,
		},
		{
			name: "undefined policy export configuration",
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{}, nil
				},
			},
			errtext: "policy export configuration is not defined",
			errkind: errors.Forbidden,
		},
		{
			name: "wrong format for policy export configuration",
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						ExportConfig: "invalid",
					}, nil
				},
			},
			errtext: "cannot unmarshal policy export configuration",
			errkind: errors.Unknown,
		},
		{
			name: "fail to get key from signer",
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						ExportConfig: `{"namespace":"transit","key":"key1"}`,
					}, nil
				},
			},
			signer: &policyfakes.FakeSigner{
				KeyStub: func(ctx context.Context, s string, s2 string) (any, error) {
					return nil, fmt.Errorf("cannot get key")
				},
			},
			errtext: "cannot get key",
			errkind: errors.Unknown,
		},
		{
			name: "key is successfully returned",
			storage: &policyfakes.FakeStorage{
				PolicyStub: func(ctx context.Context, s string, s2 string, s3 string, s4 string) (*storage.Policy, error) {
					return &storage.Policy{
						ExportConfig: `{"namespace":"transit","key":"key1"}`,
					}, nil
				},
			},
			signer: &policyfakes.FakeSigner{
				KeyStub: func(ctx context.Context, s string, s2 string) (any, error) {
					return map[string]interface{}{"key": "ed25519"}, nil
				},
			},
			key: map[string]interface{}{"key": "ed25519"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := policy.New(
				context.Background(),
				test.storage,
				nil,
				nil,
				test.signer,
				"hostname.com",
				false,
				10*time.Second,
				http.DefaultClient,
				zap.NewNop(),
			)
			key, err := svc.PolicyPublicKey(context.Background(), &goapolicy.PolicyPublicKeyRequest{})
			if err != nil {
				require.NotEmpty(t, test.errtext)
				require.Contains(t, err.Error(), test.errtext)
				if e, ok := err.(*errors.Error); ok {
					assert.Equal(t, test.errkind, e.Kind)
				}
			} else {
				require.Empty(t, test.errtext)
				require.NotNil(t, key)
				assert.Equal(t, test.key, key)
			}
		})
	}
}

func TestService_SetPolicyAutoImport(t *testing.T) {
	tests := []struct {
		name    string
		req     *goapolicy.SetPolicyAutoImportRequest
		storage policy.Storage
		result  map[string]string
		errtext string
		errkind errors.Kind
	}{
		{
			name: "missing interval",
			req: &goapolicy.SetPolicyAutoImportRequest{
				PolicyURL: "http://example.com",
				Interval:  "",
			},
			errtext: "invalid interval definition: time: invalid duration",
			errkind: errors.BadRequest,
		},
		{
			name: "invalid interval without unit",
			req: &goapolicy.SetPolicyAutoImportRequest{
				PolicyURL: "http://example.com",
				Interval:  "1",
			},
			errtext: "invalid interval definition: time: missing unit in duration",
			errkind: errors.BadRequest,
		},
		{
			name: "invalid interval duration",
			req: &goapolicy.SetPolicyAutoImportRequest{
				PolicyURL: "http://example.com",
				Interval:  "s",
			},
			errtext: "invalid interval definition: time: invalid duration",
			errkind: errors.BadRequest,
		},
		{
			name: "error saving autoimport configuration",
			req: &goapolicy.SetPolicyAutoImportRequest{
				PolicyURL: "http://example.com",
				Interval:  "1m",
			},
			storage: &policyfakes.FakeStorage{
				SaveAutoImportConfigStub: func(ctx context.Context, autoImport *storage.PolicyAutoImport) error {
					return fmt.Errorf("some error")
				},
			},
			errtext: "some error",
			errkind: errors.Unknown,
		},
		{
			name: "autoimport is saved successfully",
			req: &goapolicy.SetPolicyAutoImportRequest{
				PolicyURL: "http://example.com",
				Interval:  "1m",
			},
			storage: &policyfakes.FakeStorage{
				SaveAutoImportConfigStub: func(ctx context.Context, autoImport *storage.PolicyAutoImport) error {
					return nil
				},
			},
			result: map[string]string{
				"policyURL": "http://example.com",
				"interval":  "1m",
			},
		},
	}

	for _, test := range tests {
		svc := policy.New(
			context.Background(),
			test.storage,
			nil,
			nil,
			nil,
			"hostname.com",
			false,
			10*time.Second,
			http.DefaultClient,
			zap.NewNop(),
		)

		res, err := svc.SetPolicyAutoImport(context.Background(), test.req)
		if err != nil {
			require.NotEmpty(t, test.errtext)
			assert.Contains(t, err.Error(), test.errtext)
			assert.True(t, errors.Is(test.errkind, err), fmt.Sprintf("error kind must be: %v", test.errkind))
			assert.Nil(t, test.result)
		} else {
			require.Empty(t, test.errtext, "error is not expected, but got: %v", err)
			assert.Equal(t, test.result, res)
		}
	}
}
