// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extannotations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/event-kit/go/event_kit_api"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"net/http"
	"net/url"
	"time"
)

func RegisterEventListenerHandlers() {
	exthttp.RegisterHttpHandler("/events/experiment-started", handle(onExperimentStarted))
	exthttp.RegisterHttpHandler("/events/experiment-completed", handle(onExperimentCompleted))
	exthttp.RegisterHttpHandler("/events/experiment-step-started", handle(onExperimentStepStarted))
	exthttp.RegisterHttpHandler("/events/experiment-step-completed", handle(onExperimentStepCompleted))
}

type eventHandler func(event event_kit_api.EventRequestBody) (*AnnotationBody, error)

func handle(handler eventHandler) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {

		event, err := parseBodyToEventRequestBody(body)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to decode event request body", err))
			return
		}

		if request, err := handler(event); err == nil {
			if request != nil {
				sendAnnotations(r.Context(), RestyClient, request)
			}
		} else {
			exthttp.WriteError(w, extension_kit.ToError(err.Error(), err))
			return
		}

		exthttp.WriteBody(w, "{}")
	}
}

func onExperimentStarted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	tags := getEventBaseTags(event)
	tags = append(tags, getExecutionTags(event)...)
	tags = removeDuplicates(tags)

	startTime := time.Now().UnixMilli()
	if !event.ExperimentExecution.StartedTime.IsZero() {
		startTime = event.ExperimentExecution.StartedTime.UnixMilli()
	}

	return &AnnotationBody{
		Tags:      tags,
		Text:      "Experiment " + event.ExperimentExecution.ExperimentKey,
		Time:      startTime,
		NeedPatch: false,
	}, nil
}

func onExperimentStepStarted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	if event.ExperimentStepExecution == nil {
		return nil, errors.New("missing ExperimentStepExecution in event")
	}

	tags := getEventBaseTags(event)
	tags = append(tags, getExecutionTags(event)...)
	tags = append(tags, getStepTags(*event.ExperimentStepExecution)...)
	tags = removeDuplicates(tags)

	startTime := time.Now().UnixMilli()
	if !event.ExperimentExecution.StartedTime.IsZero() {
		startTime = event.ExperimentExecution.StartedTime.UnixMilli()
	}

	return &AnnotationBody{
		Tags:      tags,
		Text:      "Experiment " + event.ExperimentExecution.ExperimentKey + "Step " + getActionName(*event.ExperimentStepExecution),
		Time:      startTime,
		NeedPatch: false,
	}, nil
}

func onExperimentCompleted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	tags := getExecutionTags(event)
	return &AnnotationBody{Tags: tags, Time: event.ExperimentExecution.StartedTime.UnixMilli(), TimeEnd: event.ExperimentExecution.EndedTime.UnixMilli(), NeedPatch: true}, nil
}

func onExperimentStepCompleted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	tags := getStepTags(*event.ExperimentStepExecution)
	return &AnnotationBody{Tags: tags, Time: event.ExperimentStepExecution.StartedTime.UnixMilli(), TimeEnd: event.ExperimentStepExecution.EndedTime.UnixMilli(), NeedPatch: true}, nil
}

func getActionName(stepExecution event_kit_api.ExperimentStepExecution) string {
	actionName := *stepExecution.ActionId
	if stepExecution.ActionName != nil {
		actionName = *stepExecution.ActionName
	}
	if stepExecution.CustomLabel != nil {
		actionName = *stepExecution.CustomLabel
	}
	return actionName
}

func getEventBaseTags(event event_kit_api.EventRequestBody) []string {
	tags := []string{
		"source:Steadybit",
		"environment_name:" + event.Environment.Name,
		"event_name:" + event.EventName,
		"event_time:" + event.EventTime.String(),
		"event_id:" + event.Id.String(),
		"tenant_name:" + event.Tenant.Name,
		"tenant_key:" + event.Tenant.Key,
	}

	if event.Team != nil {
		tags = append(tags, "team_name:"+event.Team.Name, "team_key:"+event.Team.Key)
	}

	return tags
}

