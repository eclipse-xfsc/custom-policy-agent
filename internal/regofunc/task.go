package regofunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
)

type TaskFuncs struct {
	taskAddr   string
	httpClient *http.Client
}

func NewTaskFuncs(taskAddr string, httpClient *http.Client) *TaskFuncs {
	return &TaskFuncs{
		taskAddr:   taskAddr,
		httpClient: httpClient,
	}
}

// CreateTaskFunc returns a rego function for creating tasks.
func (t *TaskFuncs) CreateTaskFunc() (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "task.create",
			Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, taskName, taskData *ast.Term) (*ast.Term, error) {
			if t.taskAddr == "" {
				return nil, fmt.Errorf("trying to use task.create Rego function, but task address is not set")
			}

			var name string
			var data map[string]interface{}

			if err := ast.As(taskName.Value, &name); err != nil {
				return nil, fmt.Errorf("invalid task name: %s", err)
			} else if err = ast.As(taskData.Value, &data); err != nil {
				return nil, fmt.Errorf("invalid data: %s", err)
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}

			fullURL := fmt.Sprintf("%s/v1/task/%s", t.taskAddr, name)
			u, err := url.ParseRequestURI(fullURL)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequest("POST", u.String(), bytes.NewReader(jsonData))
			if err != nil {
				return nil, err
			}

			resp, err := t.httpClient.Do(req.WithContext(bctx.Context))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response code: %d", resp.StatusCode)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}

// CreateTaskListFunc returns a rego function for creating task lists.
func (t *TaskFuncs) CreateTaskListFunc() (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "tasklist.create",
			Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, taskListName, taskListData *ast.Term) (*ast.Term, error) {
			if t.taskAddr == "" {
				return nil, fmt.Errorf("trying to use tasklist.create Rego function, but task address is not set")
			}

			var name string
			var data map[string]interface{}

			if err := ast.As(taskListName.Value, &name); err != nil {
				return nil, fmt.Errorf("invalid taskList name: %s", err)
			} else if err = ast.As(taskListData.Value, &data); err != nil {
				return nil, fmt.Errorf("invalid data: %s", err)
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}

			fullURL := fmt.Sprintf("%s/v1/taskList/%s", t.taskAddr, name)
			u, err := url.ParseRequestURI(fullURL)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequest("POST", u.String(), bytes.NewReader(jsonData))
			if err != nil {
				return nil, err
			}

			resp, err := t.httpClient.Do(req.WithContext(bctx.Context))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response code: %d", resp.StatusCode)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}
