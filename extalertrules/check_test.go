package extalertrules

import (
	"context"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrepareExtractsState(t *testing.T) {
	// Given
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration":          1000 * 60,
			"expectedStateList": []string{"firing"},
		},
		Target: &action_kit_api.Target{
			Attributes: map[string][]string{
				"grafana.alert-rule.id":         {"prometheus-GoldenSignalsAlerts-test_firing"},
				"grafana.alert-rule.health":     {"ok"},
				"grafana.alert-rule.type":       {"alerting"},
				"grafana.alert-rule.datasource": {"prometheus"},
				"grafana.alert-rule.name":       {"test_firing"},
			},
		},
		ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
			ExperimentUri: extutil.Ptr("<uri-to-experiment>"),
			ExecutionUri:  extutil.Ptr("<uri-to-execution>"),
		}),
	})
	action := AlertRuleStateCheckAction{}
	state := action.NewEmptyState()

	// When
	result, err := action.Prepare(context.TODO(), &state, request)

	// Then
	require.Nil(t, result)
	require.Nil(t, err)
	require.Equal(t, "prometheus-GoldenSignalsAlerts-test_firing", state.AlertRuleId)
	//require.Equal(t, "prometheus", state.AlertRuleDatasource)
	require.Equal(t, "test_firing", state.AlertRuleName)
	require.Equal(t, "firing", state.ExpectedState[0])

}
