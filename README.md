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

### Using Docker

```sh
docker run \
  --rm \
  -p 8080 \
  --name steadybit-extension-grafana \
  --env STEADYBIT_EXTENSION_SERVICE_TOKEN="{{SERVICE_TOKEN}}" \
  --env STEADYBIT_EXTENSION_API_BASE_URL="{{API_BASE_URL}}" \
  ghcr.io/steadybit/extension-grafana:latest
```

### Using Helm in Kubernetes

```sh
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

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.

## FAQ

### The extension-grafana is unauthorized to fetch data from grafana (status code 401)

Do you provide the service account token to the extension ? Does the token still exists on Grafana ?

_warning: if you want the service account token to survive a grafana pod deletion or restart, you need to [persist the grafana data in a DB](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#database)._
