/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	ServiceToken                     string   `json:"serviceToken" split_words:"true" required:"true"`
	ApiBaseUrl                       string   `json:"apiBaseUrl" split_words:"true" required:"true"`
	DiscoveryAttributesExcludesAlert []string `json:"discoveryAttributesExcludesAlertRules" split_words:"true" required:"false"`
	SendAnnotations                  bool     `json:"sendAnnotations" split_words:"true" required:"false" default:"false"`
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
