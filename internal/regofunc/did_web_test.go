package regofunc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"
)

func TestDIDToURLFunc(t *testing.T) {
	tests := []struct {
		// test input
		name      string
		regoQuery string
		// expected result
		res     string
		errText string
	}{
		{
			name:      "DID is empty",
			regoQuery: `did_to_url("")`,
			errText:   "DID cannot be empty",
		},
		{
			name:      "invalid DID",
			regoQuery: `did_to_url("invalid-did")`,
			errText:   "invalid DID, host is not found",
		},
		{
			name:      "invalid DID Method",
			regoQuery: `did_to_url("did:sov:123456qwerty")`,
			errText:   "invalid DID, method is unknown",
		},
		{
			name:      "transformation success with DID containing domain only",
			regoQuery: `did_to_url("did:web:w3c-ccg.github.io")`,
			res:       "\"https://w3c-ccg.github.io/.well-known/did.json\"",
		},
		{
			name:      "transformation success with DID containing domain and path",
			regoQuery: `did_to_url("did:web:w3c-ccg.github.io:user:alice")`,
			res:       "\"https://w3c-ccg.github.io/user/alice/did.json\"",
		},
		{
			name:      "transformation success with DID containing network port",
			regoQuery: `did_to_url("did:web:example.com%3A3000:user:alice")`,
			res:       "\"https://example.com:3000/user/alice/did.json\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			DIDTransformerFuncs := regofunc.NewDIDWebFuncs()

			r := rego.New(
				rego.Query(test.regoQuery),
				rego.Function1(DIDTransformerFuncs.DIDToURLFunc()),
				rego.StrictBuiltinErrors(true),
			)
			resultSet, err := r.Eval(context.Background())

			if err == nil {
				resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
				assert.NoError(t, err)
				assert.Equal(t, test.res, string(resultBytes))
			} else {
				assert.ErrorContains(t, err, test.errText)
			}
		})
	}
}

func TestURLToDIDFunc(t *testing.T) {
	tests := []struct {
		// test input
		name      string
		regoQuery string
		// expected result
		res     string
		errText string
	}{
		{
			name:      "empty URL",
			regoQuery: `url_to_did("")`,
			errText:   "URL cannot be empty",
		},
		{
			name:      "URL containing special characters",
			regoQuery: `url_to_did("example.com\nH1234")`,
			errText:   "cannot parse URL",
		},
		{
			name:      "URL does not contain secure protocol (https)",
			regoQuery: `url_to_did("example.com")`,
			errText:   "invalid URL for did:web method",
		},
		{
			name:      "URL does not contain valid domain",
			regoQuery: `url_to_did("https://")`,
			errText:   "invalid URL for did:web method",
		},
		{
			name:      "transformation success with URL containing domain only",
			regoQuery: `url_to_did("https://w3c-ccg.github.io/.well-known/did.json")`,
			res:       "\"did:web:w3c-ccg.github.io\"",
		},
		{
			name:      "transformation success with URL containing domain with path",
			regoQuery: `url_to_did("https://w3c-ccg.github.io/user/alice/did.json")`,
			res:       "\"did:web:w3c-ccg.github.io:user:alice\"",
		},
		{
			name:      "transformation success with URL containing network port",
			regoQuery: `url_to_did("https://example.com:3000/user/alice/did.json")`,
			res:       "\"did:web:example.com%3A3000:user:alice\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			DIDTransformerFuncs := regofunc.NewDIDWebFuncs()

			r := rego.New(
				rego.Query(test.regoQuery),
				rego.Function1(DIDTransformerFuncs.URLToDIDFunc()),
				rego.StrictBuiltinErrors(true),
			)
			resultSet, err := r.Eval(context.Background())

			if err == nil {
				resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
				assert.NoError(t, err)
				assert.Equal(t, test.res, string(resultBytes))
			} else {
				assert.ErrorContains(t, err, test.errText)
			}
		})
	}
}
