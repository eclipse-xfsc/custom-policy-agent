package regofunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
)

const (
	verificationMethodPath  = "/v1/verification-methods/%s/%s/%s"
	verificationMethodsPath = "/v1/verification-methods/%s/%s"
	createVCProofPath       = "/v1/credential/proof"
	createVPProofPath       = "/v1/presentation/proof"
)

type SignerFuncs struct {
	signerAddr string
	httpClient *http.Client
}

func NewSignerFuncs(signerAddr string, httpClient *http.Client) *SignerFuncs {
	return &SignerFuncs{
		signerAddr: signerAddr,
		httpClient: httpClient,
	}
}

func (sf *SignerFuncs) VerificationMethodFunc() (*rego.Function, rego.Builtin3) {
	return &rego.Function{
			Name:    "verification_method",
			Decl:    types.NewFunction(types.Args(types.S, types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aDID, aNamespace, aKey *ast.Term) (*ast.Term, error) {
			if sf.signerAddr == "" {
				return nil, fmt.Errorf("trying to use verification_method Rego function, but signer address is not set")
			}

			var did, namespace, key string
			if err := ast.As(aDID.Value, &did); err != nil {
				return nil, fmt.Errorf("invalid did: %s", err)
			} else if err := ast.As(aNamespace.Value, &namespace); err != nil {
				return nil, fmt.Errorf("invalid key namespace: %s", err)
			} else if err := ast.As(aKey.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key name: %s", err)
			}

			if strings.TrimSpace(did) == "" {
				return nil, fmt.Errorf("empty did")
			}
			if strings.TrimSpace(namespace) == "" {
				return nil, fmt.Errorf("empty key namespace")
			}
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("empty keyname")
			}

			path := fmt.Sprintf(verificationMethodPath, namespace, key, did)
			uri, err := url.ParseRequestURI(sf.signerAddr + path)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequestWithContext(bctx.Context, "GET", uri.String(), nil)
			if err != nil {
				return nil, err
			}

			resp, err := sf.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response from signer: %s", resp.Status)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}

func (sf *SignerFuncs) VerificationMethodsFunc() (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "verification_methods",
			Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aDID, aNamespace *ast.Term) (*ast.Term, error) {
			if sf.signerAddr == "" {
				return nil, fmt.Errorf("trying to use verification_methods Rego function, but signer address is not set")
			}

			var did, namespace string
			if err := ast.As(aDID.Value, &did); err != nil {
				return nil, fmt.Errorf("invalid did: %s", err)
			} else if err := ast.As(aNamespace.Value, &namespace); err != nil {
				return nil, fmt.Errorf("invalid key namespace: %s", err)
			}

			if strings.TrimSpace(did) == "" {
				return nil, fmt.Errorf("empty did")
			}
			if strings.TrimSpace(namespace) == "" {
				return nil, fmt.Errorf("empty key namespace")
			}

			path := fmt.Sprintf(verificationMethodsPath, namespace, did)
			uri, err := url.ParseRequestURI(sf.signerAddr + path)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequestWithContext(bctx.Context, "GET", uri.String(), nil)
			if err != nil {
				return nil, err
			}

			resp, err := sf.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response from signer: %s", resp.Status)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}

// AddVCProofFunc calls the signer service to add a proof to a given
// Verifiable Credential. It accepts 3 arguments:
// 1. Namespace of cryptographic keys in the signer.
// 2. Key to be used for signing.
// 3. Verifiable Credential in JSON format.
func (sf *SignerFuncs) AddVCProofFunc() (*rego.Function, rego.Builtin3) {
	return &rego.Function{
			Name:    "add_vc_proof",
			Decl:    types.NewFunction(types.Args(types.S, types.S, types.A), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aNamespace, aKey, credential *ast.Term) (*ast.Term, error) {
			if sf.signerAddr == "" {
				return nil, fmt.Errorf("trying to use add_vc_proof Rego function, but signer address is not set")
			}

			var namespace, key string
			if err := ast.As(aNamespace.Value, &namespace); err != nil {
				return nil, fmt.Errorf("invalid key namespace: %s", err)
			} else if err := ast.As(aKey.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key name: %s", err)
			}

			if strings.TrimSpace(namespace) == "" {
				return nil, fmt.Errorf("empty key namespace")
			}
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("empty keyname")
			}

			// cred represents verifiable credential or presentation
			var cred map[string]interface{}
			if err := ast.As(credential.Value, &cred); err != nil {
				return nil, fmt.Errorf("invalid credential: %s", err)
			}

			if cred["type"] == nil {
				return nil, fmt.Errorf("credential data does not specify type: must be VerifiableCredential")
			}

			credType, ok := cred["type"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid credential type: string is expected")
			}

			if credType != "VerifiableCredential" { //nolint:gosec
				return nil, fmt.Errorf("unknown credential type: %q", credType)
			}

			// create the payload for proof request
			payload := map[string]interface{}{
				"namespace":  namespace,
				"key":        key,
				"credential": cred,
			}

			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequestWithContext(bctx.Context, "POST", sf.signerAddr+createVCProofPath, bytes.NewReader(payloadJSON))
			if err != nil {
				return nil, err
			}

			resp, err := sf.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response from signer: %s", resp.Status)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}

// AddVPProofFunc calls the signer service to add proof to
// a Verifiable Presentation. It accepts 4 arguments:
// 1. DID used in the proof verification method to find verification key by verifiers
// 2. Namespace of the cryptographic keys in the signer.
// 3. Key to be used for signing.
// 4. Verifiable Presentation in JSON format.
func (sf *SignerFuncs) AddVPProofFunc() (*rego.Function, rego.Builtin4) {
	return &rego.Function{
			Name:    "add_vp_proof",
			Decl:    types.NewFunction(types.Args(types.S, types.S, types.S, types.A), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aDID, aNamespace, aKey, presentation *ast.Term) (*ast.Term, error) {
			if sf.signerAddr == "" {
				return nil, fmt.Errorf("trying to use add_vp_proof Rego function, but signer address is not set")
			}

			var did, namespace, key string
			if err := ast.As(aDID.Value, &did); err != nil {
				return nil, fmt.Errorf("invalid did: %s", err)
			} else if err := ast.As(aNamespace.Value, &namespace); err != nil {
				return nil, fmt.Errorf("invalid key namespace: %s", err)
			} else if err := ast.As(aKey.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key name: %s", err)
			}

			if strings.TrimSpace(did) == "" {
				return nil, fmt.Errorf("empty did")
			}
			if strings.TrimSpace(namespace) == "" {
				return nil, fmt.Errorf("empty key namespace")
			}
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("empty keyname")
			}

			var pres map[string]interface{}
			if err := ast.As(presentation.Value, &pres); err != nil {
				return nil, fmt.Errorf("invalid presentation: %s", err)
			}

			if pres["type"] == nil {
				return nil, fmt.Errorf("presentation data does not specify type: must be VerifiablePresentation")
			}

			presType, ok := pres["type"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid presentation type: string is expected")
			}

			if presType != "VerifiablePresentation" {
				return nil, fmt.Errorf("unknown presentation type: %q", presType)
			}

			// create the payload for proof request
			payload := map[string]interface{}{
				"issuer":       did,
				"namespace":    namespace,
				"key":          key,
				"presentation": pres,
			}

			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequestWithContext(bctx.Context, "POST", sf.signerAddr+createVPProofPath, bytes.NewReader(payloadJSON))
			if err != nil {
				return nil, err
			}

			resp, err := sf.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response from signer: %s", resp.Status)
			}

			v, err := ast.ValueFromReader(resp.Body)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(v), nil
		}
}

func (sf *SignerFuncs) VerifyProofFunc() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "proof.verify",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, credential *ast.Term) (*ast.Term, error) {
			if sf.signerAddr == "" {
				return nil, fmt.Errorf("trying to use proof.verify Rego function, but signer address is not set")
			}

			// cred represents verifiable credential or presentation
			var cred map[string]interface{}
			if err := ast.As(credential.Value, &cred); err != nil {
				return nil, fmt.Errorf("invalid credential: %s", err)
			}

			if cred["type"] == nil {
				return nil, fmt.Errorf("credential data does not specify type: must be VerifiablePresentation or VerifiableCredential")
			}

			credType, ok := cred["type"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid credential type, string is expected")
			}

			if cred["proof"] == nil {
				return nil, fmt.Errorf("credential data does contain proof section")
			}

			var verifyProofPath string
			switch credType {
			case "VerifiableCredential":
				verifyProofPath = "/v1/credential/verify"
			case "VerifiablePresentation":
				verifyProofPath = "/v1/presentation/verify"
			default:
				return nil, fmt.Errorf("unknown credential type: %q", credType)
			}

			jsonCred, err := json.Marshal(cred)
			if err != nil {
				return nil, err
			}

			req, err := http.NewRequestWithContext(bctx.Context, "POST", sf.signerAddr+verifyProofPath, bytes.NewReader(jsonCred))
			if err != nil {
				return nil, err
			}

			resp, err := sf.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close() // nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected response from signer: %s", resp.Status)
			}

			var result struct {
				Valid bool `json:"valid"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return nil, fmt.Errorf("failed to decode response from signer: %v", err)
			}

			if !result.Valid {
				return nil, fmt.Errorf("proof is invalid")
			}

			return ast.NewTerm(ast.Boolean(true)), nil
		}
}
