package regofunc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
)

func TestTaskFuncs_CreateTask(t *testing.T) {
	tests := []struct {
		name        string
		taskName    interface{}
		input       map[string]interface{}
		taskHandler func(w http.ResponseWriter, r *http.Request)

		response map[string]interface{}
		errtext  string
	}{
		{
			name:  "task not found",
			input: map[string]interface{}{"test": 123},
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"task not found"}`))
			},
			errtext: "task.create: unexpected response code: 404",
		},
		{
			name:  "task service returns error",
			input: map[string]interface{}{"test": 123},
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			errtext: "task.create: unexpected response code: 500",
		},
		{
			name:  "task service returns invalid JSON response",
			input: nil,
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("boom"))
			},
			response: nil,
			errtext:  "task.create: invalid character",
		},
		{
			name:  "task is created successfully",
			input: nil,
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"taskID":"hello"}`))
			},
			response: map[string]interface{}{"taskID": "hello"},
			errtext:  "",
		},
	}

	for _, test := range tests {
		srv := httptest.NewServer(http.HandlerFunc(test.taskHandler))
		taskFuncs := regofunc.NewTaskFuncs(srv.URL, http.DefaultClient)

		query, err := rego.New(
			rego.Query(`task.create("taskName", input)`),
			rego.Function2(taskFuncs.CreateTaskFunc()),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(context.Background())
		assert.NoError(t, err)

		resultSet, err := query.Eval(context.Background(), rego.EvalInput(test.input))
		if test.errtext != "" {
			assert.Nil(t, resultSet)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.errtext)
		} else {
			assert.NoError(t, err)
			assert.NotEmpty(t, resultSet)
			assert.NotEmpty(t, resultSet[0].Expressions)
			assert.Equal(t, test.response, resultSet[0].Expressions[0].Value)
		}
	}
}

func TestTaskFuncs_CreateTaskList(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		taskHandler func(w http.ResponseWriter, r *http.Request)

		response map[string]interface{}
		errtext  string
	}{
		{
			name:  "taskList not found",
			input: map[string]interface{}{"test": 123},
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"taskList not found"}`))
			},
			errtext: "tasklist.create: unexpected response code: 404",
		},
		{
			name:  "task service returns error",
			input: map[string]interface{}{"test": 123},
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			errtext: "tasklist.create: unexpected response code: 500",
		},
		{
			name:  "task service returns invalid JSON response",
			input: nil,
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("boom"))
			},
			response: nil,
			errtext:  "tasklist.create: invalid character",
		},
		{
			name:  "taskList is created successfully",
			input: nil,
			taskHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"taskListID":"hello"}`))
			},
			response: map[string]interface{}{"taskListID": "hello"},
			errtext:  "",
		},
	}

	for _, test := range tests {
		srv := httptest.NewServer(http.HandlerFunc(test.taskHandler))
		taskFuncs := regofunc.NewTaskFuncs(srv.URL, http.DefaultClient)

		query, err := rego.New(
			rego.Query(`tasklist.create("taskListName", input)`),
			rego.Function2(taskFuncs.CreateTaskListFunc()),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(context.Background())
		assert.NoError(t, err)

		resultSet, err := query.Eval(context.Background(), rego.EvalInput(test.input))
		if test.errtext != "" {
			assert.Nil(t, resultSet)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.errtext)
		} else {
			assert.NoError(t, err)
			assert.NotEmpty(t, resultSet)
			assert.NotEmpty(t, resultSet[0].Expressions)
			assert.Equal(t, test.response, resultSet[0].Expressions[0].Value)
		}
	}
}
