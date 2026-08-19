package extannotations

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/steadybit/event-kit/go/event_kit_api"
	"strconv"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleDoesNotBlockOnGrafana verifies that the event listener answers without waiting for the
// Grafana API. Blocking here made the extension exceed the agent's Request-Timeout, which
// extension-kit answers with 503 "Timeout".
func TestHandleDoesNotBlockOnGrafana(t *testing.T) {
	worker := newAnnotationWorker(annotationQueueSize)

	blocked := make(chan struct{})
	RestyClient = resty.New()
	httpmock.ActivateNonDefault(RestyClient.GetClient())
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET", "/api/annotations",
		func(*http.Request) (*http.Response, error) {
			close(blocked)
			<-make(chan struct{}) // never returns
			return nil, nil
		})

	startedTime := time.Now().Add(-time.Minute)
	endedTime := time.Now()
	body, err := json.Marshal(event_kit_api.EventRequestBody{
		EventName:   "experiment.execution.step-completed",
		Id:          uuid.New(),
		Environment: &event_kit_api.Environment{Id: "test", Name: "gateway"},
		Tenant:      event_kit_api.Tenant{Key: "key", Name: "name"},
		ExperimentStepExecution: &event_kit_api.ExperimentStepExecution{
			ExecutionId:   42,
			ExperimentKey: "ExperimentKey",
			Id:            uuid.New(),
			Type:          event_kit_api.Action,
			ActionId:      extutil.Ptr("some_action_id"),
			StartedTime:   &startedTime,
			EndedTime:     &endedTime,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/events/experiment-step-completed", bytes.NewReader(body))

	done := make(chan struct{})
	go func() {
		defer close(done)
		handle(worker, onExperimentStepCompleted)(recorder, request, body)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return - it is still waiting for the Grafana API")
	}
	require.Equal(t, 200, recorder.Code)

	// The work is queued, not skipped: draining it hits Grafana.
	worker.start(t.Context())
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("queued annotation was never sent to Grafana")
	}
}

// TestEnqueueDropsWhenQueueIsFull makes sure a slow Grafana API cannot make the event listener
// block once the queue is saturated. The worker is deliberately not started, so nothing drains the
// queue while the test inspects it.
func TestEnqueueDropsWhenQueueIsFull(t *testing.T) {
	worker := newAnnotationWorker(4)

	for range 4 {
		worker.enqueue(&AnnotationBody{})
	}
	require.Len(t, worker.queue, 4)

	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.enqueue(&AnnotationBody{Tags: []string{"dropped"}})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("enqueue blocked on a full queue")
	}
	require.Len(t, worker.queue, 4)
}

// TestDrainWaitsForQueuedAnnotations covers the shutdown path: a rolling restart must not discard
// annotations that are still queued.
func TestDrainWaitsForQueuedAnnotations(t *testing.T) {
	worker := newAnnotationWorker(annotationQueueSize)

	RestyClient = resty.New()
	httpmock.ActivateNonDefault(RestyClient.GetClient())
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("POST", "/api/annotations",
		func(*http.Request) (*http.Response, error) {
			time.Sleep(20 * time.Millisecond)
			return httpmock.NewStringResponse(200, `{"id":1}`), nil
		})

	for range 5 {
		worker.enqueue(&AnnotationBody{Tags: []string{"tag1"}, NeedPatch: false})
	}
	worker.start(t.Context())

	worker.drain(5 * time.Second)

	require.Empty(t, worker.queue)
	require.Equal(t, 5, httpmock.GetCallCountInfo()["POST /api/annotations"])
}

// TestDrainWaitsForAnInFlightAnnotation covers the interleaving where the queue is already empty
// but the Grafana call for the last annotation is still running. A drain that only looked at the
// queue length would return here and let the shutdown abandon that annotation.
func TestDrainWaitsForAnInFlightAnnotation(t *testing.T) {
	worker := newAnnotationWorker(annotationQueueSize)

	RestyClient = resty.New()
	httpmock.ActivateNonDefault(RestyClient.GetClient())
	defer httpmock.DeactivateAndReset()

	inFlight := make(chan struct{})
	release := make(chan struct{})
	httpmock.RegisterResponder("POST", "/api/annotations",
		func(*http.Request) (*http.Response, error) {
			close(inFlight)
			<-release
			return httpmock.NewStringResponse(200, `{"id":1}`), nil
		})

	worker.enqueue(&AnnotationBody{Tags: []string{"tag1"}})
	worker.start(t.Context())

	<-inFlight
	require.Empty(t, worker.queue, "the queue must be empty for this test to exercise the race")

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		worker.drain(5 * time.Second)
	}()

	select {
	case <-drained:
		t.Fatal("drain returned while an annotation was still in flight")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("drain did not return after the in-flight annotation completed")
	}
	require.Equal(t, 1, httpmock.GetCallCountInfo()["POST /api/annotations"])
}

