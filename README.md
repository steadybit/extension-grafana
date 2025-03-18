# Steadybit extension-grafana

A [Steadybit](https://www.steadybit.com/) extension to integrate [Grafana](https://grafana.com/) into Steadybit.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_grafana).

## Prerequisites

You need to have a [Grafana service token](https://grafana.com/docs/grafana/latest/administration/service-accounts/#add-a-token-to-a-service-account-in-grafana). The token must have the following permissions:
- to read alert rules
- to read/write annotations

## Configuration

| Environment Variable                                          | Helm value                                | Meaning                                                                                                                    | Required | Default |
|---------------------------------------------------------------|-------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_EXTENSION_SERVICE_TOKEN`                           | `grafana.serviceToken`                    | Grafana Service Token                                                                                                      | yes      |         |
| `STEADYBIT_EXTENSION_API_BASE_URL`                            | `grafana.apiBaseUrl`                      | Grafana API Base URL (example: https://yourcompany.grafana.io)                                                             | yes      |         |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ALERTRULE` | `discovery.attributes.excludes.alertrule` | List of Alert Rule Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*" | no       |         |


The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

## Installation

### Kubernetes

Detailed information about agent and extension installation in kubernetes can also be found in
our [documentation](https://docs.steadybit.com/install-and-configure/install-agent/install-on-kubernetes).

#### Recommended (via agent helm chart)

All extensions provide a helm chart that is also integrated in the
[helm-chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-agent) of the agent.

You must provide additional values to activate this extension.

```
--set extension-grafana.enabled=true \
--set extension-grafana.grafana.serviceToken="{{SERVICE_TOKEN}}" \
--set extension-grafana.grafana.apiBaseUrl="{{API_BASE_URL}}" \
```

Additional configuration options can be found in
the [helm-chart](https://github.com/steadybit/extension-grafana/blob/main/charts/steadybit-extension-grafana/values.yaml) of the
extension.

#### Alternative (via own helm chart)

If you need more control, you can install the extension via its
dedicated [helm-chart](https://github.com/steadybit/extension-grafana/blob/main/charts/steadybit-extension-grafana).

```bash
helm repo add steadybit-extension-grafana https://steadybit.github.io/extension-grafana
helm repo update
helm upgrade steadybit-extension-grafana \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-agent \
    --set grafana.serviceToken="{{SERVICE_TOKEN}}" \
    --set grafana.apiBaseUrl="{{API_BASE_URL}}" \
    steadybit-extension-grafana/steadybit-extension-grafana
```

### Linux Package

This extension is currently not available as a Linux package.

## Extension registration

Make sure that the extension is registered with the agent. In most cases this is done automatically. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-registration) for more
information about extension registration and how to verify.

## FAQ

### The extension-grafana is unauthorized to fetch data from grafana (status code 401)

Do you provide the service account token to the extension ? Does the token still exists on Grafana ?

_warning: If you want the service account token to survive a Grafana pod deletion or restart, you need to [persist the Grafana data in a DB](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#database)._

## Version and Revision

The version and revision of the extension:
- are printed during the startup of the extension
- are added as a Docker label to the image
- are available via the `version.txt`/`revision.txt` files in the root of the image
