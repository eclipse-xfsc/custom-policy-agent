package regofunc

import (
	"fmt"
	"net/http"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"

	"github.com/eclipse-xfsc/microservice-core-go/pkg/ocm"
)

type OcmFuncs struct {
	addr   string
	client *ocm.Client
}

func NewOcmFuncs(ocmAddr string, httpClient *http.Client) *OcmFuncs {
	ocmClient := ocm.New(ocmAddr, ocm.WithHTTPClient(httpClient))

	return &OcmFuncs{addr: ocmAddr, client: ocmClient}
}

func (of *OcmFuncs) GetLoginProofInvitation() (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "ocm.getLoginProofInvitation",
			Decl:    types.NewFunction(types.Args(types.A, types.A), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, rScopes *ast.Term, scopesMap *ast.Term) (*ast.Term, error) {
			if of.addr == "" {
				return nil, fmt.Errorf("trying to use ocm.getLoginProofInvitation Rego function, but ocm address is not set")
			}

			var scopes []string
			var scopeToType map[string]string

			if err := ast.As(rScopes.Value, &scopes); err != nil {
				return nil, fmt.Errorf("invalid scopes array: %s", err)
			} else if err = ast.As(scopesMap.Value, &scopeToType); err != nil {
				return nil, fmt.Errorf("invalid scope to credential type map: %s", err)
			}

			var credTypes []string
			distinctTypes := make(map[string]bool, len(scopeToType))
			for _, scope := range scopes {
				credType, ok := scopeToType[scope]
				if !ok {
					return nil, fmt.Errorf("scope not found in scope to type map: %s", scope)
				}
				if credType != "" && !distinctTypes[credType] {
					credTypes = append(credTypes, credType)
				}
				distinctTypes[credType] = true
			}

			if len(credTypes) == 0 {
				return nil, fmt.Errorf("no credential types found in the scope to type map: %s", scopeToType)
			}

			res, err := of.client.GetLoginProofInvitation(bctx.Context, credTypes)
			if err != nil {
				return nil, err
			}

			type result struct {
				Link      string `json:"link"`
				RequestID string `json:"requestId"`
			}
			var val ast.Value
			val, err = ast.InterfaceToValue(result{
				Link:      res.Data.PresentationMessage,
				RequestID: res.Data.ProofRecordID,
			})
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}

func (of *OcmFuncs) SendPresentationRequest() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "ocm.sendPresentationRequest",
			Decl:    types.NewFunction(types.Args(types.A), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, request *ast.Term) (*ast.Term, error) {
			if of.addr == "" {
				return nil, fmt.Errorf("trying to use ocm.sendPresentationRequest Rego function, but ocm address is not set")
			}

			var req map[string]interface{}
			if err := ast.As(request.Value, &req); err != nil {
				return nil, fmt.Errorf("invalid request map: %s", err)
			}

			res, err := of.client.SendOutOfBandRequest(bctx.Context, req)
			if err != nil {
				return nil, err
			}

			type result struct {
				Link      string `json:"link"`
				RequestID string `json:"requestId"`
			}
			var val ast.Value
			val, err = ast.InterfaceToValue(result{
				Link:      res.Data.PresentationMessage,
				RequestID: res.Data.ProofRecordID,
			})
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}

func (of *OcmFuncs) GetLoginProofResult() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "ocm.getLoginProofResult",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, id *ast.Term) (*ast.Term, error) {
			if of.addr == "" {
				return nil, fmt.Errorf("trying to use ocm.getLoginProofResult Rego function, but ocm address is not set")
			}

			var presentationID string

			if err := ast.As(id.Value, &presentationID); err != nil {
				return nil, fmt.Errorf("invalid presentationId: %s", err)
			}

			res, err := of.client.GetLoginProofResult(bctx.Context, presentationID)
			if err != nil {
				return nil, err
			}

			claims := map[string]interface{}{}
			for _, pres := range res.Data.Presentations {
				for cName, cValue := range pres.Claims {
					claims[cName] = cValue
				}
			}

			var val ast.Value
			val, err = ast.InterfaceToValue(claims)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}

func (of *OcmFuncs) GetRawProofResult() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "ocm.getRawProofResult",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, id *ast.Term) (*ast.Term, error) {
			if of.addr == "" {
				return nil, fmt.Errorf("trying to use ocm.getRawProofResult Rego function, but ocm address is not set")
			}

			var presentationID string

			if err := ast.As(id.Value, &presentationID); err != nil {
				return nil, fmt.Errorf("invalid presentationId: %s", err)
			}

			res, err := of.client.GetRawLoginProofResult(bctx.Context, presentationID)
			if err != nil {
				return nil, err
			}

			var val ast.Value
			val, err = ast.InterfaceToValue(res)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}
