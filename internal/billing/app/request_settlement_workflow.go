package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/api/serviceerror"
	temporalclient "go.temporal.io/sdk/client"
)

type RequestSettlementWorkflowParams struct {
	RequestID       string
	TraceID         string
	UserID          int
	TokenID         int
	AccountID       string
	ReservationID   string
	SettlementID    string
	UsageEvidenceID string
	ReservedAmount  int64
	ActualAmount    int64
}

var (
	requestSettlementTemporalClient     temporalclient.Client
	requestSettlementTemporalClientErr  error
	requestSettlementTemporalClientOnce sync.Once
)

// StartRequestSettlementWorkflow schedules the durable post-relay settlement projection.
// The synchronous relay has already executed and settled the reservation before this runs.
func StartRequestSettlementWorkflow(ctx context.Context, params RequestSettlementWorkflowParams) error {
	if params.RequestID == "" || params.AccountID == "" || params.ReservationID == "" || params.SettlementID == "" {
		return fmt.Errorf("request settlement workflow requires request, account, reservation and settlement ids")
	}
	client, err := requestSettlementTemporalClientForRelay()
	if err != nil {
		return err
	}
	_, err = client.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:                       "request-settlement-" + params.RequestID,
		TaskQueue:                platformconfig.GetEnvOrDefaultString("TEMPORAL_TASK_QUEUE_BILLING", "workflow-billing"),
		WorkflowExecutionTimeout: time.Hour,
		WorkflowRunTimeout:       time.Hour,
		WorkflowTaskTimeout:      15 * time.Second,
	}, contracts.WorkflowRequestSettlement, contracts.RequestSettlementWorkflowInput{
		WorkflowVersion: "v1",
		RequestID:       params.RequestID,
		TraceID:         params.TraceID,
		UserID:          params.UserID,
		TokenID:         params.TokenID,
		AccountID:       params.AccountID,
		ReservationID:   params.ReservationID,
		SettlementID:    params.SettlementID,
		UsageEvidenceID: params.UsageEvidenceID,
		ReservedAmount:  params.ReservedAmount,
		ActualAmount:    params.ActualAmount,
	})
	if _, alreadyStarted := err.(*serviceerror.WorkflowExecutionAlreadyStarted); alreadyStarted {
		return nil
	}
	return err
}

func requestSettlementTemporalClientForRelay() (temporalclient.Client, error) {
	requestSettlementTemporalClientOnce.Do(func() {
		hostPort := platformconfig.GetEnvOrDefaultString("TEMPORAL_HOSTPORT", "")
		if hostPort == "" {
			requestSettlementTemporalClientErr = fmt.Errorf("TEMPORAL_HOSTPORT is required for request settlement workflow orchestration")
			return
		}
		requestSettlementTemporalClient, requestSettlementTemporalClientErr = temporalclient.Dial(temporalclient.Options{
			HostPort:  hostPort,
			Namespace: platformconfig.GetEnvOrDefaultString("TEMPORAL_NAMESPACE", "default"),
		})
	})
	return requestSettlementTemporalClient, requestSettlementTemporalClientErr
}
