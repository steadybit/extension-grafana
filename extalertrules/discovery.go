/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extalertrules

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-grafana/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"net/url"
	"strconv"
	"time"
)

type alertDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*alertDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*alertDiscovery)(nil)
)

func NewAlertDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &alertDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 1*time.Minute),
	)
}

func (d *alertDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("1m"),
		},
	}
}

func (d *alertDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetType,
		Label:    discovery_kit_api.PluralLabel{One: "Grafana alert-rule", Other: "Grafana alert-rules"},
		Category: extutil.Ptr("monitoring"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "grafana.alert-rule.name"},
				{Attribute: "grafana.alert-rule.group"},
				{Attribute: "grafana.alert-rule.datasource"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "grafana.alert-rule.name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (d *alertDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "grafana.alert-rule.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "Alert Rule",
				Other: "Alert Rules",
			},
		}, {
			Attribute: "grafana.alert-rule.group",
			Label: discovery_kit_api.PluralLabel{
				One:   "Grafana alert group",
				Other: "Grafana alert groups",
			},
		}, {
			Attribute: "grafana.alert-rule.datasource",
			Label: discovery_kit_api.PluralLabel{
				One:   "Grafana datasource",
				Other: "Grafana datasources",
			},
		},
	}
}

func (d *alertDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return getAllAlertRules(ctx, RestyClient), nil
}

func getAllAlertRules(ctx context.Context, client *resty.Client) []discovery_kit_api.Target {
	result := make([]discovery_kit_api.Target, 0, 1000)
	urlParsed, _ := url.Parse(client.BaseURL)
	grafanaHost := urlParsed.Hostname()

	datasources := getAllCompatibleDatasource(ctx, client)
	// for every datasource compatible
	for _, datasource := range datasources {
		var perDatasourceResponse AlertsStates
		res, err := client.R().
			SetContext(ctx).
			SetResult(&perDatasourceResponse).
			Get("/api/prometheus/" + datasource.UID + "/api/v1/rules")

		if err != nil {
			log.Err(err).Msgf("Failed to retrieve alerts states from Grafana. Full response: %v", res.String())
			return result
		}

		if res.StatusCode() != 200 && res.StatusCode() != 404 {
			log.Warn().Msgf("Grafana API responded with unexpected status code %d while retrieving alert states. Full response: %v",
				res.StatusCode(),
				res.String())
			return result
		} else {
			log.Trace().Msgf("Grafana response: %v", perDatasourceResponse.AlertsData)

			for _, alertGroup := range perDatasourceResponse.AlertsData.AlertsGroups {
				for _, rule := range alertGroup.AlertsRules {
					Id := grafanaHost + "-" + datasource.Name + "-" + alertGroup.Name + "-" + rule.Name
					result = append(result, discovery_kit_api.Target{
						Id:         Id,
						TargetType: TargetType,
						Label:      rule.Name,
						Attributes: map[string][]string{
							"grafana.alert-rule.type":       {rule.Type},
							"grafana.alert-rule.datasource": {datasource.Name},
							"grafana.alert-rule.group":      {alertGroup.Name},
							"grafana.alert-rule.name":       {rule.Name},
							"grafana.alert-rule.id":         {Id},
							"grafana.host":                  {grafanaHost},
						}})
				}
			}
		}
	}

	// add the grafana ones also
	datasource := DataSource{Name: "Grafana", Type: "grafana"}
	var grafanaAlertRules AlertsStates
	res, err := client.R().
		SetContext(ctx).
		SetResult(&grafanaAlertRules).
		Get("/api/prometheus/grafana/api/v1/rules")

	if err != nil {
		log.Err(err).Msgf("Failed to retrieve alerts states from Grafana. Full response: %v", res.String())
		return result
	}

	if res.StatusCode() != 200 && res.StatusCode() != 404 {
		log.Warn().Msgf("Grafana API responded with unexpected status code %d while retrieving alert states. Full response: %v",
			res.StatusCode(),
			res.String())
	} else {
		log.Trace().Msgf("Grafana response: %v", grafanaAlertRules.AlertsData)

		for _, alertGroup := range grafanaAlertRules.AlertsData.AlertsGroups {
			for _, rule := range alertGroup.AlertsRules {
				Id := grafanaHost + "-" + datasource.Name + "-" + alertGroup.Name + "-" + rule.Name
				result = append(result, discovery_kit_api.Target{
					Id:         Id,
					TargetType: TargetType,
					Label:      rule.Name,
					Attributes: map[string][]string{
						"grafana.alert-rule.type":       {rule.Type},
						"grafana.alert-rule.datasource": {datasource.Name},
						"grafana.alert-rule.group":      {alertGroup.Name},
						"grafana.alert-rule.name":       {rule.Name},
						"grafana.alert-rule.id":         {Id},
						"grafana.host":                  {grafanaHost},
					}})
			}
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesAlert)
}

func getAllCompatibleDatasource(ctx context.Context, client *resty.Client) []DataSource {
	var grafanaResponse []DataSource
	res, err := client.R().
		SetContext(ctx).
		SetResult(&grafanaResponse).
		Get("/api/datasources")

	if err != nil {
		log.Err(err).Msgf("Failed to retrieve alerts states from Grafana. Full response: %v", res.String())
		return grafanaResponse
	}

	grafanaResponseFiltered := make([]DataSource, 0)
	if res.StatusCode() != 200 {
		log.Warn().Msgf("Grafana API responded with unexpected status code %d while retrieving alert states. Full response: %v",
			res.StatusCode(),
			res.String())
	} else {
		for _, ds := range grafanaResponse {
			if isAlertRuleCompatible(ds) && isDatasourceHealthy(ctx, client, ds) {
				grafanaResponseFiltered = append(grafanaResponseFiltered, ds)
			}
		}
		log.Trace().Msgf("Grafana response: %v", grafanaResponse)
		log.Trace().Msgf("Grafana filtered response: %v", grafanaResponseFiltered)
	}
	return grafanaResponseFiltered
}

func isAlertRuleCompatible(ds DataSource) bool {
	// Only some datasources are compatibles with alert rules https://grafana.com/docs/grafana/latest/alerting/fundamentals/alert-rules/#data-source-managed-alert-rules
	compatibleDatasources := map[string]bool{
		"prometheus": true,
		"loki":       true,
	}
	return compatibleDatasources[ds.Type]
}

func isDatasourceHealthy(ctx context.Context, client *resty.Client, ds DataSource) bool {
	res, _ := client.R().
		SetContext(ctx).
		Get("/api/datasources/" + strconv.Itoa(ds.ID) + "/health")

	if res.StatusCode() != 200 {
		log.Warn().Msgf("Datasource %s is not healthy, skipping discovery of alert rules for this one..", ds.Name)
	}

	return res.StatusCode() == 200
}
