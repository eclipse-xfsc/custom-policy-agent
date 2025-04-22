# Policy Bundles

A policy can be exported as a ZIP bundle file. The bundle can be imported later by
another Policy service instance run by a different organization. Bundles can also 
be imported automatically and periodically via import configurations.

### Policy Export

To export a policy bundle, a client makes a GET request to the URL of the policy
in the same way as requesting policy evaluation, but using the word `export` instead of 
the word `evaluation`. This will trigger an export for the policy bundle and 
the ZIP file will be returned to the client. Example:
```shell
curl https://mypolicyservice.com/policy/repo/example/policyName/1.0/export -o bundle.zip
```

### Policy Import

Importing a policy bundle is done similarly via POST request with `Content-Type: multipart/form-data`.
```shell
curl -X POST -H "Content-Type: multipart/form-data" -F file=@bundle.zip https://mypolicyservice.com/v1/policy/import
```

### Policy Bundle Signing Overview

The ZIP bundle is digitally signed using the Signer service and can be verified 
independently without using the Signer service. Each bundle specifies the public
key for verification inside the `metadata.json` file. When a verifier fetch the 
public key, it gets a standard JWK key (see below). Depending on key/curve type
the verifier can select an algorithm to verify the bundle.
```json
{
  "crv": "P-256",
  "kid": "key1",
  "kty": "EC",
  "x": "tpAuDE3rwQHV9qjIX0poPGMTpXsJldJtom5vHH4yiNw",
  "y": "QVwQMROxu9GgtJjUwvrcPl0-_eX7azopOAw7CpZLCtY"
}
```

Some algorithms can be used with different hash functions, which can optionally
be specified during the signing process. This could be problematic as knowing which
particular hash function is used for signing is required for the verification process,
which could be performed by an external entity. Also, the hash function is not returned 
as information when verifiers fetch the public key, as the same key could be used with 
different hash functions for some algorithms. To simplify the verification process, 
for all algorithms which can optionally specify different hash functions for signing, 
we are always using the SHA256 hash function. The algorithms which allow optional selection
of hash functions are from the ECDSA and RSA family. ED25519 signing algorithm uses its 
own hash function internally, which cannot be optionally changed/selected, so it's not
affected by such issues.

### Supported Signing Algorithms

The bundles are signed using the Signer service, which in turn uses the Hashicorp Vault
to create the signature. Signing a bundle includes sending the hash of the ZIP file
and specifying key namespace and key name. The namespace is the name of the HV transit
engine and key name specifies which key inside the engine will be used.

Supported signature algorithms are:
* ECDSA-P256, ECDSA-P384, ECDSA-P521
* RSA-2048, RSA-3072, RSA-4096
* ED25519

### Policy Export Configuration

The export configuration is a JSON file sitting next to the `policy.rego` source code
in the Git repo, which specifies export options like signing key namespace and key
name. In the future it could be used to specify additional parameters, if needed.
The file is named `export-config.json`:
```export-config.json
{
  "namespace": "transit",
  "key": "key1"
}
```

This file is **required** for performing policy export. If a policy *must not be* available
for export, then remove the `export-config.json`.

You can see an example file [here](https://gitlab.eclipse.org/eclipse/xfsc/tsa/policies/-/tree/main/example/examplePolicy/1.4)

### Batch Export/Import

Exporting/importing multiple policies with a single API call is *not* supported. 
Policy export/import works for single policy only. If you need to export multiple
policies, you would have to make multiple `export` calls (same for import).

### Automatic Policy Bundle Import

The policy service allows an administrator to specify URL from where a policy
bundle could be fetched and imported periodically. 

Three new HTTP endpoints can be used to manage this functionality.

##### Create Policy Autoimport Configuration

The following example will create/update a policy import configuration,
which specify to fetch a bundle from the given URL and import it, 
once every hour.
```
POST /v1/policy/import/config
{
  "policyURL":"https://mypolicyservice.com/policy/repo/example/policyName/1.0/export",
  "interval":"1h"
}
```

##### List Active Autoimport Configurations
```
GET /v1/policy/import/config
```

##### Delete Autoimport Configuration
```
DELETE /v1/policy/import/config
{
  "policyURL":"https://mypolicyservice.com/policy/repo/example/policyName/1.0/export",
}
```
