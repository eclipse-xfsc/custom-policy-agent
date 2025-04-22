package regofunc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
)

func TestCacheGetFunc(t *testing.T) {
	expected := `{"taskID":"deadbeef"}`
	cacheSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, expected)
	}))
	defer cacheSrv.Close()

	cacheFuncs := regofunc.NewCacheFuncs(cacheSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`cache.get("open-policy-agent", "opa", "111")`),
		rego.Function3(cacheFuncs.CacheGetFunc()),
	)
	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}

func TestCacheSetFuncSuccess(t *testing.T) {
	expected := "success"
	cacheSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedRequestBody := `{"test":123}`
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		bodyString := string(bodyBytes)
		if bodyString != expectedRequestBody {
			assert.Equal(t, expectedRequestBody, bodyString)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer cacheSrv.Close()

	cacheFuncs := regofunc.NewCacheFuncs(cacheSrv.URL, http.DefaultClient)

	input := map[string]interface{}{"test": 123}
	query, err := rego.New(
		rego.Query(`cache.set("open-policy-agent", "opa", "111", input)`),
		rego.Function4(cacheFuncs.CacheSetFunc()),
	).PrepareForEval(context.Background())
	assert.NoError(t, err)

	resultSet, err := query.Eval(context.Background(), rego.EvalInput(input))
	assert.NoError(t, err)
	assert.NotEmpty(t, resultSet)
	assert.NotEmpty(t, resultSet[0].Expressions)
	assert.Equal(t, expected, resultSet[0].Expressions[0].Value)
}

func TestCacheSetFuncError(t *testing.T) {
	cacheSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedRequestBody := "test"
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		bodyString := string(bodyBytes)
		assert.Equal(t, expectedRequestBody, bodyString)

		w.WriteHeader(http.StatusNotFound)
	}))
	defer cacheSrv.Close()

	cacheFuncs := regofunc.NewCacheFuncs(cacheSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`cache.set("open-policy-agent", "opa", "111", "test")`),
		rego.Function4(cacheFuncs.CacheSetFunc()),
	)

	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, resultSet)
}
