# Steadybit extension-grafana

TODO describe what your extension is doing here from a user perspective.

TODO optionally add your extension to the [Reliability Hub](https://hub.steadybit.com/) by creating
a [pull request](https://github.com/steadybit/reliability-hub-db) and add a link to this README.

## Configuration

| Environment Variable                                          | Helm value                                | Meaning                                                                                                                    | Required | Default |
|---------------------------------------------------------------|-------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_EXTENSION_SERVICE_TOKEN`                           | `stackstate.serviceToken`                 | Grafana Service Token                                                                                                      | yes      |         |
| `STEADYBIT_EXTENSION_API_BASE_URL`                            | `stackstate.apiBaseUrl`                   | Grafana API Base URL (example: https://yourcompany.grafana.io)                                                             | yes      |         |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ALERTRULE` | `discovery.attributes.excludes.alertrule` | List of Alert Rule Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*" | no       |         |


The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

## Installation

### Using Docker

```sh
docker run \
  --rm \
  -p 8080 \
  --name steadybit-extension-grafana \
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
    steadybit-extension-grafana/steadybit-extension-grafana
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.
