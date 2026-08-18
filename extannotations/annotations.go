// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extannotations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aquilax/truncate"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/event-kit/go/event_kit_api"
	"github.com/steadybit/extension-grafana/config"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extsignals"
)

const (
	// annotationQueueSize bounds how many annotation updates may wait for the Grafana API.
	annotationQueueSize = 256
	// annotationTimeout bounds the Grafana interaction for a single event, i.e. a find plus a patch
	// and whatever retries fit into it. Attempts that no longer fit are cut short by the deadline.
	annotationTimeout = 30 * time.Second
	// annotationDrainTimeout bounds how long a shutdown waits for queued annotations to be sent.
	annotationDrainTimeout = 10 * time.Second
)

var defaultAnnotationWorker = newAnnotationWorker(annotationQueueSize)

// annotationWorker decouples the event listeners from the Grafana API. The Grafana API must not be
// called on the request goroutine: an event listener that blocks on a third-party API exceeds the
// agent's Request-Timeout, extension-kit then answers 503 "Timeout", and the platform reports the
// trigger event listener as failed.
type annotationWorker struct {
	queue chan *AnnotationBody
	// pending counts the annotations that are queued *or* currently being sent. Queue length alone
	// is not enough: it drops to zero the moment the worker picks up the last annotation, while the
	// Grafana call is still in flight and must not be abandoned by a shutdown.
	pending atomic.Int64
	// sent receives a signal after every completed send, so a drain can wait for pending to reach 0
	// instead of polling.
	sent chan struct{}
}

func newAnnotationWorker(queueSize int) *annotationWorker {
	return &annotationWorker{
		queue: make(chan *AnnotationBody, queueSize),
		sent:  make(chan struct{}, 1),
	}
}

func RegisterEventListenerHandlers() {
	if !config.Config.SendAnnotations {
		log.Info().Msg("Annotations are disabled. Skipping event listener registration.")
		return
	}
	defaultAnnotationWorker.start(context.Background())
	// Annotations are sent in the background, so without this a rolling restart would silently
	// discard everything that is still queued. Run before the HTTP servers go down, so the queue
	// can still be drained.
	extsignals.AddSignalHandler(extsignals.SignalHandler{
		Name:  "AnnotationWorkerDrain",
		Order: extsignals.OrderStopCustom,
		Handler: func(os.Signal) {
			defaultAnnotationWorker.drain(annotationDrainTimeout)
		},
	})
	exthttp.RegisterHttpHandler("/events/experiment-started", handle(defaultAnnotationWorker, onExperimentStarted))
	exthttp.RegisterHttpHandler("/events/experiment-completed", handle(defaultAnnotationWorker, onExperimentCompleted))
	exthttp.RegisterHttpHandler("/events/experiment-step-started", handle(defaultAnnotationWorker, onExperimentStepStarted))
	exthttp.RegisterHttpHandler("/events/experiment-step-completed", handle(defaultAnnotationWorker, onExperimentStepCompleted))
}

// start processes queued annotations until ctx is canceled. Exactly one worker keeps the order in
// which the platform delivered the events - a step's "started" annotation has to be created before
// the "completed" event can patch it - and bounds the load put on the Grafana API.
func (w *annotationWorker) start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case annotation := <-w.queue:
				w.send(ctx, annotation)
				w.pending.Add(-1)
				w.signalSent()
			}
		}
	}()
}

// signalSent reports that one annotation finished, without blocking when nobody is draining.
func (w *annotationWorker) signalSent() {
	select {
	case w.sent <- struct{}{}:
	default:
	}
}

// drain waits until everything that is queued or in flight has been sent, giving up after timeout.
// It is best effort: annotations still pending when the timeout hits are lost, which beats blocking
// a shutdown indefinitely on an unresponsive Grafana API.
func (w *annotationWorker) drain(timeout time.Duration) {
	if w.pending.Load() == 0 {
		return
	}
	log.Info().Msgf("Waiting up to %s for %d pending annotation(s) to be sent.", timeout, w.pending.Load())

	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			log.Warn().Msgf("Timed out draining annotations, %d were not sent.", w.pending.Load())
			return
		case <-w.sent:
			if w.pending.Load() == 0 {
				return
			}
		}
	}
}

func (w *annotationWorker) send(ctx context.Context, annotation *AnnotationBody) {
	ctx, cancel := context.WithTimeout(ctx, annotationTimeout)
	defer cancel()
	sendAnnotations(ctx, RestyClient, annotation)
}

// enqueue hands the annotation over to the worker, dropping it when the queue is saturated. It
// never blocks: the caller is an event listener that has to answer the agent right away.
func (w *annotationWorker) enqueue(annotation *AnnotationBody) {
	// Count it before handing it over, so the worker can never complete and decrement before the
	// increment is visible - which would let a concurrent drain see a pending count of 0 or below.
	w.pending.Add(1)
	select {
	case w.queue <- annotation:
	default:
		w.pending.Add(-1)
		log.Warn().Msgf("Annotation queue is full (%d entries), dropping annotation with tags %v. The Grafana API is likely too slow to keep up.", cap(w.queue), annotation.Tags)
	}
}

type eventHandler func(event event_kit_api.EventRequestBody) (*AnnotationBody, error)

