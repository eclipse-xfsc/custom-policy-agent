package regofunc

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
)

func GetHeaderFunc(headers map[string]string) (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "external.http.header",
			Decl:    types.NewFunction(types.Args(types.S), types.S),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, paramHeader *ast.Term) (*ast.Term, error) {
			var header string
			if err := ast.As(paramHeader.Value, &header); err != nil {
				return nil, fmt.Errorf("invalid header parameter: %s", err)
			}

			headerValue := headers[header]
			v, err := ast.InterfaceToValue(headerValue)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}