// TestDrainGivesUpAfterTimeout makes sure an unresponsive Grafana API cannot block a shutdown.
func TestDrainGivesUpAfterTimeout(t *testing.T) {
	worker := newAnnotationWorker(annotationQueueSize)
	worker.enqueue(&AnnotationBody{Tags: []string{"never sent"}})

	// No worker is started, so nothing drains the queue.
	start := time.Now()
	worker.drain(200 * time.Millisecond)

	require.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond)
	require.Equal(t, int64(1), worker.pending.Load())
}

// TestSendAnnotations tests the sendAnnotations function
func TestSendAnnotations(t *testing.T) {
	client := resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	ctx := context.TODO()
	annotation := &AnnotationBody{
		NeedPatch: true,
		Tags:      []string{"tag1", "tag2"},
		Time:      time.Date(2024, 7, 18, 8, 0, 0, 0, time.UTC).UnixMilli(),
		TimeEnd:   time.Date(2024, 7, 18, 9, 0, 0, 0, time.UTC).UnixMilli(),
	}

	// Mock the findAnnotations function call
	httpmock.RegisterResponder("GET", "/api/annotations",
		httpmock.NewStringResponder(200, `[]`))

	// Call the function
	sendAnnotations(ctx, client, annotation)

	// Assertions
	assert.True(t, httpmock.GetCallCountInfo()["GET /api/annotations"] > 0)
}

// TestHandlePatchAnnotation tests the handlePatchAnnotation function
func TestHandlePatchAnnotation(t *testing.T) {
	client := resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	ctx := context.TODO()
	annotation := &AnnotationBody{
		NeedPatch: true,
		Tags:      []string{"tag1", "tag2"},
		Time:      time.Date(2024, 7, 18, 8, 0, 0, 0, time.UTC).UnixMilli(),
		TimeEnd:   time.Date(2024, 7, 18, 9, 0, 0, 0, time.UTC).UnixMilli(),
	}

	// Mock the findAnnotations function call
	httpmock.RegisterResponder("GET", "/api/annotations",
		httpmock.NewStringResponder(200, `[]`))

	// Call the function
	handlePatchAnnotation(ctx, client, annotation)

	// Assertions
	assert.True(t, httpmock.GetCallCountInfo()["GET /api/annotations"] > 0)
}

// TestHandlePostAnnotation tests the handlePostAnnotation function
func TestHandlePostAnnotation(t *testing.T) {
	client := resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	ctx := context.TODO()
	annotation := &AnnotationBody{
		NeedPatch: false,
		Tags:      []string{"tag1", "tag2"},
		Time:      time.Date(2024, 7, 18, 8, 0, 0, 0, time.UTC).UnixMilli(),
		TimeEnd:   time.Date(2024, 7, 18, 9, 0, 0, 0, time.UTC).UnixMilli(),
	}

	_, err := json.Marshal(annotation)
	assert.NoError(t, err)

	// Mock the post request
	httpmock.RegisterResponder("POST", "/api/annotations",
		httpmock.NewStringResponder(200, `{"status":"success"}`))

	// Call the function
	handlePostAnnotation(ctx, client, annotation)

	// Assertions
	assert.True(t, httpmock.GetCallCountInfo()["POST /api/annotations"] > 0)
}

// TestFindAnnotations tests the findAnnotations function
func TestFindAnnotations(t *testing.T) {
	client := resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	testAnnotation := &Annotation{
		Tags:     []string{"event:experiment.execution.completed", "exec_id:73983", "exp_key:ADM-891", "exp_name:test extension-grafana", "source:Steadybit"},
		Time:     time.Date(2024, 7, 18, 8, 0, 0, 0, time.UTC).UnixMilli(),
		TimeEnd:  time.Date(2024, 7, 18, 9, 0, 0, 0, time.UTC).UnixMilli(),
		ID:       1,
		Text:     "test",
		NewState: "inactive",
	}

	ctx := context.TODO()
	annotation := &AnnotationBody{
		Tags:    testAnnotation.Tags,
		Time:    testAnnotation.Time,
		TimeEnd: testAnnotation.TimeEnd,
	}

	// Mock the get request
	expectedQuery := "limit=10&tags=exec_id%3A73983&tags=exp_key%3AADM-891&tags=event%3Aexperiment.execution.created"
	httpmock.RegisterResponderWithQuery("GET", "/api/annotations", expectedQuery,
		httpmock.NewJsonResponderOrPanic(200, []Annotation{*testAnnotation}))

	// Call the function
	annotations, _, err := findAnnotations(ctx, client, annotation)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, annotations)
	assert.Equal(t, 1, len(annotations))
}

