package regofunc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
)

func TestResolveFunc(t *testing.T) {
	expected := `{"data":{"@context":"https://w3id.org/did-resolution/v1","didDocument":"document"}}`
	resolverSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, expected)
	}))
	defer resolverSrv.Close()

	DIDResolverFuncs := regofunc.NewDIDResolverFuncs(resolverSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`did.resolve("did:indy:idunion:BDrEcHc8Tb4Lb2VyQZWEDE")`),
		rego.Function1(DIDResolverFuncs.ResolveFunc()),
		rego.StrictBuiltinErrors(true),
	)
	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}
