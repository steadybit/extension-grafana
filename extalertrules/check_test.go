package extalertrules

import (
	"context"
	"errors"
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-grafana/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

// RoundTripperFunc lets us stub HTTP responses.
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(fn RoundTripperFunc) *resty.Client {
	return resty.NewWithClient(&http.Client{Transport: fn})
}

func TestToMetric(t *testing.T) {
	// override base URL
	config.Config.ApiBaseUrl = "http://grafana.local"

	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	alert := &AlertRule{
		Name:  "my-rule",
		State: "normal",
	}

	m := toMetric("rule-id", alert, now)
	metric := m.Metric

	assert.Equal(t, "grafana_alert_rule_state", *m.Name)
	assert.Equal(t, "success", metric["state"], "normal should map to success")
	assert.Equal(t, "Alert rule state is: normal", metric["tooltip"])
	assert.True(t, strings.HasPrefix(metric["url"], "http://grafana.local/alerting/list"), "url should start with base URL")
	assert.Equal(t, now, m.Timestamp)
}

func TestAlertRuleCheckStatus_Success_NoExpected(t *testing.T) {
	// stub Grafana API returning one group with our rule
	body := `{"data":{"groups":[{"rules":[{"name":"r1","state":"firing"}]}]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	state := &AlertRuleCheckState{
		AlertRuleDatasource: "DS1",
		AlertRuleName:       "r1",
		AlertRuleId:         "id1",
		// in the future → not yet completed
		End: time.Now().Add(1 * time.Minute),
	}

	res, err := AlertRuleCheckStatus(context.Background(), state, client)
	assert.NoError(t, err)
	assert.False(t, res.Completed)
	assert.Nil(t, res.Error)
	assert.Len(t, *res.Metrics, 1)
	assert.Equal(t, "danger", (*res.Metrics)[0].Metric["state"])
}

func TestAlertRuleCheckStatus_MissingRule(t *testing.T) {
	body := `{"data":{"groups":[{"rules":[{"name":"other","state":"pending"}]}]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	state := &AlertRuleCheckState{
		AlertRuleDatasource: "DSX",
		AlertRuleName:       "rX",
		AlertRuleId:         "idx",
		End:                 time.Now().Add(1 * time.Minute),
	}

	res, err := AlertRuleCheckStatus(context.Background(), state, client)
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestAlertRuleCheckStatus_HTTPError(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})

	state := &AlertRuleCheckState{
		AlertRuleDatasource: "DSX",
		AlertRuleName:       "rX",
		AlertRuleId:         "idx",
		End:                 time.Now().Add(1 * time.Minute),
	}

	res, err := AlertRuleCheckStatus(context.Background(), state, client)
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestAlertRuleCheckStatus_AllTheTimeMode_Mismatch(t *testing.T) {
	body := `{"data":{"groups":[{"rules":[{"name":"r2","state":"firing"}]}]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	state := &AlertRuleCheckState{
		AlertRuleDatasource: "DS1",
		AlertRuleName:       "r2",
		AlertRuleId:         "id2",
		ExpectedState:       []string{"normal"},
		StateCheckMode:      stateCheckModeAllTheTime,
		End:                 time.Now().Add(-1 * time.Second), // already completed
	}

	res, err := AlertRuleCheckStatus(context.Background(), state, client)
	// Should get a failure error
	assert.NoError(t, err, "Status call itself should not error")
	assert.True(t, res.Completed)
	assert.NotNil(t, res.Error)
	assert.Contains(t, res.Error.Title, "has state 'firing' whereas")
}

func TestAlertRuleCheckStatus_AtLeastOnceMode(t *testing.T) {
	body := `{"data":{"groups":[{"rules":[{"name":"r3","state":"pending"}]}]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	state := &AlertRuleCheckState{
		AlertRuleDatasource: "DS1",
		AlertRuleName:       "r3",
		AlertRuleId:         "id3",
		ExpectedState:       []string{"pending"},
		StateCheckMode:      stateCheckModeAtLeastOnce,
		// simulate already finished
		End: time.Now().Add(-1 * time.Second),
	}

	res, err := AlertRuleCheckStatus(context.Background(), state, client)
	assert.NoError(t, err)
	assert.True(t, res.Completed)
	assert.Nil(t, res.Error, "should succeed because we saw 'pending' at least once")
}

func TestAlertRuleCheckStatus_AtLeastOnceMode_NoMatch(t *testing.T) {
	body := `{"data":{"groups":[{"rules":[{"name":"r3","state":"firing"}]}]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	state := &AlertRuleCheckState{
		AlertRuleDatasource: "DS1",
		AlertRuleName:       "r3",
		AlertRuleId:         "id3",
		ExpectedState:       []string{"pending"},
		StateCheckMode:      stateCheckModeAtLeastOnce,
		End:                 time.Now().Add(-1 * time.Second),
	}

	res, err := AlertRuleCheckStatus(context.Background(), state, client)
	assert.NoError(t, err)
	assert.True(t, res.Completed)
	assert.NotNil(t, res.Error)
	assert.Contains(t, res.Error.Title, "didn't have status")
}
