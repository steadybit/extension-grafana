# Changelog

## Unreleased

- fix: bound the annotation search to the annotation's own time window. Searching `/api/annotations`
  by tags without `from`/`to` makes Grafana scan the whole annotation history, which grows with every
  experiment and step until the search no longer answers within the request timeout - leaving every
  annotation without its end time.

- fix: the experiment event listeners no longer call the Grafana API on the request goroutine.
  Waiting for the annotation find/patch round-trips made the extension exceed the agent's request
  timeout, which was answered with `503 Timeout` and reported as a failed trigger event listener.
  Annotations are now queued and processed by a single worker, which keeps the order of the events.
- fix: bound requests to the Grafana API with a timeout (`STEADYBIT_EXTENSION_API_TIMEOUT`, default
  `5s`). Resty applies no timeout by default.
- fix: drain queued annotations on shutdown so a rolling restart does not silently discard them.
- fix: reject an `experiment-step-completed` event without step execution data instead of panicking.

## v1.1.8

- chore(deps): bump github.com/stretchr/testify from 1.11.1 to 1.12.0
- chore(deps): bump steadybit kits and drop Go patch pin (#101)
- chore(deps): bump steadybit kits and drop Go patch pin (#102)

## v1.1.7

- chore(deps): bump github.com/jarcoal/httpmock from 1.4.1 to 1.4.2
- feat: support filtering targets out of discovery
- fix(alert rule discovery): deduplicate targets and exclude recording rules (#99)
- fix: emit the alert rule state metric immediately on Start (#100)

## v1.1.7

- fix(alert rule discovery): deduplicate targets sharing the same id — the same rule name can appear multiple times within one Prometheus rule group (e.g. kube-prometheus-stack), which created duplicate targets on the platform
- fix(alert rule discovery): exclude recording rules from discovery — they have no alert state and cannot be checked

## v1.1.6

- chore(deps): update dependencies

## v1.1.5

- Add a "Fail early" option to the alert rule check. When enabled (the default, matching the previous behavior), the check fails as soon as a deviating state is observed. When disabled, the check keeps collecting events for the whole duration and only fails at the end of the step.
- chore(deps): bump github.com/steadybit/event-kit/go/event_kit_api
- chore(deps): bump go to 1.26.5 (#94)
- chore(deps): bump go-openapi/swag/loading to fix go mod tidy (#96)
- chore: add Claude Code workflows (#92)
- chore: silence SonarQube finding on secrets: inherit in Claude workflows
- ci: skip build on .trivyignore.yml-only changes [skip ci]
- feat(alert rule check): add fail early option (#93)
- refactor: register extension index via exthttp.RegisterRevisionedHandler (#95)

## v1.1.4

- chore(deps): bump alpine from 3.23 to 3.24
- chore(deps): bump golang.org/x/net to v0.55.0 (CVE-2026-39821) (#88)
- chore(deps): update dependencies
- fix: write ended_time tag on patch and keep timestamp tags untruncated

## v1.1.3

- chore: update to go 1.26.4
- feat: add weekly auto patch-release workflow

## v1.1.2

- Support discovery group attribute via `STEADYBIT_EXTENSION_DISCOVERY_GROUP` env var (or `discovery.group` Helm value) — when set, the extension adds `steadybit.group=<value>` to every discovered target
- Update dependencies

## v1.1.1

- Bump Go to 1.26.3

## v1.0.13

- Bump Go to 1.25.9
- Support if-none-match for the extension list endpoint
- Update dependencies

## v1.0.12

- feat(chart): split image.name into image.registry + image.name
- Support global.priorityClassName
- Update alpine packages in Docker image to address CVEs
- Update dependencies

## v1.0.11

- Update dependencies

## v1.0.9

- Update dependencies

## v1.0.8

- Fix nil pointer in grafana error handling
- Update dependencies

## v1.0.7

- Update dependencies

## v1.0.6

- Update dependencies

## v1.0.5

- Update dependencies

## v1.0.4

- Use uid instead of name for user statement in Dockerfile

## v1.0.1

- Fix for better handling of annotations
- Fix to handle multiple grafana targets
- Update dependencies

## v1.0.0

- Add support for Grafana Alert Rules
	- Discovery of Alert rules
 	- Check alert rules states
- Add support for Grafana annotations
	- Send Steadybit events as annotations
