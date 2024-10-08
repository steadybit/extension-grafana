// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extannotations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aquilax/truncate"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/event-kit/go/event_kit_api"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
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
	if event.ExperimentExecution != nil {
		if !event.ExperimentExecution.StartedTime.IsZero() {
			startTime = event.ExperimentExecution.StartedTime.UnixMilli()
		}
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
	if event.ExperimentStepExecution != nil {
		if !event.ExperimentStepExecution.StartedTime.IsZero() {
			startTime = event.ExperimentStepExecution.StartedTime.UnixMilli()
		}
	}

	return &AnnotationBody{
		Tags:      tags,
		Text:      "Step " + getActionName(*event.ExperimentStepExecution),
		Time:      startTime,
		NeedPatch: false,
	}, nil
}

func onExperimentCompleted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	log.Debug().Msg("onExperimentStepCompleted, tagging:")
	tags := getEventBaseTags(event)
	log.Debug().Msgf("getEventBaseTags: %v", tags)
	tags = append(tags, getExecutionTags(event)...)
	log.Debug().Msgf("getExecutionTags: %v", tags)
	tags = removeDuplicates(tags)
	log.Debug().Msgf("removeDuplicates: %v", tags)
	return &AnnotationBody{Tags: tags, Time: event.ExperimentExecution.StartedTime.UnixMilli(), TimeEnd: event.ExperimentExecution.EndedTime.UnixMilli(), NeedPatch: true}, nil
}

func onExperimentStepCompleted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	log.Debug().Msg("onExperimentStepCompleted, tagging:")
	tags := getEventBaseTags(event)
	log.Debug().Msgf("getEventBaseTags: %v", tags)
	tags = append(tags, getExecutionTags(event)...)
	log.Debug().Msgf("getExecutionTags: %v", tags)
	tags = append(tags, getStepTags(*event.ExperimentStepExecution)...)
	log.Debug().Msgf("getStepTags: %v", tags)
	tags = removeDuplicates(tags)
	log.Debug().Msgf("removeDuplicates: %v", tags)

	var startTime int64
	var endTime int64
	if event.ExperimentStepExecution != nil {
		if event.ExperimentStepExecution.StartedTime != nil {
			startTime = event.ExperimentStepExecution.StartedTime.UnixMilli()
		}
		if event.ExperimentStepExecution.EndedTime != nil {
			endTime = event.ExperimentStepExecution.EndedTime.UnixMilli()
		}
	}

	return &AnnotationBody{Tags: tags, Time: startTime, TimeEnd: endTime, NeedPatch: true}, nil
}

func getActionName(stepExecution event_kit_api.ExperimentStepExecution) string {
	actionName := ""
	if stepExecution.ActionId != nil {
		actionName = *stepExecution.ActionId
	}
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
		"env:" + truncate.Truncate(event.Environment.Name, 20, "...", truncate.PositionEnd),
		"event:" + truncate.Truncate(event.EventName, 50, "...", truncate.PositionEnd),
		"event_id:" + event.Id.String(),
		"tenant:" + truncate.Truncate(event.Tenant.Name, 10, "...", truncate.PositionEnd),
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
		"exec_id:" + fmt.Sprintf("%g", event.ExperimentExecution.ExecutionId),
		"exp_key:" + event.ExperimentExecution.ExperimentKey,
		"exp_name:" + truncate.Truncate(event.ExperimentExecution.Name, 20, "...", truncate.PositionEnd),
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
		tags = append(tags, "step_name:"+truncate.Truncate(*step.ActionName, 20, "...", truncate.PositionEnd))
	}
	if step.CustomLabel != nil {
		tags = append(tags, "step_label:"+truncate.Truncate(*step.CustomLabel, 20, "...", truncate.PositionEnd))
	}
	tags = append(tags, fmt.Sprintf("step_exec_id:%.0f", step.ExecutionId))
	tags = append(tags, "step_exp_key:"+step.ExperimentKey)
	tags = append(tags, fmt.Sprintf("step_id:%s", step.Id))

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
	log.Debug().Msgf("Sending annotation: %v", annotation)
	if annotation.NeedPatch {
		handlePatchAnnotation(ctx, client, annotation)
	} else {
		handlePostAnnotation(ctx, client, annotation)
	}
}

