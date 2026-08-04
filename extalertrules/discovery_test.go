package extalertrules

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllAlertRules_FiltersRecordingRulesAndDeduplicatesTargetsById(t *testing.T) {
	datasources := `[{"uid":"prom-uid","name":"Prometheus","type":"prometheus"}]`
	// kube-prometheus-stack defines the same alerting rule name twice within one group (different
	// thresholds) and ships recording rules, which have no alert state and must not become targets
	promRules := `{"data":{"groups":[
		{"name":"kubernetes-storage","rules":[
			{"name":"KubePersistentVolumeFillingUp","state":"normal","type":"alerting"},
			{"name":"KubePersistentVolumeFillingUp","state":"normal","type":"alerting"},
			{"name":"namespace_workload_pod:kube_pod_owner:relabel","state":"normal","type":"recording"},
			{"name":"KubePodCrashLooping","state":"normal","type":"alerting"}
		]},
		{"name":"other-group","rules":[
			{"name":"KubePersistentVolumeFillingUp","state":"normal","type":"alerting"},
			{"name":"LegacyRuleWithoutType","state":"normal"}
		]}
	]}}`
	grafanaRules := `{"data":{"groups":[]}}`

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		var body string
		switch {
		case req.URL.Path == "/api/datasources":
			body = datasources
		case strings.HasPrefix(req.URL.Path, "/api/datasources/uid/"):
			body = `{"status":"OK"}`
		case req.URL.Path == "/api/prometheus/prom-uid/api/v1/rules":
			body = promRules
		case req.URL.Path == "/api/prometheus/grafana/api/v1/rules":
			body = grafanaRules
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("not found"))}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	client.SetBaseURL("http://grafana.local")

	targets := getAllAlertRules(context.Background(), client)

	// 4 unique targets: the duplicated alerting rule collapses to one per group, the recording
	// rule is dropped, the rule without a type is kept
	require.Len(t, targets, 4)
	seen := make(map[string]bool)
	for _, target := range targets {
		assert.False(t, seen[target.Id], "duplicate target id %s", target.Id)
		seen[target.Id] = true
	}
	assert.True(t, seen["grafana.local-Prometheus-kubernetes-storage-KubePersistentVolumeFillingUp"])
	assert.True(t, seen["grafana.local-Prometheus-kubernetes-storage-KubePodCrashLooping"])
	assert.True(t, seen["grafana.local-Prometheus-other-group-KubePersistentVolumeFillingUp"])
	assert.True(t, seen["grafana.local-Prometheus-other-group-LegacyRuleWithoutType"])
	assert.False(t, seen["grafana.local-Prometheus-kubernetes-storage-namespace_workload_pod:kube_pod_owner:relabel"])
}
