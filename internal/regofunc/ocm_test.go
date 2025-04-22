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

func TestGetLoginProofInvitationSuccess(t *testing.T) {
	expected := `{"link":"https://ocm:443/ocm/didcomm/?d_m=eyJAdHlwZSI","requestId":"2cf01406-b15f-4960-a6a7-7bc62cd37a3c"}`
	ocmResponse := `{
		"statusCode": 201,
		"message": "Presentation request send successfully",
		"data": {
			"proofRecordId": "2cf01406-b15f-4960-a6a7-7bc62cd37a3c",			
			"presentationMessage": "https://ocm:443/ocm/didcomm/?d_m=eyJAdHlwZSI"
		}
	}`

	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, ocmResponse)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`ocm.getLoginProofInvitation(["openid", "profile"], {"openid": "credType1", "profile": "credType2"})`),
		rego.Function2(ocmFuncs.GetLoginProofInvitation()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))

	// "scope to credential type" map with duplicate and empty credential types
	r = rego.New(
		rego.Query(`ocm.getLoginProofInvitation(["openid", "profile", "email"], {"openid": "credType1", "profile": "credType1", "email": ""})`),
		rego.Function2(ocmFuncs.GetLoginProofInvitation()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err = r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err = json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}

func TestGetLoginProofInvitationErr(t *testing.T) {
	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"key":"value"}`)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)

	// invalid scopes array
	r := rego.New(
		rego.Query(`ocm.getLoginProofInvitation("openid", {"openid": "credType1", "profile": "credType2"})`),
		rego.Function2(ocmFuncs.GetLoginProofInvitation()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.Error(t, err)
	assert.Empty(t, resultSet)
	assert.Contains(t, err.Error(), "invalid scopes array")

	// invalid "scope to credential type" map
	r = rego.New(
		rego.Query(`ocm.getLoginProofInvitation(["openid", "profile"], "map")`),
		rego.Function2(ocmFuncs.GetLoginProofInvitation()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err = r.Eval(context.Background())
	assert.Error(t, err)
	assert.Empty(t, resultSet)
	assert.Contains(t, err.Error(), "invalid scope to credential type map")

	// empty types in "scope to credential type" map
	r = rego.New(
		rego.Query(`ocm.getLoginProofInvitation(["openid", "profile"], {"openid": "", "profile": ""})`),
		rego.Function2(ocmFuncs.GetLoginProofInvitation()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err = r.Eval(context.Background())
	assert.Error(t, err)
	assert.Empty(t, resultSet)
	assert.Contains(t, err.Error(), "no credential types found in the scope to type map")

}

func TestSendPresentationRequestSuccess(t *testing.T) {
	expected := `{"link":"https://ocm:443/ocm/didcomm/?d_m=eyJAdHlwZSI","requestId":"2cf01406-b15f-4960-a6a7-7bc62cd37a3c"}`
	ocmResponse := `{
		"statusCode": 201,
		"message": "Presentation request send successfully",
		"data": {
			"proofRecordId": "2cf01406-b15f-4960-a6a7-7bc62cd37a3c",
			"presentationMessage": "https://ocm:443/ocm/didcomm/?d_m=eyJAdHlwZSI"
		}
	}`

	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, ocmResponse)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`ocm.sendPresentationRequest({
			"attributes": [
				{
					"schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0",
					"credentialDefinitionId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpir",
					"attributeName": "prcLastName",
					"value": "",
					"condition": ""
				},
				{
					"schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0",
					"credentialDefinitionId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpir",
					"attributeName": "email",
					"value": "",
					"condition": ""
				}
			],
			"options": {
				"type": "Aries1.0"
			}
		})`),
		rego.Function1(ocmFuncs.SendPresentationRequest()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}

func TestSendPresentationRequestErr(t *testing.T) {
	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`ocm.sendPresentationRequest({
			"attributes": [
				{
					"schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0",
					"credentialDefinitionId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpir",
					"attributeName": "prcLastName",
					"value": "",
					"condition": ""
				},
				{
					"schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0",
					"credentialDefinitionId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpir",
					"attributeName": "email",
					"value": "",
					"condition": ""
				}
			],
			"options": {
				"type": "Aries1.0"
			}
		})`),
		rego.Function1(ocmFuncs.SendPresentationRequest()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response code: 500 Internal Server Error")
	assert.Empty(t, resultSet)
}

func TestGetLoginProofResult(t *testing.T) {
	expected := `{"family_name":"Doe","name":"John"}`
	ocmResponse := `{
		"statusCode": 200,
		"data": {
			"state": "done",
			"presentations": [
				{
					"credentialSubject": {
						"family_name":"Doe"
					}
				},
				{
					"credentialSubject": {
						"name":"John"
					}
				}
			]
		}
	}`

	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, ocmResponse)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`ocm.getLoginProofResult("2cf01406-b15f-4960-a6a7-7bc62cd37a3c")`),
		rego.Function1(ocmFuncs.GetLoginProofResult()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}

func TestGetRawLoginProofResultSuccess(t *testing.T) {
	expected := `{"data":{"presentations":[{"credDefId":"7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpire","credentialSubject":{"email":"23957edb-991d-4b5f-bf76-153103ba45b7","prcLastName":"NA"},"revRegId":null,"schemaId":"7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0","timestamp":null}],"state":"done"},"message":"Proof presentation fetch successfully","statusCode":200}`

	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, expected)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)

	r := rego.New(
		rego.Query(`ocm.getRawProofResult("2cf01406-b15f-4960-a6a7-7bc62cd37a3c")`),
		rego.Function1(ocmFuncs.GetRawProofResult()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}

func TestGetRawLoginProofResultErr(t *testing.T) {
	ocmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ocmSrv.Close()

	ocmFuncs := regofunc.NewOcmFuncs(ocmSrv.URL, http.DefaultClient)
	r := rego.New(
		rego.Query(`ocm.getRawProofResult("2cf01406-b15f-4960-a6a7-7bc62cd37a3c")`),
		rego.Function1(ocmFuncs.GetRawProofResult()),
		rego.StrictBuiltinErrors(true),
	)

	resultSet, err := r.Eval(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response code: 500 Internal Server Error")
	assert.Empty(t, resultSet)
}
