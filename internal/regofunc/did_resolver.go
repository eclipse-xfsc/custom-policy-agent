package regofunc

import (
	"fmt"
	"net/http"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

type DIDResolverFuncs struct {
	resolverAddr string
	httpClient   *http.Client
}

func NewDIDResolverFuncs(resolverAddr string, httpClient *http.Client) *DIDResolverFuncs {
	return &DIDResolverFuncs{
		resolverAddr: resolverAddr,
		httpClient:   httpClient,
	}
}

func (dr *DIDResolverFuncs) ResolveFunc() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "did.resolve",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
			var DID string

			if err := ast.As(a.Value, &DID); err != nil {
				return nil, fmt.Errorf("invalid DID: %s", err)
			}
			if DID == "" {
				return nil, errors.New("DID cannot be empty")
			}

			req, err := http.NewRequest("GET", dr.resolverAddr+"/1.0/identifiers/"+DID, nil)
			if err != nil {
				return nil, err
			}

			resp, err := dr.httpClient.Do(req.WithContext(bctx.Context))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}
