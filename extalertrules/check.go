/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extalertrules

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-grafana/config"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"net/url"
	"slices"
	"strings"
	"time"
)

type AlertRuleStateCheckAction struct{}

// Make sure action implements all required interfaces
var (
	_ action_kit_sdk.Action[AlertRuleCheckState]           = (*AlertRuleStateCheckAction)(nil)
	_ action_kit_sdk.ActionWithStatus[AlertRuleCheckState] = (*AlertRuleStateCheckAction)(nil)
)

type AlertRuleCheckState struct {
	AlertRuleId         string
	AlertRuleDatasource string
	AlertRuleName       string
	End                 time.Time
	ExpectedState       []string
	StateCheckMode      string
	StateCheckSuccess   bool
}

func NewAlertRuleStateCheckAction() action_kit_sdk.Action[AlertRuleCheckState] {
	return &AlertRuleStateCheckAction{}
}

func (m *AlertRuleStateCheckAction) NewEmptyState() AlertRuleCheckState {
	return AlertRuleCheckState{}
}

func (m *AlertRuleStateCheckAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.check", TargetType),
		Label:       "Alert Rule Check",
		Description: "collects information about the alert rule state and optionally verifies that the state value is the one expected.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType:          TargetType,
			QuantityRestriction: extutil.Ptr(action_kit_api.All),
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "default",
					Description: extutil.Ptr("Find alert rule by id"),
					Query:       "grafana.alert-rule.id=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Grafana"),
		Kind:        action_kit_api.Check,
		TimeControl: action_kit_api.TimeControlInternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr(""),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
			{
				Name:        "expectedStateList",
				Label:       "Expected State List",
				Description: extutil.Ptr(""),
				Type:        action_kit_api.StringArray,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Firing",
						Value: "firing",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Pending",
						Value: "pending",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Normal",
						Value: "normal",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Inactive",
						Value: "inactive",
					},
				}),
				Required: extutil.Ptr(false),
				Order:    extutil.Ptr(2),
			},
			{
				Name:         "stateCheckMode",
				Label:        "State Check Mode",
				Description:  extutil.Ptr("How often should the state be checked ?"),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(stateCheckModeAllTheTime),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "All the time",
						Value: stateCheckModeAllTheTime,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "At least once",
						Value: stateCheckModeAtLeastOnce,
					},
				}),
				Required: extutil.Ptr(true),
				Order:    extutil.Ptr(3),
			},
		},
		Widgets: extutil.Ptr([]action_kit_api.Widget{
			action_kit_api.StateOverTimeWidget{
				Type:  action_kit_api.ComSteadybitWidgetStateOverTime,
				Title: "Grafana Prometheus Alert Rule State",
				Identity: action_kit_api.StateOverTimeWidgetIdentityConfig{
					From: "grafana.alert-rule.id",
				},
				Label: action_kit_api.StateOverTimeWidgetLabelConfig{
					From: "grafana.alert-rule.name",
				},
				State: action_kit_api.StateOverTimeWidgetStateConfig{
					From: "state",
				},
				Tooltip: action_kit_api.StateOverTimeWidgetTooltipConfig{
					From: "tooltip",
				},
				Url: extutil.Ptr(action_kit_api.StateOverTimeWidgetUrlConfig{
					From: extutil.Ptr("url"),
				}),
				Value: extutil.Ptr(action_kit_api.StateOverTimeWidgetValueConfig{
					Hide: extutil.Ptr(true),
				}),
			},
		}),
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("1s"),
		}),
	}
}

func (m *AlertRuleStateCheckAction) Prepare(_ context.Context, state *AlertRuleCheckState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	alertRuleId := request.Target.Attributes["grafana.alert-rule.id"]
	if len(alertRuleId) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'grafana.alert-rule.id' attribute.", nil))
	}

	duration := request.Config["duration"].(float64)
	end := time.Now().Add(time.Millisecond * time.Duration(duration))

	var expectedState []string
	if request.Config["expectedStateList"] != nil {
		expectedState = extutil.ToStringArray(request.Config["expectedStateList"])
	}

	var stateCheckMode string
	if request.Config["stateCheckMode"] != nil {
		stateCheckMode = fmt.Sprintf("%v", request.Config["stateCheckMode"])
	}

	state.AlertRuleId = alertRuleId[0]
	state.AlertRuleDatasource = request.Target.Attributes["grafana.alert-rule.datasource"][0]
	state.AlertRuleName = request.Target.Attributes["grafana.alert-rule.name"][0]
	state.End = end
	state.ExpectedState = expectedState
	state.StateCheckMode = stateCheckMode

	return nil, nil
}

