# Changelog

## (next)

- Support if-none-match for the extension list endpoint

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
