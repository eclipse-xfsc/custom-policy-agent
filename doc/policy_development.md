# Policy Development Extensions

The policy service extends the standard Rego runtime with custom
built-in functions and some custom functionalities described here.

### Pure Evaluation Results

In the Rego language there is a blank identifier variable which
can be used for assignments. If used like it's shown below, the
result from the policy evaluation will not be embedded in variable,
but will be returned as _pure_ result.

> This is custom functionality developed in the policy service
> and is not standard behaviour of the OPA Rego runtime.

Below are two examples for what it means.

If the following policy is evaluated, the returned result will be
_embedded_ in an attribute named `credential`:

```
package example.createProof

credential := add_vc_proof("transit", "key1", input)
```

Result:

```json
{
    "credential": {
        "@context": "...",
        "type": "VerifiableCredential",
        "credentialSubject": {...},
        "proof": {...}
    }
}
```

If however a blank identifier is used for the assignment, the result
of the policy evaluation won't be embedded in an attribute named
`credential` but will be returned directly:

```
package example.createProof

_ = add_vc_proof("transit", "key1", input)
```

Result:

```json
{
    "@context": "...",
    "type": "VerifiableCredential",
    "credentialSubject": {...},
    "proof": {...}
}
```

A policy developer can use the blank identifier assignment to skip the
mapping of a function call to a JSON attribute name. The result of the
function call will be returned directly as JSON.

This is useful in case you want to return a DID document or Verifiable Credential
from policy evaluation, and the document must not be mapped to an upper level attribute.

### Extension Functions

A number of Rego extension functions are developed and injected in the
policy service Rego runtime. Here is a list with brief description for
each one of them.

#### external.http.header

The function retrieves an HTTP header value taken from the incoming request
during the current policy evaluation. The header name is in Canonical format
because of the way Go `net/http` library formats headers.

For example, inside Rego the value of a header named `Authorization` can be retrieved
as follows:

```
package example.example

auth := external.http.header("Authorization")
```

#### cache.get

The function retrieves JSON data from the Cache service. It accepts
three parameters used to identify the underlying Cache key. Only the
first one named `key` is required, the other two may be empty.

Example:

```
package example.cacheGet

data := cache.get("mykey", "", "")
```

#### cache.set

The function inserts JSON data into the Cache service. It accepts
four parameters. First three are used to identify/construct the
underlying Cache key. The last one is the data to be stored.

Example:

```
package example.cacheSet

result := cache.set("mykey", "", "", input.data)
```

#### did.resolve

