/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

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
		Id:      TargetType,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),

		// Labels used in the UI
		Label: discovery_kit_api.PluralLabel{One: "Robot", Other: "Robots"},

		// Category for the targets to appear in
		Category: extutil.Ptr("example"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "robot.reportedBy"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func (d *alertDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "robot.reportedBy",
			Label: discovery_kit_api.PluralLabel{
				One:   "Reported by",
				Other: "Reported by",
			},
		},
	}
}

func (d *alertDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return getAllAlertRules(ctx, RestyClient), nil
}

func getAllAlertRules(ctx context.Context, client *resty.Client) []discovery_kit_api.Target {
	result := make([]discovery_kit_api.Target, 0, 1000)

	datasources := getAllDatasources(ctx, client)
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

		if res.StatusCode() != 200 {
			log.Error().Msgf("Grafana API responded with unexpected status code %d while retrieving alert states. Full response: %v",
				res.StatusCode(),
				res.String())
			return result
		}

		log.Trace().Msgf("Stackstate response: %v", perDatasourceResponse.AlertsData)

		for _, alertGroup := range perDatasourceResponse.AlertsData.AlertsGroups {
			for _, rule := range alertGroup.AlertsRules {
				Id := datasource.Name + "-" + alertGroup.Name + "-" + rule.Name
				result = append(result, discovery_kit_api.Target{
					Id:         Id,
					TargetType: TargetType,
					Label:      rule.Name,
					Attributes: map[string][]string{
						"grafana.alert-rule.health":          {rule.Health},
						"grafana.alert-rule.last-evaluation": {rule.LastEvaluation.Format("2006-01-02 15:04:05")},
						"grafana.alert-rule.type":            {rule.Type},
						"grafana.alert-rule.state":           {rule.State},
						"grafana.alert-rule.datasource":      {datasource.Name},
						"grafana.alert-rule.group":           {alertGroup.Name},
						"grafana.alert-rule.name":            {rule.Name},
						"grafana.alert-rule.id":              {Id},
					}})
			}
		}
	}

	// add the grafana ones also
	datasource := DataSource{Name: "grafana"}
	var grafanaAlertRules AlertsStates
	res, err := client.R().
		SetContext(ctx).
		SetResult(&grafanaAlertRules).
		Get("/prometheus/grafana/api/v1/rules")

	if err != nil {
		log.Err(err).Msgf("Failed to retrieve alerts states from Grafana. Full response: %v", res.String())
		return result
	}

	if res.StatusCode() != 200 {
		log.Error().Msgf("Grafana API responded with unexpected status code %d while retrieving alert states. Full response: %v",
			res.StatusCode(),
			res.String())
		return result
	}

	log.Trace().Msgf("Stackstate response: %v", grafanaAlertRules.AlertsData)

	for _, alertGroup := range grafanaAlertRules.AlertsData.AlertsGroups {
		for _, rule := range alertGroup.AlertsRules {
			Id := datasource.Name + "-" + alertGroup.Name + "-" + rule.Name
			result = append(result, discovery_kit_api.Target{
				Id:         Id,
				TargetType: TargetType,
				Label:      rule.Name,
				Attributes: map[string][]string{
					"grafana.alert-rule.health":          {rule.Health},
					"grafana.alert-rule.last-evaluation": {rule.LastEvaluation.Format("2006-01-02 15:04:05")},
					"grafana.alert-rule.type":            {rule.Type},
					"grafana.alert-rule.state":           {rule.State},
					"grafana.alert-rule.datasource":      {datasource.Name},
					"grafana.alert-rule.group":           {alertGroup.Name},
					"grafana.alert-rule.name":            {rule.Name},
					"grafana.alert-rule.id":              {Id},
				}})
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesAlert)
}

func getAllDatasources(ctx context.Context, client *resty.Client) []DataSource {
	var grafanaResponse []DataSource
	res, err := client.R().
		SetContext(ctx).
		SetResult(&grafanaResponse).
		Get("/api/datasources")

	if err != nil {
		log.Err(err).Msgf("Failed to retrieve alerts states from Grafana. Full response: %v", res.String())
		return grafanaResponse
	}

	if res.StatusCode() != 200 {
		log.Error().Msgf("Grafana API responded with unexpected status code %d while retrieving alert states. Full response: %v",
			res.StatusCode(),
			res.String())
		return grafanaResponse
	}

	i := 0
	for _, ds := range grafanaResponse {
		if isAlertRuleCompatible(ds) {
			grafanaResponse[i] = ds
			i++
		}
	}
	log.Trace().Msgf("Grafana response: %v", grafanaResponse)

	return grafanaResponse
}

func isAlertRuleCompatible(ds DataSource) bool {
	// Only some datasources are compatibles with alert rules https://grafana.com/docs/grafana/latest/alerting/fundamentals/alert-rules/#data-source-managed-alert-rules
	compatibleDatasources := map[string]bool{
		"prometheus": true,
		"loki":       true,
	}
	return compatibleDatasources[ds.Type]
}