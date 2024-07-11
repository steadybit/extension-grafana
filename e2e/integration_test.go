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
		Name: "extension-stackstate",
		Port: 8083,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", fmt.Sprintf("stackstate.apiBaseUrl=http://host.minikube.internal:%s", port),
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
			Name: "service check meets expectations",
			Test: testServiceCheck(server, "CLEAR", "CLEAR", ""),
		},
		{
			Name: "service check fails expectations",
			Test: testServiceCheck(server, "DEVIATING", "CLEAR", action_kit_api.Failed),
		},
		{
			Name: "service check errors",
			Test: testServiceCheck(server, "STATUS-500", "CLEAR", action_kit_api.Failed),
		},
	})
}

func testServiceCheck(server *mockServer, status, expectedStatus string, wantedActionStatus action_kit_api.ActionKitErrorStatus) func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
	return func(t *testing.T, minikube *e2e.Minikube, e *e2e.Extension) {
		target := &action_kit_api.Target{
			Name: "111",
			Attributes: map[string][]string{
				"stackstate.service.id": {"111"},
				"k8s.service.name":      {"eins-oelf"},
				"k8s.cluster-name":      {"cluster-eins-elf"},
				"k8s.namespace":         {"namespace-eins-elf"},
			},
		}

		config := struct {
			Duration       int    `json:"duration"`
			ExpectedStatus string `json:"expectedStatus"`
		}{Duration: 5_000, ExpectedStatus: expectedStatus}

		server.state = status
		action, err := e.RunAction("com.steadybit.extension_stackstate.service.check", target, config, &action_kit_api.ExecutionContext{})
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

	target, err := e2e.PollForTarget(ctx, e, "com.steadybit.extension_stackstate.service", func(target discovery_kit_api.Target) bool {
		return e2e.HasAttribute(target, "stackstate.service.id", "111")
	})
	require.NoError(t, err)
	assert.Equal(t, target.TargetType, "com.steadybit.extension_stackstate.service")
	assert.Equal(t, target.Attributes["k8s.service.name"], []string{"eins-oelf"})
	assert.Equal(t, target.Attributes["k8s.cluster-name"], []string{"cluster-eins-elf"})
	assert.Equal(t, target.Attributes["k8s.namespace"], []string{"namespace-eins-elf"})
}