// TestPatchAnnotation tests the patchAnnotation function
func TestPatchAnnotation(t *testing.T) {
	client := resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	ctx := context.TODO()

	testAnnotation := &Annotation{
		Tags:     []string{"tag1", "tag2"},
		Time:     time.Date(2024, 7, 18, 8, 0, 0, 0, time.UTC).UnixMilli(),
		TimeEnd:  time.Date(2024, 7, 18, 9, 0, 0, 0, time.UTC).UnixMilli(),
		ID:       1,
		Text:     "test",
		NewState: "inactive",
	}

	annotation := &AnnotationBody{
		TimeEnd: testAnnotation.TimeEnd,
		ID:      strconv.Itoa(testAnnotation.ID),
	}

	// Mock the put request
	httpmock.RegisterResponder("PATCH", "/api/annotations/1",
		httpmock.NewStringResponder(200, `{"status":"success"}`))

	// Call the function
	patchAnnotation(ctx, client, annotation)

	// Assertions
	assert.True(t, httpmock.GetCallCountInfo()["PATCH /api/annotations/1"] > 0)
}

func TestOnExperimentStepStarted(t *testing.T) {
	startTime := time.Now()
	t.Run("Success", func(t *testing.T) {
		event := event_kit_api.EventRequestBody{
			Environment: &event_kit_api.Environment{
				Id:   "test",
				Name: "test",
			},
			EventName: "test",
			EventTime: time.Time{},
			ExperimentExecution: &event_kit_api.ExperimentExecution{
				EndedTime:     nil,
				ExecutionId:   0,
				ExperimentKey: "exp123",
				Hypothesis:    "test",
				Name:          "test",
				PreparedTime:  startTime,
				Reason:        nil,
				ReasonDetails: nil,
				StartedTime:   time.Now(),
				State:         "test",
			},
			ExperimentStepExecution: &event_kit_api.ExperimentStepExecution{
				ActionId:      new("test"),
				ActionKind:    nil,
				ActionName:    new("test"),
				CustomLabel:   new("test"),
				EndedTime:     nil,
				ExecutionId:   0,
				ExperimentKey: "test",
				Id:            uuid.New(),
				StartedTime:   &startTime,
				State:         "created",
				Type:          "",
			},
			ExperimentStepTargetExecution: nil,
			Id:                            uuid.New(),
			Principal:                     nil,
			Team: &event_kit_api.Team{
				Id:   "test",
				Key:  "test",
				Name: "test",
			},
			Tenant: event_kit_api.Tenant{
				Key:  "Test",
				Name: "test",
			},
		}

		annotation, err := onExperimentStepStarted(event)
		assert.NoError(t, err)
		assert.NotNil(t, annotation)

		assert.Contains(t, annotation.Tags, "team_name:test")
		assert.Contains(t, annotation.Tags, "source:Steadybit")
		assert.Contains(t, annotation.Tags, "event:test")
		assert.Contains(t, annotation.Tags, "tenant_key:Test")
		assert.Contains(t, annotation.Tags, "tenant:test")
		assert.Contains(t, annotation.Tags, "exp_key:exp123")
		assert.Contains(t, annotation.Tags, "exp_name:test")
		assert.Contains(t, annotation.Tags, "step_name:test")
		assert.Contains(t, annotation.Tags, "step_label:test")
		assert.Equal(t, event.ExperimentStepExecution.StartedTime.UnixMilli(), annotation.Time)
		assert.False(t, annotation.NeedPatch)
	})
}

func TestOnExperimentStarted(t *testing.T) {
	startTime := time.Now()
	t.Run("Success", func(t *testing.T) {
		event := event_kit_api.EventRequestBody{
			Environment: &event_kit_api.Environment{
				Id:   "test",
				Name: "test",
			},
			EventName: "test",
			EventTime: time.Time{},
			ExperimentExecution: &event_kit_api.ExperimentExecution{
				EndedTime:     nil,
				ExecutionId:   0,
				ExperimentKey: "exp123",
				Hypothesis:    "test",
				Name:          "test",
				PreparedTime:  startTime,
				Reason:        nil,
				ReasonDetails: nil,
				StartedTime:   time.Now(),
				State:         "test",
			},
			ExperimentStepExecution: &event_kit_api.ExperimentStepExecution{
				ActionId:      new("test"),
				ActionKind:    nil,
				ActionName:    new("test"),
				CustomLabel:   new("test"),
				EndedTime:     nil,
				ExecutionId:   0,
				ExperimentKey: "test",
				Id:            uuid.New(),
				StartedTime:   &startTime,
				State:         "created",
				Type:          "",
			},
			ExperimentStepTargetExecution: nil,
			Id:                            uuid.New(),
			Principal:                     nil,
			Team: &event_kit_api.Team{
				Id:   "test",
				Key:  "test",
				Name: "test",
			},
			Tenant: event_kit_api.Tenant{
				Key:  "Test",
				Name: "test",
			},
		}

		annotation, err := onExperimentStarted(event)
		assert.NoError(t, err)
		assert.NotNil(t, annotation)

		assert.Contains(t, annotation.Tags, "team_name:test")
		assert.Contains(t, annotation.Tags, "source:Steadybit")
		assert.Contains(t, annotation.Tags, "event:test")
		assert.Contains(t, annotation.Tags, "tenant_key:Test")
		assert.Contains(t, annotation.Tags, "tenant:test")
		assert.Contains(t, annotation.Tags, "exp_key:exp123")
		assert.Contains(t, annotation.Tags, "exp_name:test")
		assert.Equal(t, event.ExperimentExecution.StartedTime.UnixMilli(), annotation.Time)
		assert.False(t, annotation.NeedPatch)
	})
}

