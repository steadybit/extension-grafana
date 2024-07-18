/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extannotations

type AnnotationResponse struct {
	Message string `json:"message"`
	ID      int    `json:"id"`
}

type Annotation struct {
	ID           int                    `json:"id"`
	AlertID      int                    `json:"alertId"`
	DashboardID  int                    `json:"dashboardId"`
	DashboardUID string                 `json:"dashboardUID"`
	PanelID      int                    `json:"panelId"`
	UserID       int                    `json:"userId"`
	UserName     string                 `json:"userName"`
	NewState     string                 `json:"newState"`
	PrevState    string                 `json:"prevState"`
	Time         int64                  `json:"time"`
	TimeEnd      int64                  `json:"timeEnd"`
	Text         string                 `json:"text"`
	Metric       string                 `json:"metric"`
	Tags         []string               `json:"tags"`
	Data         map[string]interface{} `json:"data"`
}
type AnnotationBody struct {
	Tags      []string `json:"tags"`
	Time      int64    `json:"time"`
	TimeEnd   int64    `json:"timeEnd"`
	Text      string   `json:"text"`
	NeedPatch bool
	ID        string
}
