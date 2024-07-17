/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package main

import (
	_ "github.com/KimMachineGun/automemlimit" // By default, it sets `GOMEMLIMIT` to 90% of cgroup's memory limit.
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/event-kit/go/event_kit_api"
	"github.com/steadybit/extension-grafana/config"
	"github.com/steadybit/extension-grafana/extalertrules"
	"github.com/steadybit/extension-grafana/extannotations"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	_ "go.uber.org/automaxprocs" // Importing automaxprocs automatically adjusts GOMAXPROCS.
	_ "net/http/pprof"           //allow pprof
)

func main() {
	extlogging.InitZeroLog()

	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.DebugLevel)

	exthealth.SetReady(false)
	exthealth.StartProbes(8084)

	config.ParseConfiguration()
	config.ValidateConfiguration()
	initRestyClient()

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))

	discovery_kit_sdk.Register(extalertrules.NewAlertDiscovery())
	action_kit_sdk.RegisterAction(extalertrules.NewAlertRuleStateCheckAction())
	extannotations.RegisterEventListenerHandlers()

	action_kit_sdk.InstallSignalHandler()

	action_kit_sdk.RegisterCoverageEndpoints()

	exthealth.SetReady(true)

	exthttp.Listen(exthttp.ListenOpts{
		Port: 8083,
	})
}

func initRestyClient() {
	extalertrules.RestyClient = resty.New()
	extalertrules.RestyClient.SetBaseURL(config.Config.ApiBaseUrl)
	extalertrules.RestyClient.SetHeader("Authorization", "Bearer "+config.Config.ServiceToken)
	extalertrules.RestyClient.SetHeader("Content-Type", "application/json")

	extannotations.RestyClient = resty.New()
	extannotations.RestyClient.SetBaseURL(config.Config.ApiBaseUrl)
	extannotations.RestyClient.SetHeader("Authorization", "Bearer "+config.Config.ServiceToken)
	extannotations.RestyClient.SetHeader("Content-Type", "application/json")
}

type ExtensionListResponse struct {
	action_kit_api.ActionList       `json:",inline"`
	discovery_kit_api.DiscoveryList `json:",inline"`
	event_kit_api.EventListenerList `json:",inline"`
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		ActionList:    action_kit_sdk.GetActionList(),
		DiscoveryList: discovery_kit_sdk.GetDiscoveryList(),
		EventListenerList: event_kit_api.EventListenerList{
			EventListeners: []event_kit_api.EventListener{
				{
					Method:   "POST",
					Path:     "/events/experiment-started",
					ListenTo: []string{"experiment.execution.created"},
				},
				{
					Method:   "POST",
					Path:     "/events/experiment-completed",
					ListenTo: []string{"experiment.execution.completed", "experiment.execution.failed", "experiment.execution.canceled", "experiment.execution.errored"},
				},
				{
					Method:   "POST",
					Path:     "/events/experiment-step-started",
					ListenTo: []string{"experiment.execution.step-started"},
				},
				{
					Method:   "POST",
					Path:     "/events/experiment-step-completed",
					ListenTo: []string{"experiment.execution.step-completed", "experiment.execution.step-canceled", "experiment.execution.step-errored", "experiment.execution.step-failed"},
				},
			},
		},
	}
}
