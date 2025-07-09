# application-rbac-validator

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart for the application-rbac-validator webhook.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| clusterTokens | object | `{"example-destination-server-name-token":"<example_token>"}` | A mapping of destination server names to cluster access tokens used by the webhook. |
| clusterTokens.example-destination-server-name-token | string | `"<example_token>"` | Example entry for a destination server token. |
| controllerManager | object | `{"manager":{"args":["--metrics-bind-address=:8443","--leader-elect","--health-probe-bind-address=:8081","--metrics-cert-path=/tmp/k8s-metrics-server/metrics-certs","--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs"],"containerSecurityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}},"image":{"repository":"controller","tag":""},"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}},"replicas":1,"serviceAccount":{"annotations":{}}}` | Configuration for the controller manager. |
| controllerManager.manager | object | `{"args":["--metrics-bind-address=:8443","--leader-elect","--health-probe-bind-address=:8081","--metrics-cert-path=/tmp/k8s-metrics-server/metrics-certs","--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs"],"containerSecurityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}},"image":{"repository":"controller","tag":""},"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}}` | Manager-specific settings within the controller. |
| controllerManager.manager.args | list | `["--metrics-bind-address=:8443","--leader-elect","--health-probe-bind-address=:8081","--metrics-cert-path=/tmp/k8s-metrics-server/metrics-certs","--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs"]` | Command-line arguments passed to the manager container. |
| controllerManager.manager.containerSecurityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Container security context for the manager container. |
| controllerManager.manager.containerSecurityContext.allowPrivilegeEscalation | bool | `false` | Specifies whether a process can gain more privileges than its parent. |
| controllerManager.manager.containerSecurityContext.capabilities | object | `{"drop":["ALL"]}` | Linux capabilities to drop from the container. |
| controllerManager.manager.image | object | `{"repository":"controller","tag":""}` | Image configuration for the manager container. |
| controllerManager.manager.image.repository | string | `"controller"` | The repository of the manager container image. |
| controllerManager.manager.image.tag | string | `""` | The tag of the manager container image. |
| controllerManager.manager.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Resource requests and limits for the manager container. |
| controllerManager.manager.resources.limits | object | `{"cpu":"500m","memory":"128Mi"}` | CPU and memory limits for the manager container. |
| controllerManager.manager.resources.requests | object | `{"cpu":"10m","memory":"64Mi"}` | CPU and memory requests for the manager container. |
| controllerManager.replicas | int | `1` | The number of replicas for the controller manager deployment. |
| controllerManager.serviceAccount | object | `{"annotations":{}}` | Service account configuration for the controller manager. |
| controllerManager.serviceAccount.annotations | object | `{}` | Annotations to add to the service account. |
| kubernetesClusterDomain | string | `"cluster.local"` | The Kubernetes cluster domain. Defaults to 'cluster.local'. |
| metricsService | object | `{"ports":[{"name":"https","port":8443,"protocol":"TCP","targetPort":8443}],"type":"ClusterIP"}` | Configuration for the metrics service. |
| metricsService.ports | list | `[{"name":"https","port":8443,"protocol":"TCP","targetPort":8443}]` | List of ports to expose for the metrics service. |
| metricsService.ports[0].name | string | `"https"` | The name of the port. |
| metricsService.ports[0].port | int | `8443` | The port number exposed by the service. |
| metricsService.ports[0].protocol | string | `"TCP"` | The protocol used by the port (e.g., TCP, UDP, SCTP). |
| metricsService.ports[0].targetPort | int | `8443` | The target port on the pods to which the service sends traffic. |
| metricsService.type | string | `"ClusterIP"` | The type of service (e.g., ClusterIP, NodePort, LoadBalancer). |
| webhook | object | `{"namespaceSelector":{}}` | Configuration for the webhook. |
| webhook.namespaceSelector | object | `{}` | Namespace selector configuration for the validating webhook. |
| webhookService | object | `{"ports":[{"port":443,"protocol":"TCP","targetPort":9443}],"type":"ClusterIP"}` | Configuration for the webhook service. |
| webhookService.ports | list | `[{"port":443,"protocol":"TCP","targetPort":9443}]` | List of ports to expose for the webhook service. |
| webhookService.ports[0].port | int | `443` | The port number exposed by the service. |
| webhookService.ports[0].protocol | string | `"TCP"` | The protocol used by the port (e.g., TCP, UDP, SCTP). |
| webhookService.ports[0].targetPort | int | `9443` | The target port on the pods to which the service sends traffic. |
| webhookService.type | string | `"ClusterIP"` | The type of service (e.g., ClusterIP, NodePort, LoadBalancer). |

