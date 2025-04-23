package workflows

import (
	"context"
	"fmt"
	"time"

	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func (workflows *DataProcessingWorkflows) CreateIfNotExistsXSyncSchedule(temporalClient client.Client) error {
	scheduleID := "x-sync-schedule"

	// Check if schedule already exists
	handle := temporalClient.ScheduleClient().GetHandle(context.Background(), scheduleID)
	_, err := handle.Describe(context.Background())
	if err == nil {
		return nil
	}

	// Only create if schedule doesn't exist
	scheduleSpec := client.ScheduleSpec{
		Intervals: []client.ScheduleIntervalSpec{
			{
				Every: 1 * time.Minute,
			},
		},
	}

	scheduleAction := &client.ScheduleWorkflowAction{
		Workflow:  "XSyncWorkflow",
		TaskQueue: "default",
		Args:      []interface{}{XSyncWorkflowInput{}},
	}

	_, err = temporalClient.ScheduleClient().Create(context.Background(), client.ScheduleOptions{
		ID:     scheduleID,
		Spec:   scheduleSpec,
		Action: scheduleAction,
	})
	return err
}

type XSyncWorkflowInput struct{}

type XSyncWorkflowResponse struct {
	EndTime             time.Time `json:"endTime"`
	Success             bool      `json:"success"`
	LastRecordID        string    `json:"lastRecordID"`
	LastRecordTimestamp time.Time `json:"lastRecordTimestamp"`
}

func (w *DataProcessingWorkflows) XSyncWorkflow(ctx workflow.Context, input XSyncWorkflowInput) (XSyncWorkflowResponse, error) {
	if w.Store == nil {
		return XSyncWorkflowResponse{}, errors.New("store is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    1,
		},
	})

	var previousResult XSyncWorkflowResponse

	if workflow.HasLastCompletionResult(ctx) {
		err := workflow.GetLastCompletionResult(ctx, &previousResult)
		if err != nil {
			return XSyncWorkflowResponse{}, err
		}
		workflow.GetLogger(ctx).Info("Recovered last result", "value", previousResult)
	}

	workflowResponse := XSyncWorkflowResponse{
		LastRecordID:        previousResult.LastRecordID,
		LastRecordTimestamp: previousResult.LastRecordTimestamp,
	}

	var response XFetchActivityResponse
	err := workflow.ExecuteActivity(ctx, w.XFetchActivity, XFetchActivityInput{}).Get(ctx, &response)
	if err != nil {
		return workflowResponse, err
	}

	fmt.Println("response", response)

	filteredRecords := []types.Record{}
	for _, record := range response.Records {
		if previousResult.LastRecordTimestamp.IsZero() {
			filteredRecords = append(filteredRecords, record)
			continue
		}

		if record.Timestamp.After(previousResult.LastRecordTimestamp) {
			filteredRecords = append(filteredRecords, record)
		}
	}

	if len(filteredRecords) == 0 {
		workflowResponse.EndTime = time.Now()
		return XSyncWorkflowResponse{
			EndTime:             time.Now(),
			Success:             true,
			LastRecordID:        previousResult.LastRecordID,
			LastRecordTimestamp: previousResult.LastRecordTimestamp,
		}, nil
	}

	w.Logger.Info("filteredRecords", "value", filteredRecords)
	err = workflow.ExecuteActivity(ctx, w.XIndexActivity, XIndexActivityInput{Records: filteredRecords}).Get(ctx, nil)
	if err != nil {
		return XSyncWorkflowResponse{}, err
	}

	lastRecord := response.Records[0]

	w.Logger.Info("lastRecord", "value", lastRecord)

	lastRecordID := ""
	if id, ok := lastRecord.Data["id"]; ok && id != nil {
		lastRecordID = id.(string)
	}
	workflowResponse.LastRecordID = lastRecordID
	workflowResponse.LastRecordTimestamp = lastRecord.Timestamp
	workflowResponse.Success = true
	workflowResponse.EndTime = time.Now()

	return workflowResponse, nil
}

type XFetchActivityInput struct {
	Username string `json:"username"`
}

type XFetchActivityResponse struct {
	Records []types.Record `json:"records"`
}

func (w *DataProcessingWorkflows) XFetchActivity(ctx context.Context, input XFetchActivityInput) (XFetchActivityResponse, error) {
	records, err := dataprocessing.Sync("x", w.Store)
	if err != nil {
		return XFetchActivityResponse{}, err
	}
	return XFetchActivityResponse{Records: records}, nil
}

type XIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type XIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) XIndexActivity(ctx context.Context, input XIndexActivityInput) (XIndexActivityResponse, error) {
	documents, err := x.ToDocuments(input.Records)
	if err != nil {
		return XIndexActivityResponse{}, err
	}
	w.Logger.Info("X", "tweets", len(documents))
	err = w.Memory.Store(ctx, documents)
	if err != nil {
		return XIndexActivityResponse{}, err
	}

	return XIndexActivityResponse{}, nil
}
