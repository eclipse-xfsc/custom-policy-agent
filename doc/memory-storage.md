# Memory Storage Implementation

Policies (rego source code and metadata) are stored in a collection in the service memory.
Policies are being fetched on service start-up by cloning a GIT repository
containing policies. This implementation is fully compatible with the [storage
interface](../internal/service/policy/storage.go).

In order to use the Memory Storage implementation you **must** provide 
`POLICY_REPOSITORY_CLONE_URL` environment variable. Other configurations 
such as GIT authentication can be found in the [config](../internal/config/config.go) file.

> Storing policies in-memory means that every instance of the policy service has 
> its own set of policies. You cannot rely on different instances of the policy service
> to store the exact same state of a policy set.


Memory storage implementation can be found [here](../internal/storage/memory/storage.go)

Storage interface can be found [here](../internal/service/policy/storage.go)