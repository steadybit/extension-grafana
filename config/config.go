/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// DefaultApiTimeout bounds a single request to the Grafana API. Resty does not apply a timeout by
// default, so without this a slow Grafana API keeps a request open until the caller's context is
// canceled - which for an event listener means exceeding the agent's Request-Timeout.
const DefaultApiTimeout = 5 * time.Second

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	ServiceToken                     string   `json:"serviceToken" split_words:"true" required:"true"`
	ApiBaseUrl                       string   `json:"apiBaseUrl" split_words:"true" required:"true"`
	DiscoveryAttributesExcludesAlert []string `json:"discoveryAttributesExcludesAlertRules" split_words:"true" required:"false"`
	SendAnnotations                  bool     `json:"sendAnnotations" split_words:"true" required:"false" default:"false"`
	// ApiTimeout is the timeout for a single request to the Grafana API.
	ApiTimeout time.Duration `json:"apiTimeout" split_words:"true" required:"false" default:"5s"`
}

// GetApiTimeout returns the configured timeout, falling back to DefaultApiTimeout for
// Specifications that were not populated from the environment (tests, e2e).
func (s *Specification) GetApiTimeout() time.Duration {
	if s.ApiTimeout > 0 {
		return s.ApiTimeout
	}
	return DefaultApiTimeout
}

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
}

func ValidateConfiguration() {
	// You may optionally validate the configuration here.
}