Resolve DID using the [Universal DID Resolver](https://github.com/decentralized-identity/universal-resolver)
and return the resolved DID document.

Example:

```
package example.didResolve

result := did.resolve("did:key:z6Mkfriq1MqLBoPWecGoDLjguo1sB9brj6wT3qZ5BxkKpuP6")
```

#### task.create

Start asynchronous task and pass the given data as task input. The function accepts two
parameters: task name and the input data.

Example:

```
package example.taskCreate

result := task.create("task-name", input.data)
```

#### tasklist.create

Start asynchronous task list and pass the given data as input. The function accepts two
parameters: task list name and the input data.

Example:

```
package example.tasklist

result := tasklist.create("task-list-name", input.data)
```

#### keys.get

Retrieve a specific public key from the signer service. The function accepts
three arguments which specify the DID, key namespace and key name. The key
is returned in JWK format wrapped in a DID verification method envelope.

Example:

```
package example.getkey

_ := keys.get("did:web:example.com", "transit", "key1")
```

Result:

```json
{
  "id": "did:web:example.com#key1",
  "type": "JsonWebKey2020",
  "controller": "did:web:example.com",
  "publicKeyJwk": {
    "kty": "OKP",
    "kid": "key1",
    "crv": "Ed25519",
    "x": "djRlRCtKdWFxcjJwMjlGTjAwa0w2ZHpHWVZURGN1eVJydDdrN1p5eEo5Yz0"
  }
}
```

#### keys.getAll

Retrieve all public keys from the signer service. The function accepts
two arguments specifying DID and key namespace. The result is JSON array of
keys in JWK format wrapped in a DID verification method envelope.

Example:

```
package example.getAllKeys

_ := keys.getAll("did:web:example.com", "transit")
```

Result:

```json
[
  {
    "id": "did:web:example.com#key1",
    "type": "JsonWebKey2020",
    "controller": "did:web:example.com",
    "publicKeyJwk": {
      "kty": "OKP",
      "kid": "key1",
      "crv": "Ed25519",
      "x": "djRlRCtKdWFxcjJwMjlGTjAwa0w2ZHpHWVZURGN1eVJydDdrN1p5eEo5Yz0"
    }
  },
  {
    "id": "did:web:example.com#key2",
    "type": "JsonWebKey2020",
    "controller": "did:web:example.com",
    "publicKeyJwk": {
      "kty": "EC",
      "kid": "key2",
      "crv": "P-256",
      "x": "8Kfl7wsUWeNOTgMR2wFWRhnU6o8jLnPuRcXQvJBu-Is",
      "y": "_yVgBlJiWsquGWJPhuxrp_gy1x5g6fhhbDP9oyGWph4"
    }
  }
]
```

#### add_vc_proof

Add a proof to Verifiable Credential.
The function accepts three arguments:

- Key namespace where the signing key must be present.
- Key name of the signing key to be used.
- A Verifiable Credential document in JSON format.
  It calls the signer service to generate a proof and returns the response,
  which is the same VC but with the generated proof section by the signer.

Example Policy:

```
package example.addProof

_ := add_vc_proof("transit", "key1", input)
```

Example VC given to policy evaluation:

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://w3id.org/security/suites/jws-2020/v1",
    "https://www.w3.org/2018/credentials/examples/v1"
  ],
  "credentialSubject": {
    "allow": true,
    "id": "example/examplePolicy/1.0"
  },
  "issuanceDate": "2022-07-12T13:59:35.246990412Z",
  "issuer": "did:web:gaiax.vereign.com:tsa:policy:policy:example:returnDID:1.0:evaluation",
  "type": "VerifiableCredential"
}
```

Example Response:

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://w3id.org/security/suites/jws-2020/v1",
    "https://www.w3.org/2018/credentials/examples/v1"
  ],
  "credentialSubject": {
    "allow": true,
    "id": "example/examplePolicy/1.0"
  },
  "issuanceDate": "2022-07-12T13:59:35.246990412Z",
  "issuer": "did:web:gaiax.vereign.com:tsa:policy:policy:example:returnDID:1.0:evaluation",
  "proof": {
    "created": "2022-07-21T09:57:37.761706653Z",
    "jws": "eyJhbGciOiJKc29uV2ViU2lnbmF0dXJlMjAyMCIsImI2NCI6ZmFsc2UsImNyaXQiOlsiYjY0Il19..MEUCIQCOuTbwqembXOv2wjPhPkjR5Minf27DhO_KbNmXdxRxKQIgK3DTaucbir5SYNi_5Xwj8mpKoXxoKzF5_ZYUJB98IBE",
    "proofPurpose": "assertionMethod",
    "type": "JsonWebSignature2020",
    "verificationMethod": "did:web:gaiax.vereign.com:tsa:policy:policy:example:returnDID:1.0:evaluation#key1"
  },
  "type": "VerifiableCredential"
}
```

#### add_vp_proof

Add a proof to Verifiable Presentation.
The function accepts four arguments:

- Issuer DID used for identifying the verification method to verify the proof.
- Key namespace where the signing key must be present.
- Key name of the signing key to be used.
- A Verifiable Presentation document in JSON format.
  It calls the signer service to generate a proof and returns the response,
  which is the same VC but with the generated proof section by the signer.

Example Policy:

```
package example.addProof

_ := add_vp_proof("did: web:example.com", "transit", "key1", input)
```

#### proof.verify

Verify a proof for Verifiable Credential or Verifiable Presentation.
The function accepts one argument which represents a VC or VP in JSON format.
It calls the signer service to validate the proof.

Example Policy:

```
package example.verifyProof

valid := proof.verify(input)
```

Example VC given to policy evaluation:

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://w3id.org/security/suites/jws-2020/v1",
    "https://www.w3.org/2018/credentials/examples/v1"
  ],
  "credentialSubject": {
    "allow": true,
    "id": "example/examplePolicy/1.0"
  },
  "issuanceDate": "2022-07-12T13:59:35.246990412Z",
  "issuer": "did:web:gaiax.vereign.com:tsa:policy:policy:example:returnDID:1.0:evaluation",
  "proof": {
    "created": "2022-07-21T09:57:37.761706653Z",
    "jws": "eyJhbGciOiJKc29uV2ViU2lnbmF0dXJlMjAyMCIsImI2NCI6ZmFsc2UsImNyaXQiOlsiYjY0Il19..MEUCIQCOuTbwqembXOv2wjPhPkjR5Minf27DhO_KbNmXdxRxKQIgK3DTaucbir5SYNi_5Xwj8mpKoXxoKzF5_ZYUJB98IBE",
    "proofPurpose": "assertionMethod",
    "type": "JsonWebSignature2020",
    "verificationMethod": "did:web:gaiax.vereign.com:tsa:policy:policy:example:returnDID:1.0:evaluation#key1"
  },
  "type": "VerifiableCredential"
}
```

Result:

```json
{
  "valid": true
}
```

#### ocm.getLoginProofInvitation

Get a Proof Invitation URL from OCM's "out-of-band" endpoint.
This function accepts two arguments. The first argument is an array of scopes used to identify
credential types in OCM. The second argument is a map between scopes and credential types
which is statically defined in a `data.json` file.

Example request body:

```json
{
  "scope": ["openid", "email"]
}
```

Example `data.json` file containing "scope-to-credential-type" map:

```json
{
  "scopes": {
    "openid": "principalMemberCredential",
    "email": "universityCert"
  }
}
```

Example policy:

```rego
package example.GetLoginProofInvitation

