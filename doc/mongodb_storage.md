# MongoDB Storage Implementation

Policies (rego source code and metadata) are stored in a MongoDB collection `policies`,
with one collection document representing one policy. A document contains additional
policy state un-related to OPA and Rego, but necessary for implementing the TSA
requirements (e.g. policy lock/unlock).

The Mongo database is used as read-only source of truth for the current policy state when
policies need to be evaluated. Policy storage is updated externally from a separate
component. The update process is automatically triggered by updating policy source
code files in an external Git server.

In order to use MongoDB as a storage you **must** provide `MONGO_ADDR` environment 
variable. Other configurations can be found in the [config](../internal/config/config.go) file.

Mongo DB storage implementation can be found [here](../internal/storage/mongodb/storage.go)

Storage interface can be found [here](../internal/service/policy/storage.go)