package regofunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
)

type CacheFuncs struct {
	cacheAddr  string
	httpClient *http.Client
}

func NewCacheFuncs(cacheAddr string, httpClient *http.Client) *CacheFuncs {
	return &CacheFuncs{
		cacheAddr:  cacheAddr,
		httpClient: httpClient,
	}
}

func (cf *CacheFuncs) CacheGetFunc() (*rego.Function, rego.Builtin3) {
	return &rego.Function{
			Name:    "cache.get",
			Decl:    types.NewFunction(types.Args(types.S, types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, a, b, c *ast.Term) (*ast.Term, error) {
			if cf.cacheAddr == "" {
				return nil, fmt.Errorf("trying to use cache.get Rego function, but cache address is not set")
			}

			var key, namespace, scope string

			if err := ast.As(a.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key: %s", err)
			} else if err = ast.As(b.Value, &namespace); err != nil {
				return nil, fmt.Errorf("invalid namespace: %s", err)
			} else if err = ast.As(c.Value, &scope); err != nil {
				return nil, fmt.Errorf("invalid scope: %s", err)
			}

			req, err := http.NewRequest("GET", cf.cacheAddr+"/v1/cache", nil)
			req.Header = http.Header{
				"x-cache-key":       []string{key},
				"x-cache-namespace": []string{namespace},
				"x-cache-scope":     []string{scope},
			}
			if err != nil {
				return nil, err
			}

			resp, err := cf.httpClient.Do(req.WithContext(bctx.Context))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
				return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, resp.Status)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}

func (cf *CacheFuncs) CacheSetFunc() (*rego.Function, rego.Builtin4) {
	return &rego.Function{
			Name:    "cache.set",
			Decl:    types.NewFunction(types.Args(types.S, types.S, types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, k, n, s, d *ast.Term) (*ast.Term, error) {
			if cf.cacheAddr == "" {
				return nil, fmt.Errorf("trying to use cache.set Rego function, but cache address is not set")
			}

			var key, namespace, scope string
			var data map[string]interface{}

			if err := ast.As(k.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key: %s", err)
			} else if err = ast.As(n.Value, &namespace); err != nil {
				return nil, fmt.Errorf("invalid namespace: %s", err)
			} else if err = ast.As(s.Value, &scope); err != nil {
				return nil, fmt.Errorf("invalid scope: %s", err)
			} else if err = ast.As(d.Value, &data); err != nil {
				return nil, fmt.Errorf("invalid data: %s", err)
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequest("POST", cf.cacheAddr+"/v1/cache", bytes.NewReader(jsonData))
			if err != nil {
				return nil, err
			}

			req.Header = http.Header{
				"x-cache-key":       []string{key},
				"x-cache-namespace": []string{namespace},
				"x-cache-scope":     []string{scope},
			}

			resp, err := cf.httpClient.Do(req.WithContext(bctx.Context))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response code: %d", resp.StatusCode)
			}

			var val ast.Value
			val, err = ast.InterfaceToValue("success")
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}
