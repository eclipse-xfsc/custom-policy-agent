# policy

![Version: 1.4.3](https://img.shields.io/badge/Version-1.4.3-informational?style=flat-square) ![AppVersion: v1.4.3](https://img.shields.io/badge/AppVersion-v1.4.3-informational?style=flat-square)

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

![Version: 1.1.2](https://img.shields.io/badge/Version-1.1.2-informational?style=flat-square) ![AppVersion: v1.1.2](https://img.shields.io/badge/AppVersion-v1.1.2-informational?style=flat-square)


## Values


| Key                                                              | Type    | Default | Description                                                                                                                                      |
|------------------------------------------------------------------|---------|---------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| addresses.cache                                                  | string  | `"http://cache:8080"` | The address for caching service                                                                                                                  |
| addresses.didResolver                                            | string  | `"http://didresolver:8080"` | The address for DID resolver                                                                                                                     |
| addresses.ocm                                                    | string  | `"https://gaiax.vereign.com/ocm"` | The address for OCM                                                                                                                              |
| addresses.signer                                                 | string  | `"http://signer:8080"` | The address for signer service                                                                                                                   |
| addresses.task                                                   | string  | `"http://task:8080"` | The address for task service                                                                                                                     |
| autoscaling.enabled                                              | bool    | `false` | Enable autoscaling                                                                                                                               |
| autoscaling.maxReplicas                                          | int     | `3` | Maximum replicas for autoscaling                                                                                                                 |
| autoscaling.minReplicas                                          | int     | `2` | Minimum replicas for autoscaling                                                                                                                 |
| autoscaling.targetCPUUtilizationPercentage                       | int     | `70` | Target CPU utilization percentage for autoscaling trigger                                                                                        |
| autoscaling.targetMemoryUtilizationPercentage                    | int     | `70` | Target memory utilization percentage for autoscaling trigger                                                                                     |
| configMapName                                                    | string  | `"my-configmap"` | The name of the ConfigMap containing environment variables                                                                                       |
| image.name                                                       | string  | `"tsa/policy"` | The name of the Docker image                                                                                                                     |
| image.pullPolicy                                                 | string  | `"IfNotPresent"` | The pull policy for the Docker image                                                                                                             |
| image.pullSecrets                                                | string  | `"deployment-key-light"` | The secret used for pulling the Docker image when an internal image is used                                                                      |
| image.repository                                                 | string  | `"node-654e3bca7fbeeed18f81d7c7.ps-xaas.io"` | The repository for the Docker image                                                                                                              |
| image.sha                                                        | string  | `""` | The SHA of the Docker image, usually generated by the CI. Uses image.tag if empty                                                                |
| image.tag                                                        | string  | `"latest"` | The tag of the Docker image. Uses .Chart.AppVersion if empty                                                                                     |
| ingress.annotations."kubernetes.io/ingress.class"                | string  | `"nginx"` | Annotations for the Ingress                                                                                                                      |
| ingress.annotations."nginx.ingress.kubernetes.io/rewrite-target" | string  | `"/$2"` | Annotations for the Nginx Ingress Controller                                                                                                     |
| ingress.enabled                                                  | bool    | `true` | Enable Ingress                                                                                                                                   |
| ingress.frontendDomain                                           | string  | `"tsa.xfsc.dev"` | The frontend domain for the Ingress                                                                                                              |
| ingress.frontendTlsSecretName                                    | string  | `"cert-manager-tls"` | The name of the TLS secret for the Ingress                                                                                                       |
| ingress.tlsEnabled                                               | bool    | `true` | Enable TLS for the Ingress                                                                                                                       |
| istio.injection.pod                                              | bool    | `true` | Enable Istio sidecar injection for pods                                                                                                          |
| log.encoding                                                     | string  | `"json"` | The encoding format for logging                                                                                                                  |
| log.level                                                        | string  | `"debug"` | The log level for logging                                                                                                                        |
| metrics.enabled                                                  | bool    | `true` | Enable Prometheus metrics                                                                                                                        |
| metrics.port                                                     | int     | `2112` | The port for Prometheus metrics                                                                                                                  |
| mongo.addr                                                       | string  | `"mongodb://mongodb-mongodb-replicaset.infra:27017/policy?replicaSet=rs0&authSource=admin"` | The address for MongoDB                                                                                                                          |
| mongo.collection                                                 | string  | `"policies"` | The MongoDB collection name                                                                                                                      |
| mongo.dbname                                                     | string  | `"policy"` | The name of the MongoDB database                                                                                                                 |
| mongo.pass                                                       | string  | `""` | The password for MongoDB                                                                                                                         |
| mongo.user                                                       | string  | `""` | The username for MongoDB                                                                                                                         |
| memoryStorage.policiesRepo                                       | string  | `""` | Required, if MongoDB is not used. The url of git repository where policies for in memory storage are stored. Used as a replacement for Mongodb   |
| memoryStorage.policiesRepoVar                                    | string  | `""` | Environment variable key for memoryStorage.policiesRepo                                                                                          |
| memoryStorage.policiesFolder                                     | string  | `""` | The folder within memoryStorage.policiesRepo where policies are read from. If ommited, memoryStorage.policiesRepo is expected to store policies. |
| memoryStorage.policiesFolderVar                                  | string  | `""` | Environment variable key for memoryStorage.policiesFolder                                                                                        |
| name                                                             | string  | `"policy"` | The name of the application                                                                                                                      |
| nameOverride                                                     | string  | `""` | Overrides the application name                                                                                                                   |
| podAnnotations                                                   | object  | `{}` | Annotations for the pods                                                                                                                         |
| policy.http.host                                                 | string  | `""` | The host for the policy HTTP service                                                                                                             |
| policy.http.port                                                 | int     | `8080` | The port for the policy HTTP service                                                                                                             |
| policy.http.timeout.idle                                         | string  | `"120s"` | The idle timeout for the policy HTTP service                                                                                                     |
| policy.http.timeout.read                                         | string  | `"10s"` | The read timeout for the policy HTTP service                                                                                                     |
| policy.http.timeout.write                                        | string  | `"10s"` | The write timeout for the policy HTTP service                                                                                                    |
| policy.nats.subject                                              | string  | `"external"` | The subject for NATS messaging                                                                                                                   |
| policy.nats.url                                                  | string  | `"nats"` | The URL for NATS                                                                                                                                 |
| replicaCount                                                     | int     | `2` | The default number of instances to start                                                                                                         |
| resources.limits.cpu                                             | string  | `"300m"` | The CPU limit for the resources                                                                                                                  |
| resources.limits.memory                                          | string  | `"256Mi"` | The memory limit for the resources                                                                                                               |
| resources.requests.cpu                                           | string  | `"50m"` | The CPU request for the resources                                                                                                                |
| resources.requests.memory                                        | string  | `"128Mi"` | The memory request for the resources                                                                                                             |
| secretEnv                                                        | string  | `nil` | Environment variables sourced from Secret                                                                                                        |
| secretRefName                                                    | string  | `"my-secret"` | The name of the Secret containing sensitive data                                                                                                 |
| security.runAsGid                                                | int     | `0` | The group used by the apps for security                                                                                                          |
| security.runAsNonRoot                                            | bool    | `false` | Indicates if apps run as non-root by default                                                                                                     |
| security.runAsUid                                                | int     | `0` | The user used by the apps for security                                                                                                           |
| service.port                                                     | int     | `8080` | The port for the service                                                                                                                         |
| useConfigMap                                                     | bool    | `false` | Indicates whether to use a ConfigMap for environment variables                                                                                   |
| useSecretRef                                                     | bool    | `false` | Indicates whether to use a Secret for sensitive data                                                                                             |
