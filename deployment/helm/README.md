# Policy Service

## Prerequisites

- Kubernetes 1.19+
- Helm 3+

## Install Helm Chart

```console
helm install [RELEASE_NAME] -f values.yaml .
```

_See [configuration](#configuration) below._

_See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation._

## Dependencies

By default, dependencies are not included in the application/service's Helm chart. Please install dependencies  separately using their respective vendor Helm charts. The dependencies that have to be installed manually are:

- [MongoDB](https://github.com/mongodb/helm-charts/tree/main/charts/community-operator)
To disable dependencies during installation, see [multiple releases](#multiple-releases) below.

There is an option to use in-memory storage instead of mongodb. See [Storage](../../README.md#policy-storage)

_See [helm dependency](https://helm.sh/docs/helm/helm_dependency/) for command documentation._

## Uninstall Helm Chart

```console
helm uninstall [RELEASE_NAME]
```

This removes all the Kubernetes components associated with the chart and deletes the release.

_See [helm uninstall](https://helm.sh/docs/helm/helm_uninstall/) for command documentation._

## Upgrading Chart

```console
# Upgrade chart to development environment values
helm upgrade -f values-dev.yaml [RELEASE_NAME] .
```

## Configuration

See [Customizing the Chart Before Installing](https://helm.sh/docs/intro/using_helm/#customizing-the-chart-before-installing). To see all configurable options with detailed comments:

```console
helm show values .
```

### Secrets

The following Helm template snippet demonstrates how to use Kubernetes secrets in your Helm charts:

```yaml
{{- range $key, $value := .Values.secretEnv }}
- name: "{{ $key }}"
  valueFrom:
    secretRef:
      name: "{{ $value.name }}"
{{- end }}
```
This template iterates over a list of secret environment variables specified in the Helm chart values file (values.yaml). For each secret environment variable defined, it retrieves the corresponding value from a Kubernetes Secret and injects it into the container as an environment variable.

#### How to use

To utilize this template in your Helm charts, follow these steps:

    Define Secret Environment Variables: In your Helm chart values.yaml file, define a list of secret environment variables along with the names of the corresponding Kubernetes Secret objects. For example:

```yaml
secretEnv:
  DB_PASSWORD: 
    name: my-db-secret
  API_KEY:
    name: api-key-secret
```
In this example, DB_PASSWORD and API_KEY are the environment variable names, and my-db-secret and api-key-secret are the names of the Kubernetes Secret objects containing the respective values.

Update Helm Template: Update your Helm chart template file (e.g., deployment.yaml) to include the provided template snippet. This will dynamically fetch the values of the secret environment variables from the specified Kubernetes Secrets and inject them into your container.

Deploy Helm Chart: Deploy your Helm chart to your Kubernetes cluster using the helm install command.

Edit Helm Chart Values: Open the values.yaml file in your Helm chart and configure the following values:

    useConfigMap: Set to true to use a ConfigMap, or false otherwise.
    configMapName: Name of the ConfigMap containing environment variables.
    useSecretRef: Set to true to use a Secret, or false otherwise.
    secretRefName: Name of the Secret containing sensitive environment variables.
    
### Istio intergration

Optional Istio integration is done the following way in the `values.yaml` file:

```yaml
istio:
  injection:
    pod: true
```

You may also `helm show values` on this chart's [dependencies](#dependencies) for additional options

## Values

The chart ships with two storage variants:

- `values.yaml`: MongoDB storage, resource injection disabled by default.
- `values-policy-repo.yaml`: Git policy repository storage, XFSC resource injection enabled.
- `values-dev.yaml`: local/development MongoDB variant.
- `values-train.yaml`: policy repository variant for the train environment.

### Resource injection

```yaml
podAnnotations:
  inject.xfsc.io/enabled: "false"
  inject.xfsc.io/needs: "nats,task,cache,did-resolver,signer"
```

When `inject.xfsc.io/enabled` is `true`, the chart does not render `NATS_URL`, `TASK_ADDR`, `CACHE_ADDR`, `DID_RESOLVER`, or `SIGNER_SIGN_URL`; those connection values are expected to be injected by the XFSC resource provisioner. When it is `false`, the values under `policy.nats.url` and `addresses.*` are rendered directly into the container environment.

`NATS_SUBJECT` and `OCM_ADDR` remain regular application configuration in both modes.

### Storage selection

MongoDB mode:

```yaml
policy:
  repository:
    enabled: false

mongo:
  addr: mongodb://mongodb:27017/policy
  dbname: policy
  collection: policies
  authMechanism: SCRAM-SHA-1
```

Policy repository mode:

```yaml
policy:
  repository:
    enabled: true
    cloneUrl: https://example.org/policies.git
    branch: main
    folder: Policies
    lockOnValidationFailure: false
```

The modes are mutually exclusive in the Deployment template. `MONGO_*` variables are emitted only in MongoDB mode and `POLICY_REPOSITORY_*` variables only in repository mode.

### Main configuration keys

| Key | Default | Environment variable |
| --- | --- | --- |
| `policy.externalAddr` | `http://policy:8080` | `EXTERNAL_HTTP_ADDR` |
| `policy.nats.url` | `nats://nats:4222` | `NATS_ADDR` when injection is disabled |
| `policy.nats.subject` | `policy_notifier` | `NATS_SUBJECT` |
| `mongo.addr` | `mongodb://mongodb:27017/policy` | `MONGO_ADDR` |
| `mongo.dbname` | `policy` | `MONGO_DBNAME` |
| `mongo.collection` | `policies` | `MONGO_COLLECTION` |
| `mongo.authMechanism` | `SCRAM-SHA-1` | `MONGO_AUTH_MECHANISM` |
| `policy.repository.cloneUrl` | empty | `POLICY_REPOSITORY_CLONE_URL` |
| `policy.repository.folder` | empty | `POLICY_REPOSITORY_FOLDER` |
| `addresses.cache` | `http://cache:8080` | `CACHE_ADDR` when injection is disabled |
| `addresses.task` | `http://task:8080` | `TASK_ADDR` when injection is disabled |
| `addresses.didResolver` | `http://did-resolver:8080` | `DID_RESOLVER_ADDR` when injection is disabled |
| `addresses.signer` | `http://signer:8080` | `SIGNER_ADDR` when injection is disabled |
| `addresses.ocm` | `http://ocm:8080` | `OCM_ADDR` |
| `metrics.addr` | `:2112` | `METRICS_ADDR` |
| `refresher.pollInterval` | `10s` | `REFRESHER_POLL_INTERVAL` |
| `autoImport.pollInterval` | `10s` | `AUTO_IMPORT_POLL_INTERVAL` |
| `auth.enabled` | `false` | `AUTH_ENABLED` |
| `auth.jwkUrl` | empty | `AUTH_JWK_URL` |
| `auth.refreshInterval` | `1h` | `AUTH_REFRESH_INTERVAL` |
| `ipFilter.enabled` | `false` | `IP_FILTER_ENABLE` |
| `ipFilter.allowedIPs` | `[]` | `IP_FILTER_ALLOWED_IPS` |