func handlePatchAnnotation(ctx context.Context, client *resty.Client, annotation *AnnotationBody) {
	annotationsFound, resp, err := findAnnotations(ctx, client, annotation)
	tagsSearched := selectTagsForSearch(annotation.Tags)
	if err != nil {
		log.Err(err).Msgf("Error found when finding annotation with these tags %s. Full response: %v", tagsSearched, resp.String())
		return
	}

	switch len(annotationsFound) {
	case 1:
		annotation.ID = strconv.Itoa(annotationsFound[0].ID)
		patchAnnotation(ctx, client, annotation)
	case 0:
		log.Warn().Msgf("Failed to find annotation with tags %s.", tagsSearched)
	default:
		log.Warn().Msgf("Found multiple annotations with tags %s. Full response: %v", tagsSearched, resp.String())
	}
}

func findAnnotations(ctx context.Context, client *resty.Client, annotation *AnnotationBody) ([]Annotation, *resty.Response, error) {
	var annotationsFound []Annotation
	resp, err := client.R().
		SetContext(ctx).
		SetResult(&annotationsFound).
		AddRetryCondition(
			func(r *resty.Response, err error) bool {
				return len(*r.Result().(*[]Annotation)) == 0
			},
		).
		SetQueryParamsFromValues(url.Values{
			"tags":  selectTagsForSearch(annotation.Tags),
			"limit": {"10"},
		}).
		Get("/api/annotations")

	//log.Debug().Msg(url.Values{
	//	"tags":  selectTagsForSearch(annotation.Tags),
	//	"limit": {"10"},
	//}.Encode())

	if err != nil {
		return nil, resp, err
	}

	return annotationsFound, resp, nil
}

func patchAnnotation(ctx context.Context, client *resty.Client, annotation *AnnotationBody) {
	var annotationResponse AnnotationResponse
	res, err := client.R().
		SetContext(ctx).
		SetResult(&annotationResponse).
		SetBody(fmt.Sprintf(`{"timeEnd": %d}`, annotation.TimeEnd)).
		Patch("/api/annotations/" + annotation.ID)

	if err != nil {
		log.Err(err).Msgf("Failed to patch annotation ID %s. Full response: %v", annotation.ID, res.String())
		return
	}

	if !res.IsSuccess() {
		log.Err(err).Msgf("Grafana API responded with unexpected status code %d while patching annotations. Full response: %v", res.StatusCode(), res.String())
	} else {
		log.Debug().Msgf("Successfully patched annotation %s", annotation.ID)
	}
}

func handlePostAnnotation(ctx context.Context, client *resty.Client, annotation *AnnotationBody) {
	annotationBytes, err := json.Marshal(annotation)
	if err != nil {
		log.Err(err).Msgf("Failed to marshal annotation %v. Full response: %v", annotation, err)
		return
	}

	var annotationResponse AnnotationResponse
	res, err := client.R().
		SetContext(ctx).
		SetResult(&annotationResponse).
		SetBody(annotationBytes).
		Post("/api/annotations")

	if err != nil {
		log.Err(err).Msgf("Failed to post annotation, body: %v. Full response: %v", annotationBytes, res.String())
		return
	}

	if !res.IsSuccess() {
		log.Err(err).Msgf("Grafana API responded with unexpected status code %d while posting annotations. Full response: %v", res.StatusCode(), res.String())
	}
}

func selectTagsForSearch(tags []string) []string {
	searchTags := make([]string, 0)
	for _, v := range tags {
		if strings.Contains(v, "exec_id") {
			searchTags = append(searchTags, v)
		}
		if strings.Contains(v, "exp_key") {
			searchTags = append(searchTags, v)
		}
		if strings.Contains(v, "step_exp_key") {
			searchTags = append(searchTags, v)
			searchTags = append(searchTags, "event:experiment.execution.step-started")
		}
		if strings.Contains(v, "step_id") {
			searchTags = append(searchTags, v)
		}
	}
	if !slices.Contains(searchTags, "event:experiment.execution.step-started") {
		searchTags = append(searchTags, "event:experiment.execution.created")
	}

	return removeDuplicates(searchTags)
}
