package extannotations

import (
	"context"
	"encoding/json"
	"github.com/steadybit/extension-kit/extutil"

	"github.com/steadybit/event-kit/go/event_kit_api"
	"strconv"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

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
		Tags:     []string{"event_name:experiment.execution.completed", "execution_id:73983", "experiment_key:ADM-891", "experiment_name:test extension-grafana", "source:Steadybit"},
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
	expectedQuery := "limit=10&tags=execution_id%3A73983&tags=experiment_key%3AADM-891&tags=event_name%3Aexperiment.execution.created"
	httpmock.RegisterResponderWithQuery("GET", "/api/annotations", expectedQuery,
		httpmock.NewJsonResponderOrPanic(200, []Annotation{*testAnnotation}))

	// Call the function
	annotations, err := findAnnotations(ctx, client, annotation)

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
				ActionId:      extutil.Ptr("test"),
				ActionKind:    nil,
				ActionName:    extutil.Ptr("test"),
				CustomLabel:   extutil.Ptr("test"),
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
		assert.Contains(t, annotation.Tags, "event_name:test")
		assert.Contains(t, annotation.Tags, "tenant_key:Test")
		assert.Contains(t, annotation.Tags, "tenant_name:test")
		assert.Contains(t, annotation.Tags, "experiment_key:exp123")
		assert.Contains(t, annotation.Tags, "experiment_name:test")
		assert.Contains(t, annotation.Tags, "step_action_name:test")
		assert.Contains(t, annotation.Tags, "step_custom_label:test")
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
				ActionId:      extutil.Ptr("test"),
				ActionKind:    nil,
				ActionName:    extutil.Ptr("test"),
				CustomLabel:   extutil.Ptr("test"),
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
		assert.Contains(t, annotation.Tags, "event_name:test")
		assert.Contains(t, annotation.Tags, "tenant_key:Test")
		assert.Contains(t, annotation.Tags, "tenant_name:test")
		assert.Contains(t, annotation.Tags, "experiment_key:exp123")
		assert.Contains(t, annotation.Tags, "experiment_name:test")
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
				ActionId:      extutil.Ptr("test"),
				ActionKind:    nil,
				ActionName:    extutil.Ptr("test"),
				CustomLabel:   extutil.Ptr("test"),
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
				ActionId:      extutil.Ptr("test"),
				ActionKind:    nil,
				ActionName:    extutil.Ptr("test"),
				CustomLabel:   extutil.Ptr("test"),
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
