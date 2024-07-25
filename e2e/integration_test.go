/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"context"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	actValidate "github.com/steadybit/action-kit/go/action_kit_test/validate"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestWithMinikube(t *testing.T) {
	server := createMockGrafanaServer()
	defer server.http.Close()
	split := strings.SplitAfter(server.http.URL, ":")
	port := split[len(split)-1]

	extlogging.InitZeroLog()

	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-grafana",
		Port: 8083,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", fmt.Sprintf("grafana.apiBaseUrl=http://host.minikube.internal:%s", port),
				"--set", "logging.level=trace",
			}
		},
	}

	e2e.WithDefaultMinikube(t, &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "validate discovery",
			Test: validateDiscovery,
		},
		{
			Name: "test discovery",
			Test: testDiscovery,
		},
		{
			Name: "validate Actions",
			Test: validateActions,
		},
		{
			Name: "alert rule check meets expectations",
			Test: testAlertRuleCheckNormal(server, "normal", "normal", ""),
		},
		{
			Name: "alert rule check fails expectations",
			Test: testAlertRuleCheckFiring(server, "firing", "firing", ""),
		},
		{
			Name: "alert rule check fails expectations",
			Test: testAlertRuleCheckInactive(server, "inactive", "inactive", ""),
		},
		//{
		//	Name: "alert rule check errors",
		//	Test: testAlertRuleCheck(server, "STATUS-500", "", action_kit_api.Failed),
		//},
	})
}

func testAlertRuleCheckNormal(server *mockServer, status, expectedState string, wantedActionStatus action_kit_api.ActionKitErrorStatus) func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
	return func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
		target := &action_kit_api.Target{
			Name: "test_firing",
			Attributes: map[string][]string{
				"grafana.alert-rule.type":       {"alerting"},
				"grafana.alert-rule.datasource": {"loki"},
				"grafana.alert-rule.group":      {"GoldenSignalsAlerts"},
				"grafana.alert-rule.name":       {"test_normal"},
				"grafana.alert-rule.id":         {"loki-GoldenSignalsAlerts-test_normal"},
			},
		}

		config := struct {
			Duration      int    `json:"duration"`
			ExpectedState string `json:"expectedState"`
		}{Duration: 1_000, ExpectedState: expectedState}

		server.state = status
		action, err := e.RunAction("com.steadybit.extension_grafana.alert-rule.check", target, config, &action_kit_api.ExecutionContext{})
		require.NoError(t, err)
		defer func() { _ = action.Cancel() }()

		err = action.Wait()
		if wantedActionStatus == "" {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, fmt.Sprintf("[%s]", wantedActionStatus))
		}
	}
}

func testAlertRuleCheckFiring(server *mockServer, status, expectedState string, wantedActionStatus action_kit_api.ActionKitErrorStatus) func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
	return func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
		target := &action_kit_api.Target{
			Name: "test_firing",
			Attributes: map[string][]string{
				"grafana.alert-rule.health":          {"ok"},
				"grafana.alert-rule.last-evaluation": {""},
				"grafana.alert-rule.type":            {"alerting"},
				"grafana.alert-rule.state":           {"firing"},
				"grafana.alert-rule.datasource":      {"prometheus"},
				"grafana.alert-rule.group":           {"GoldenSignalsAlerts"},
				"grafana.alert-rule.name":            {"test_firing"},
				"grafana.alert-rule.id":              {"prometheus-GoldenSignalsAlerts-test_firing"},
			},
		}

		config := struct {
			Duration      int    `json:"duration"`
			ExpectedState string `json:"expectedState"`
		}{Duration: 1_000, ExpectedState: expectedState}

		server.state = status
		action, err := e.RunAction("com.steadybit.extension_grafana.alert-rule.check", target, config, &action_kit_api.ExecutionContext{})
		require.NoError(t, err)
		defer func() { _ = action.Cancel() }()

		err = action.Wait()
		if wantedActionStatus == "" {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, fmt.Sprintf("[%s]", wantedActionStatus))
		}
	}
}

func testAlertRuleCheckInactive(server *mockServer, status, expectedState string, wantedActionStatus action_kit_api.ActionKitErrorStatus) func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
	return func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
		target := &action_kit_api.Target{
			Name: "test_firing",
			Attributes: map[string][]string{
				"grafana.alert-rule.type":       {"alerting"},
				"grafana.alert-rule.datasource": {"grafana"},
				"grafana.alert-rule.group":      {"GoldenSignalsAlerts"},
				"grafana.alert-rule.name":       {"test_inactive"},
				"grafana.alert-rule.id":         {"grafana-GoldenSignalsAlerts-test_inactive"},
			},
		}

		config := struct {
			Duration      int    `json:"duration"`
			ExpectedState string `json:"expectedState"`
		}{Duration: 1_000, ExpectedState: expectedState}

		server.state = status
		action, err := e.RunAction("com.steadybit.extension_grafana.alert-rule.check", target, config, &action_kit_api.ExecutionContext{})
		require.NoError(t, err)
		defer func() { _ = action.Cancel() }()

		err = action.Wait()
		if wantedActionStatus == "" {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, fmt.Sprintf("[%s]", wantedActionStatus))
		}
	}
}

func validateDiscovery(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	assert.NoError(t, validate.ValidateEndpointReferences("/", e.Client))
}

func testDiscovery(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	target, err := e2e.PollForTarget(ctx, e, "com.steadybit.extension_grafana.alert-rule", func(target discovery_kit_api.Target) bool {
		return e2e.HasAttribute(target, "grafana.alert-rule.id", "host.minikube.internal-prometheus-GoldenSignalsAlerts-test_firing")
	})
	require.NoError(t, err)
	assert.Equal(t, target.TargetType, "com.steadybit.extension_grafana.alert-rule")
	assert.Equal(t, target.Attributes["grafana.alert-rule.type"], []string{"alerting"})
	assert.Equal(t, target.Attributes["grafana.alert-rule.datasource"], []string{"Prometheus"})
	assert.Equal(t, target.Attributes["grafana.alert-rule.name"], []string{"test_firing"})
}

func validateActions(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	assert.NoError(t, actValidate.ValidateEndpointReferences("/", e.Client))
}