func getExecutionTags(event event_kit_api.EventRequestBody) []string {
	if event.ExperimentExecution == nil {
		return []string{}
	}
	tags := []string{
		"execution_id:" + fmt.Sprintf("%g", event.ExperimentExecution.ExecutionId),
		"experiment_key:" + event.ExperimentExecution.ExperimentKey,
		"experiment_name:" + event.ExperimentExecution.Name,
	}

	if event.ExperimentExecution.StartedTime.IsZero() {
		tags = append(tags, "started_time:"+time.Now().Format(time.RFC3339))
	} else {
		tags = append(tags, "started_time:"+event.ExperimentExecution.StartedTime.Format(time.RFC3339))
	}

	if event.ExperimentExecution.EndedTime != nil && !(*event.ExperimentExecution.EndedTime).IsZero() {
		tags = append(tags, "ended_time:"+event.ExperimentExecution.EndedTime.Format(time.RFC3339))
	}

	return tags
}

func getStepTags(step event_kit_api.ExperimentStepExecution) []string {
	var tags []string
	if step.Type == event_kit_api.Action {
		tags = append(tags, "step_action_id:"+*step.ActionId)
	}
	if step.ActionName != nil {
		tags = append(tags, "step_action_name:"+*step.ActionName)
	}
	if step.CustomLabel != nil {
		tags = append(tags, "step_custom_label:"+*step.CustomLabel)
	}
	return tags
}

func removeDuplicates(tags []string) []string {
	allKeys := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		if _, value := allKeys[tag]; !value {
			allKeys[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

func parseBodyToEventRequestBody(body []byte) (event_kit_api.EventRequestBody, error) {
	var event event_kit_api.EventRequestBody
	err := json.Unmarshal(body, &event)
	return event, err
}

func sendAnnotations(ctx context.Context, client *resty.Client, annotation *AnnotationBody) {

	if annotation.NeedPatch {
		// Find the annotation
		var annotationsFound []Annotation
		res, err := client.R().
			SetContext(ctx).
			SetResult(&annotationsFound).
			SetQueryParamsFromValues(url.Values{
				"tags":  annotation.Tags,
				"limit": {"10"},
				"from":  {fmt.Sprintf("%v", annotation.Time)},
			}).
			Get("/api/annotations")

		if err != nil {
			log.Err(err).Msgf("Failed to find annotation with these tags %s. Full response: %v", annotation.Tags, res.String())
		}

		if len(annotationsFound) == 1 {
			var annotationResponse AnnotationResponse
			res, err := client.R().
				SetContext(ctx).
				SetResult(&annotationResponse).
				SetBody(fmt.Sprintf(`{"timeEnd": %s}`, annotation.TimeEnd)).
				Put("/api/annotations/")

			if err != nil {
				log.Err(err).Msgf("Failed to patch annotation ID %s. Full response: %v", annotationsFound[0].ID, res.String())
			}

			if !res.IsSuccess() {
				log.Err(err).Msgf("Grafana API responded with unexpected status code %d while patching annotations. Full response: %v", res.StatusCode(), res.String())
			}
		} else if len(annotationsFound) == 0 {
			log.Err(err).Msgf("Failed to find annotation with tags %s. Full response: %v", annotation.Tags, err)
		} else if len(annotationsFound) > 1 {
			log.Err(err).Msgf("Found multiple annotations with tags %s. Full response: %v", annotation.Tags, err)
		}

	} else {
		annotationBytes, err := json.Marshal(annotation)
		if err != nil {
			log.Err(err).Msgf("Failed to marshal annotation %v. Full response: %v", annotation, err)
		}

		var annotationResponse AnnotationResponse
		res, err := client.R().
			SetContext(ctx).
			SetResult(&annotationResponse).
			SetBody(annotationBytes).
			Post("/api/annotations")

		if err != nil {
			log.Err(err).Msgf("Failed to post annotation, body: %v. Full response: %v", annotationBytes, res.String())
		}

		if !res.IsSuccess() {
			log.Err(err).Msgf("Grafana API responded with unexpected status code %d while posting annotations. Full response: %v", res.StatusCode(), res.String())
		}

	}

}