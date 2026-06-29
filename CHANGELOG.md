# Changelog

## Unreleased

- Add a "Fail early" option to the alert rule check. When enabled (the default, matching the previous behavior), the check fails as soon as a deviating state is observed. When disabled, the check keeps collecting events for the whole duration and only fails at the end of the step.

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