func (m *AlertRuleStateCheckAction) Start(_ context.Context, _ *AlertRuleCheckState) (*action_kit_api.StartResult, error) {
	return nil, nil
}

func (m *AlertRuleStateCheckAction) Status(ctx context.Context, state *AlertRuleCheckState) (*action_kit_api.StatusResult, error) {
	return AlertRuleCheckStatus(ctx, state, RestyClient)
}

func AlertRuleCheckStatus(ctx context.Context, state *AlertRuleCheckState, client *resty.Client) (*action_kit_api.StatusResult, error) {
	now := time.Now()

	var grafanaResponse AlertsStates
	var alertRule *AlertRule

	uri := "/api/prometheus/" + strings.ToLower(state.AlertRuleDatasource) + "/api/v1/rules"
	res, err := client.R().
		SetContext(ctx).
		SetResult(&grafanaResponse).
		Get(uri)

	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to retrieve alerts states from Grafana for Datasource %s with uri %s. Full response: %v", state.AlertRuleDatasource, uri, res.String()), err))
	}

	if !res.IsSuccess() {
		log.Err(err).Msgf("Grafana API responded with unexpected status code %d while retrieving alert rule states for Datasource %s. Full response: %v", res.StatusCode(), state.AlertRuleDatasource, res.String())
	} else {
		for _, alertGroup := range grafanaResponse.AlertsData.AlertsGroups {
			idx := slices.IndexFunc(alertGroup.AlertsRules, func(c AlertRule) bool { return c.Name == state.AlertRuleName })
			if idx != -1 {
				alertRule = &alertGroup.AlertsRules[idx]
				break
			}
		}
		if alertRule == nil {
			return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to retrieve your alert rule %s from Grafana for Datasource %s. Full response: %v", state.AlertRuleName, state.AlertRuleDatasource, res.String()), err))
		}
	}

	completed := now.After(state.End)
	var checkError *action_kit_api.ActionKitError

	if len(state.ExpectedState) > 0 {
		if state.StateCheckMode == stateCheckModeAllTheTime {
			if !slices.Contains(state.ExpectedState, alertRule.State) {
				checkError = extutil.Ptr(action_kit_api.ActionKitError{
					Title: fmt.Sprintf("AlertRule '%s' has state '%s' whereas '%s' is expected.",
						alertRule.Name,
						alertRule.State,
						state.ExpectedState),
					Status: extutil.Ptr(action_kit_api.Failed),
				})
			}
		} else if state.StateCheckMode == stateCheckModeAtLeastOnce {
			if slices.Contains(state.ExpectedState, alertRule.State) {
				state.StateCheckSuccess = true
			}
			if completed && !state.StateCheckSuccess {
				checkError = extutil.Ptr(action_kit_api.ActionKitError{
					Title: fmt.Sprintf("AlertRule '%s' didn't have status '%s' at least once.",
						alertRule.Name,
						state.ExpectedState),
					Status: extutil.Ptr(action_kit_api.Failed),
				})
			}
		}
	}

	metrics := []action_kit_api.Metric{
		*toMetric(state.AlertRuleId, alertRule, now),
	}

	return &action_kit_api.StatusResult{
		Completed: completed,
		Error:     checkError,
		Metrics:   extutil.Ptr(metrics),
	}, nil
}

func toMetric(alertRuleID string, alertRule *AlertRule, now time.Time) *action_kit_api.Metric {
	var tooltip string
	var state string

	tooltip = fmt.Sprintf("Alert rule state is: %s", alertRule.State)
	if alertRule.State == "normal" {
		state = "success"
	} else if alertRule.State == "pending" {
		state = "warn"
	} else if alertRule.State == "inactive" {
		state = "success"
	} else if alertRule.State == "firing" {
		state = "danger"
	}

	return extutil.Ptr(action_kit_api.Metric{
		Name: extutil.Ptr("grafana_alert_rule_state"),
		Metric: map[string]string{
			"grafana.alert-rule.id":   alertRuleID,
			"grafana.alert-rule.name": alertRule.Name,
			"state":                   state,
			"tooltip":                 tooltip,
			"url":                     fmt.Sprintf("%s/alerting/list?search=%s", config.Config.ApiBaseUrl, url.QueryEscape(alertRule.Name)),
		},
		Timestamp: now,
		Value:     0,
	})
}