func handle(worker *annotationWorker, handler eventHandler) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {

		event, err := parseBodyToEventRequestBody(body)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to decode event request body", err))
			return
		}

		if request, err := handler(event); err == nil {
			if request != nil {
				worker.enqueue(request)
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
		Text:      fmt.Sprintf("Experiment %s", event.ExperimentExecution.ExperimentKey),
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
		Text:      fmt.Sprintf("Step %s", getActionName(*event.ExperimentStepExecution)),
		Time:      startTime,
		NeedPatch: false,
	}, nil
}

func onExperimentCompleted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	if event.ExperimentExecution == nil {
		return nil, errors.New("missing ExperimentExecution in event")
	}

	log.Debug().Msg("onExperimentCompleted, tagging:")
	tags := getEventBaseTags(event)
	log.Debug().Msgf("getEventBaseTags: %v", tags)
	tags = append(tags, getExecutionTags(event)...)
	log.Debug().Msgf("getExecutionTags: %v", tags)
	tags = removeDuplicates(tags)
	log.Debug().Msgf("removeDuplicates: %v", tags)

	var endTime int64
	if event.ExperimentExecution.EndedTime != nil {
		endTime = event.ExperimentExecution.EndedTime.UnixMilli()
	}

	return &AnnotationBody{Tags: tags, Time: event.ExperimentExecution.StartedTime.UnixMilli(), TimeEnd: endTime, NeedPatch: true}, nil
}

func onExperimentStepCompleted(event event_kit_api.EventRequestBody) (*AnnotationBody, error) {
	if event.ExperimentStepExecution == nil {
		return nil, errors.New("missing ExperimentStepExecution in event")
	}

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
	if event.ExperimentStepExecution.StartedTime != nil {
		startTime = event.ExperimentStepExecution.StartedTime.UnixMilli()
	}
	if event.ExperimentStepExecution.EndedTime != nil {
		endTime = event.ExperimentStepExecution.EndedTime.UnixMilli()
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
		fmt.Sprintf("env:%s", truncate.Truncate(event.Environment.Name, 20, "...", truncate.PositionEnd)),
		fmt.Sprintf("event:%s", truncate.Truncate(event.EventName, 50, "...", truncate.PositionEnd)),
		fmt.Sprintf("event_id:%s", event.Id.String()),
		fmt.Sprintf("tenant:%s", truncate.Truncate(event.Tenant.Name, 10, "...", truncate.PositionEnd)),
		fmt.Sprintf("tenant_key:%s", event.Tenant.Key),
	}

	if event.Team != nil {
		tags = append(tags, fmt.Sprintf("team_name:%s", event.Team.Name), fmt.Sprintf("team_key:%s", event.Team.Key))
	}

	return tags
}

func getExecutionTags(event event_kit_api.EventRequestBody) []string {
	if event.ExperimentExecution == nil {
		return []string{}
	}
	tags := []string{
		fmt.Sprintf("exec_id:%g", event.ExperimentExecution.ExecutionId),
		fmt.Sprintf("exp_key:%s", event.ExperimentExecution.ExperimentKey),
		fmt.Sprintf("exp_name:%s", truncate.Truncate(event.ExperimentExecution.Name, 20, "...", truncate.PositionEnd)),
	}

	if event.ExperimentExecution.StartedTime.IsZero() {
		tags = append(tags, fmt.Sprintf("started_time:%s", formatTagTime(time.Now())))
	} else {
		tags = append(tags, fmt.Sprintf("started_time:%s", formatTagTime(event.ExperimentExecution.StartedTime)))
	}

	if event.ExperimentExecution.EndedTime != nil && !(*event.ExperimentExecution.EndedTime).IsZero() {
		tags = append(tags, fmt.Sprintf("ended_time:%s", formatTagTime(*event.ExperimentExecution.EndedTime)))
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

// formatTagTime renders a timestamp for use inside an annotation tag value.
// Grafana treats the ":" in "key:value" tags as a separator and cuts the pill
// at the first colon, so an RFC3339 value like "2026-06-15T19:21:04Z" would be
// displayed as just "2026-06-15T19". We keep the value colon-free so the full
// timestamp stays visible.
func formatTagTime(t time.Time) string {
	return strings.ReplaceAll(t.Format(time.RFC3339), ":", ".")
}

func findTagWithPrefix(tags []string, prefix string) (string, bool) {
	for _, tag := range tags {
		if strings.HasPrefix(tag, prefix) {
			return tag, true
		}
	}
	return "", false
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
		found := annotationsFound[0]
		annotation.ID = strconv.Itoa(found.ID)
		// The PATCH overwrites the annotation's tags, so start from the tags that
		// already exist on the found annotation (e.g. started_time, event:...created)
		// and add the ended_time tag computed for the completion event. This keeps
		// the search discriminator tags intact while finally exposing the end time.
		finalTags := found.Tags
		if tag, ok := findTagWithPrefix(annotation.Tags, "ended_time:"); ok {
			finalTags = removeDuplicates(append(found.Tags, tag))
		}
		annotation.Tags = finalTags
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
	patchBody, err := json.Marshal(map[string]any{
		"timeEnd": annotation.TimeEnd,
		"tags":    annotation.Tags,
	})
	if err != nil {
		log.Err(err).Msgf("Failed to marshal patch body for annotation ID %s.", annotation.ID)
		return
	}

	var annotationResponse AnnotationResponse
	res, err := client.R().
		SetContext(ctx).
		SetResult(&annotationResponse).
		SetBody(patchBody).
		Patch(fmt.Sprintf("/api/annotations/%s", annotation.ID))

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