func TestOnExperimentCompleted(t *testing.T) {
	startTime := time.Now()
	endTime := startTime
	t.Run("Success", func(t *testing.T) {
		event := event_kit_api.EventRequestBody{
			Environment: &event_kit_api.Environment{
				Id:   "test",
				Name: "test",
			},
			EventName: "test",
			EventTime: time.Time{},
			ExperimentExecution: &event_kit_api.ExperimentExecution{
				EndedTime:     &endTime,
				ExecutionId:   0,
				ExperimentKey: "exp123",
				Hypothesis:    "test",
				Name:          "test",
				PreparedTime:  startTime,
				Reason:        nil,
				ReasonDetails: nil,
				StartedTime:   startTime,
				State:         "test",
			},
			ExperimentStepExecution: &event_kit_api.ExperimentStepExecution{
				ActionId:      new("test"),
				ActionKind:    nil,
				ActionName:    new("test"),
				CustomLabel:   new("test"),
				EndedTime:     nil,
				ExecutionId:   0,
				ExperimentKey: "test",
				Id:            uuid.New(),
				StartedTime:   &startTime,
				State:         "created",
				Type:          "",
			},
			ExperimentStepTargetExecution: nil,
			Id:                            uuid.New(),
			Principal:                     nil,
			Team: &event_kit_api.Team{
				Id:   "test",
				Key:  "test",
				Name: "test",
			},
			Tenant: event_kit_api.Tenant{
				Key:  "Test",
				Name: "test",
			},
		}

		annotation, err := onExperimentCompleted(event)
		assert.NoError(t, err)
		assert.NotNil(t, annotation)

		assert.Equal(t, event.ExperimentExecution.StartedTime.UnixMilli(), annotation.Time)
		assert.Equal(t, event.ExperimentExecution.EndedTime.UnixMilli(), annotation.TimeEnd)
		assert.True(t, annotation.NeedPatch)
	})
}

func TestOnExperimentStepCompleted(t *testing.T) {
	startTime := time.Now()
	endTime := startTime
	t.Run("Success", func(t *testing.T) {
		event := event_kit_api.EventRequestBody{
			Environment: &event_kit_api.Environment{
				Id:   "test",
				Name: "test",
			},
			EventName: "test",
			EventTime: time.Time{},
			ExperimentExecution: &event_kit_api.ExperimentExecution{
				EndedTime:     &endTime,
				ExecutionId:   0,
				ExperimentKey: "exp123",
				Hypothesis:    "test",
				Name:          "test",
				PreparedTime:  startTime,
				Reason:        nil,
				ReasonDetails: nil,
				StartedTime:   startTime,
				State:         "test",
			},
			ExperimentStepExecution: &event_kit_api.ExperimentStepExecution{
				ActionId:      new("test"),
				ActionKind:    nil,
				ActionName:    new("test"),
				CustomLabel:   new("test"),
				EndedTime:     &endTime,
				ExecutionId:   0,
				ExperimentKey: "test",
				Id:            uuid.New(),
				StartedTime:   &startTime,
				State:         "created",
				Type:          "",
			},
			ExperimentStepTargetExecution: nil,
			Id:                            uuid.New(),
			Principal:                     nil,
			Team: &event_kit_api.Team{
				Id:   "test",
				Key:  "test",
				Name: "test",
			},
			Tenant: event_kit_api.Tenant{
				Key:  "Test",
				Name: "test",
			},
		}

		annotation, err := onExperimentStepCompleted(event)
		assert.NoError(t, err)
		assert.NotNil(t, annotation)

		assert.Equal(t, event.ExperimentExecution.StartedTime.UnixMilli(), annotation.Time)
		assert.Equal(t, event.ExperimentStepExecution.EndedTime.UnixMilli(), annotation.TimeEnd)
		assert.True(t, annotation.NeedPatch)
	})
}
