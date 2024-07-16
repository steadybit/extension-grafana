/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extalertrules

import "time"

type DataSource struct {
	ID          int         `json:"id"`
	UID         string      `json:"uid"`
	OrgID       int         `json:"orgId"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	TypeName    string      `json:"typeName"`
	TypeLogoUrl string      `json:"typeLogoUrl"`
	Access      string      `json:"access"`
	URL         string      `json:"url"`
	User        string      `json:"user"`
	Database    string      `json:"database"`
	BasicAuth   bool        `json:"basicAuth"`
	IsDefault   bool        `json:"isDefault"`
	JsonData    interface{} `json:"jsonData"`
	ReadOnly    bool        `json:"readOnly"`
}
type AlertsStates struct {
	AlertsData AlertsData `json:"data,omitempty"`
	Status     string     `json:"status"`
}

type AlertsData struct {
	AlertsGroups []AlertGroup `json:"groups,omitempty"`
}

type AlertGroup struct {
	Name        string      `json:"name"`
	AlertsRules []AlertRule `json:"rules,omitempty"`
}

type AlertRule struct {
	State          string    `json:"state"`
	Name           string    `json:"name"`
	Health         string    `json:"health"`
	Type           string    `json:"type"`
	LastEvaluation time.Time `json:"lastEvaluation"`
}