_ = ocm.getLoginProofInvitation(input.scope, data.scopes)
```

Result:

```json
{
  "link": "https://ocm:443/didcomm/?d_m=eyJAdHlwZSI6Imh0dHBzOi8vZGlkY29tbS5vc9tbSJ9fQ",
  "requestId": "851076fa-da78-444a-9127-e636c5102f40"
}
```

#### ocm.SendPresentationRequest

Send a Presentation Request containing attributes (claims) with corresponding `schemaId` and `credentialDefinitionId`
and receive an Invitation URL in return.

> Note: `schemaId`, `credentialDefinitionId` and `attributeName` are required fields for each attribute.

Example request body:

```json
{
  "attributes": [
    {
      "schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0",
      "credentialDefinitionId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpir",
      "attributeName": "prcLastName"
    },
    {
      "schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0",
      "credentialDefinitionId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpir",
      "attributeName": "email"
    }
  ],
  "options": {
    "type": "Aries1.0"
  }
}
```

Example policy:

```rego
package example.SendPresentationRequest

_ = ocm.sendPresentationRequest(input)
```

Result:

```json
{
  "link": "https://ocm:443/didcomm/?d_m=eyJAdHlwZSI6Imh0dHBzOi8vZGlkY29tbS5vc9tbSJ9fQ",
  "requestId": "851076fa-da78-444a-9127-e636c5102f40"
}
```

#### ocm.GetLoginProofResult

Get a Proof Invitation result from OCM containing a flattened list of claims.
This function accepts one argument which is the `resuestId` from the result of one of the
`ocm.getLoginProofInvitation` or `ocm.sendPresentationRequest` functions.

Example request body:

```json
{
  "requestId": "87839b89-da07-4d30-bb57-392e49999fc3"
}
```

Example policy:

```rego
package example.GetLoginProofResult

_ = ocm.getLoginProofResult(input.requestId)
```

Result:

```json
{
  "name": "John Doe",
  "given_name": "John",
  "family_name": "Doe",
  "email": "example@example.com",
  "email_verified": true,
  "preferred_username": "john",
  "gender": "NA"
}
```

#### ocm.GetRawProofResult

Get a Proof Invitation result containing the raw response from the OCM.
This function accepts one argument which is the `resuestId` from the result of one of the
`ocm.getLoginProofInvitation` or `ocm.sendPresentationRequest` functions.

Example request body:

```json
{
  "requestId": "87839b89-da07-4d30-bb57-392e49999fc3"
}
```

Example policy:

```rego
package example.GetRawProofResult

_ = ocm.getRawProofResult(input.requestId)
```

Result before Proof request is accepted:

```json
{
  "data": {
    "presentations": [
      {
        "credDefId": "",
        "credentialSubject": {},
        "revRegId": "",
        "schemaId": "",
        "timestamp": ""
      }
    ],
    "state": "request-sent"
  },
  "message": "Proof presentation fetch successfully",
  "statusCode": 200
}
```

Result after Proof request is accepted:

```json
{
  "data": {
    "presentations": [
      {
        "credDefId": "7KuDTpQh3GJ7Gp6kErpWvM:3:CL:40329:principalTestCredDefExpire",
        "credentialSubject": {
          "email": "23957edb-991d-4b5f-bf76-153103ba45b7",
          "prcLastName": "NA"
        },
        "revRegId": null,
        "schemaId": "7KuDTpQh3GJ7Gp6kErpWvM:2:principalTestSchema:1.0"
      }
    ],
    "state": "done"
  },
  "message": "Proof presentation fetch successfully",
  "statusCode": 200
}
```

#### storage.set

Set data to the storage if the key exist the data will be updated.
The result is nil if there is no error.
This function accepts two arguments. The first one is the `key`.
The second one is `data` you want to write. In json format.

Example request body:

```json
{
  "key": "example",
  "data": { "some": "data" }
}
```

Example policy:

```rego
package example.storageSet

_ = storage.set(input.key, input.data)
```

Result when set:

```
null
```

#### storage.get

Get data from the storage the result is object bound to given key if there is no error.
This function accepts one argument `key`.

Example request body:

```json
{
  "key": "example"
}
```

Example policy:

```rego
package example.storageGet

_ = storage.get(input.key)
```

Result when set:

```
{ "some": "updated_data" }
```

#### storage.delete

Delete data from the storage the result is null if there is no error.
This function accepts one argument `key`.

Example request body:

```json
{
  "key": "example"
}
```

Example policy:

```rego
package example.storageDelete

_ = storage.delete(input.key)
```

Result when set:

```
null
```
