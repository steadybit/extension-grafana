/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package e2e

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-grafana/extalertrules"
	"github.com/steadybit/extension-kit/exthttp"
	"net"
	"net/http"
	"net/http/httptest"
	"time"
)

type mockServer struct {
	http  *httptest.Server
	state string
}

func createMockGrafanaServer() *mockServer {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		panic(fmt.Sprintf("httptest: failed to listen: %v", err))
	}
	mux := http.NewServeMux()

	server := httptest.Server{Listener: listener, Config: &http.Server{Handler: mux}}
	server.Start()
	log.Info().Str("url", server.URL).Msg("Started Mock-Server")

	mock := &mockServer{http: &server, state: "CLEAR"}
	mux.Handle("GET /api/datasources", handler(mock.viewDatasources))
	mux.Handle("GET /api/prometheus/loki/api/v1/rules", handler(mock.viewLokiAlertRules))
	mux.Handle("GET /api/prometheus/prometheus/api/v1/rules", handler(mock.viewPrometheusAlertRules))
	mux.Handle("GET /api/prometheus/grafana/api/v1/rules", handler(mock.viewGrafanaAlertRules))
	return mock
}

func handler[T any](getter func() T) http.Handler {
	return exthttp.PanicRecovery(exthttp.LogRequest(exthttp.GetterAsHandler(getter)))
}

func (m *mockServer) viewDatasources() []extalertrules.DataSource {
	if m.state == "STATUS-500" {
		panic("status 500")
	}
	return []extalertrules.DataSource{
		{
			ID:          1,
			UID:         "prometheus",
			OrgID:       1,
			Name:        "Prometheus",
			Type:        "prometheus",
			TypeName:    "Prometheus",
			TypeLogoUrl: "public/app/plugins/datasource/prometheus/img/prometheus_logo.svg",
			Access:      "proxy",
			URL:         "http://prometheus-kube-prometheus-prometheus.prometheus:9090/",
			User:        "",
			Database:    "",
			BasicAuth:   false,
			IsDefault:   true,
			JsonData:    nil,
			ReadOnly:    true,
		},
		{
			ID:          3,
			UID:         "loki",
			OrgID:       3,
			Name:        "Loki",
			Type:        "loki",
			TypeName:    "Loki",
			TypeLogoUrl: "public/app/plugins/datasource/prometheus/img/loki_logo.svg",
			Access:      "proxy",
			URL:         "http://prometheus-kube-prometheus-loki.prometheus:9090/",
			User:        "",
			Database:    "",
			BasicAuth:   false,
			IsDefault:   true,
			JsonData:    nil,
			ReadOnly:    true,
		},
		{
			ID:          2,
			UID:         "alertmanager",
			OrgID:       1,
			Name:        "Alertmanager",
			Type:        "alertmanager",
			TypeName:    "Alertmanager",
			TypeLogoUrl: "public/app/plugins/datasource/alertmanager/img/logo.svg",
			Access:      "proxy",
			URL:         "http://prometheus-kube-prometheus-alertmanager.prometheus:9093/",
			User:        "",
			Database:    "",
			BasicAuth:   false,
			IsDefault:   true,
			JsonData:    nil,
			ReadOnly:    true,
		},
	}
}

func (m *mockServer) viewPrometheusAlertRules() extalertrules.AlertsStates {
	if m.state == "STATUS-500" {
		panic("status 500")
	}
	return extalertrules.AlertsStates{
		AlertsData: extalertrules.AlertsData{AlertsGroups: []extalertrules.AlertGroup{
			{
				Name: "GoldenSignalsAlerts",
				AlertsRules: []extalertrules.AlertRule{
					{
						State:          "firing",
						Name:           "test_firing",
						Health:         "ok",
						Type:           "alerting",
						LastEvaluation: time.Now().Add(time.Duration(-10) * time.Minute),
					},
				},
			},
		}},
		Status: "success",
	}
}

func (m *mockServer) viewGrafanaAlertRules() extalertrules.AlertsStates {
	if m.state == "STATUS-500" {
		panic("status 500")
	}
	return extalertrules.AlertsStates{
		AlertsData: extalertrules.AlertsData{AlertsGroups: []extalertrules.AlertGroup{
			{
				Name: "Trivy",
				AlertsRules: []extalertrules.AlertRule{
					{
						State:          "inactive",
						Name:           "test_inactive",
						Health:         "ok",
						Type:           "alerting",
						LastEvaluation: time.Now().Add(time.Duration(-10) * time.Minute),
					},
				},
			},
		}},
		Status: "success",
	}
}

func (m *mockServer) viewLokiAlertRules() extalertrules.AlertsStates {
	if m.state == "STATUS-500" {
		panic("status 500")
	}
	return extalertrules.AlertsStates{
		AlertsData: extalertrules.AlertsData{AlertsGroups: []extalertrules.AlertGroup{
			{
				Name: "GoldenSignalsAlerts",
				AlertsRules: []extalertrules.AlertRule{
					{
						State:          "normal",
						Name:           "test_normal",
						Health:         "ok",
						Type:           "alerting",
						LastEvaluation: time.Now().Add(time.Duration(-10) * time.Minute),
					},
				},
			},
		}},
		Status: "success",
	}
}
