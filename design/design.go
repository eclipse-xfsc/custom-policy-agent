// nolint:revive
package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = API("policy", func() {
	Title("Policy Service")
	Description("The policy service exposes HTTP API for executing policies.")
	Server("policy", func() {
		Description("Policy Server")
		Host("development", func() {
			Description("Local development server")
			URI("http://localhost:8081")
		})
	})
})

var _ = Service("policy", func() {
	Description("Policy Service provides evaluation of policies through Open Policy Agent.")

	Method("Evaluate", func() {
		Description("Evaluate executes a policy with the given 'data' as input.")
		Payload(EvaluateRequest)
		Result(EvaluateResult)
		HTTP(func() {
			GET("/policy/{repository}/{group}/{policyName}/{version}/evaluation/did.json")
			GET("/policy/{repository}/{group}/{policyName}/{version}/evaluation")
			POST("/policy/{repository}/{group}/{policyName}/{version}/evaluation")
			Header("evaluationID:x-evaluation-id", String, "EvaluationID allows overwriting the randomly generated evaluationID", func() {
				Example("did:web:example.com")
			})
			Header("ttl:x-cache-ttl", Int, "Policy result cache TTL in seconds", func() {
				Example(60)
			})
			Body("input")
			Response(StatusOK, func() {
				Body("result")
				Header("ETag")
			})
		})
	})

	Method("Validate", func() {
		Description("Validate executes a policy with the given 'data' as input and validates the output schema.")
		Payload(EvaluateRequest)
		Result(EvaluateResult)
		HTTP(func() {
			GET("/policy/{repository}/{group}/{policyName}/{version}/validation/did.json")
			GET("/policy/{repository}/{group}/{policyName}/{version}/validation")
			POST("/policy/{repository}/{group}/{policyName}/{version}/validation")
			Header("evaluationID:x-evaluation-id", String, "EvaluationID allows overwriting the randomly generated evaluationID", func() {
				Example("did:web:example.com")
			})
			Header("ttl:x-cache-ttl", Int, "Policy result cache TTL in seconds", func() {
				Example(60)
			})
			Body("input")
			Response(StatusOK, func() {
				Body("result")
				Header("ETag")
			})
		})
	})

	Method("Lock", func() {
		Description("Lock a policy so that it cannot be evaluated.")
		Payload(LockRequest)
		Result(Empty)
		HTTP(func() {
			POST("/policy/{repository}/{group}/{policyName}/{version}/lock")
			Response(StatusOK)
		})
	})

	Method("Unlock", func() {
		Description("Unlock a policy so it can be evaluated again.")
		Payload(UnlockRequest)
		Result(Empty)
		HTTP(func() {
			DELETE("/policy/{repository}/{group}/{policyName}/{version}/lock")
			Response(StatusOK)
		})
	})

	Method("ExportBundle", func() {
		Description("Export a signed policy bundle.")
		Payload(ExportBundleRequest)
		Result(ExportBundleResult)
		HTTP(func() {
			GET("/policy/{repository}/{group}/{policyName}/{version}/export")

			// bypass response body encoder code generation, so that
			// a zip bytes buffer (io.ReadCloser) can be returned to the client
			// while specific response headers can be specified
			// in the ExportBundleResult type.
			SkipResponseBodyEncodeDecode()

			Response(StatusOK, func() {
				Header("content-type")
				Header("content-length")
				Header("content-disposition")
			})
		})
	})

	Method("PolicyPublicKey", func() {
		Description("PolicyPublicKey returns the public key in JWK format which must be used to verify a signed policy bundle.")
		Payload(PolicyPublicKeyRequest)
		Result(Any)
		HTTP(func() {
			GET("/policy/{repository}/{group}/{policyName}/{version}/key")
			Response(StatusOK)
		})
	})

	Method("ImportBundle", func() {
		Description("Import a signed policy bundle.")
		Payload(func() {
			Attribute("length", Int)
		})
		Result(Any)
		HTTP(func() {
			POST("/v1/policy/import")
			Header("length:Content-Length")

			SkipRequestBodyEncodeDecode()

			Response(StatusOK)
			Response(StatusForbidden)
			Response(StatusInternalServerError)
		})
	})

	Method("ListPolicies", func() {
		Description("List policies from storage with optional filters.")
		Payload(PoliciesRequest)
		Result(PoliciesResult)
		HTTP(func() {
			GET("/v1/policies")
			Params(func() {
				Param("locked", Boolean, "Filter to return locked/unlocked policies (optional).")
				Param("policyName", String, "Filter to return policies (optional).")
				Param("rego", Boolean, "Include policy source code in results (optional).")
				Param("data", Boolean, "Include policy static data in results (optional). ")
				Param("dataConfig", Boolean, "Include static data config (optional).")
			})
			Response(StatusOK)
		})
	})

	Method("SetPolicyAutoImport", func() {
		Description("SetPolicyAutoImport enables automatic import of policy bundle on a given time interval.")
		Payload(SetPolicyAutoImportRequest)
		Result(Any)
		HTTP(func() {
			POST("/v1/policy/import/config")
			Response(StatusOK)
		})
	})

	Method("PolicyAutoImport", func() {
		Description("PolicyAutoImport returns all automatic import configurations.")
		Payload(Empty)
		Result(Any)
		HTTP(func() {
			GET("/v1/policy/import/config")
			Response(StatusOK)
		})
	})

	Method("DeletePolicyAutoImport", func() {
		Description("DeletePolicyAutoImport removes a single automatic import configuration.")
		Payload(DeletePolicyAutoImportRequest)
		Result(Any)
		HTTP(func() {
			DELETE("/v1/policy/import/config")
			Response(StatusOK)
		})
	})

	Method("SubscribeForPolicyChange", func() {
		Description("Subscribe for policy change notifications by registering webhook callbacks which the policy service will call.")
		Payload(SubscribeRequest)
		Result(Any)
		HTTP(func() {
			POST("/policy/{repository}/{group}/{policyName}/{version}/notifychange")
			Response(StatusOK)
		})
	})
})

var _ = Service("health", func() {
	Description("Health service provides health check endpoints.")

	Method("Liveness", func() {
		Payload(Empty)
		Result(HealthResponse)
		HTTP(func() {
			GET("/liveness")
			Response(StatusOK)
		})
	})

	Method("Readiness", func() {
		Payload(Empty)
		Result(HealthResponse)
		HTTP(func() {
			GET("/readiness")
			Response(StatusOK)
		})
	})
})

var _ = Service("openapi", func() {
	Description("The openapi service serves the OpenAPI(v3) definition.")
	Meta("swagger:generate", "false")
	HTTP(func() {
		Path("/swagger-ui")
	})
	Files("/openapi.json", "./gen/http/openapi3.json", func() {
		Description("JSON document containing the OpenAPI(v3) service definition")
	})
	Files("/{*filepath}", "./swagger/")
})
