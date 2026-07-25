package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/api/serviceerror"
	temporalclient "go.temporal.io/sdk/client"
)

const (
	subscriptionResetScheduleInterval = time.Minute
	subscriptionResetScheduleBatch    = 300
)

var (
	subscriptionResetSchedulerOnce sync.Once
	subscriptionResetClientOnce    sync.Once
	subscriptionResetClient        temporalclient.Client
	subscriptionResetClientErr     error
)

// StartSubscriptionResetScheduler starts Temporal workflows for subscriptions
// whose periodic quota reset is due. The workflow owns the reset side effects.
func StartSubscriptionResetScheduler(ctx context.Context) {
	subscriptionResetSchedulerOnce.Do(func() {
		go func() {
			scheduleDueSubscriptionResets(ctx)
			ticker := time.NewTicker(subscriptionResetScheduleInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					scheduleDueSubscriptionResets(ctx)
				}
			}
		}()
	})
}

func scheduleDueSubscriptionResets(ctx context.Context) {
	client, err := subscriptionResetTemporalClient()
	if err != nil {
		platformobservability.SysLog("subscription reset scheduler unavailable: " + err.Error())
		return
	}

	var subscriptions []commerceschema.UserSubscription
	if err := platformdb.DB.WithContext(ctx).
		Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", platformruntime.GetTimestamp(), "active").
		Order("next_reset_time asc, id asc").
		Limit(subscriptionResetScheduleBatch).
		Find(&subscriptions).Error; err != nil {
		platformobservability.SysLog("load due subscription resets: " + err.Error())
		return
	}

	for _, subscription := range subscriptions {
		startDueSubscriptionResetWorkflow(ctx, client, subscription)
	}
}

func startDueSubscriptionResetWorkflow(ctx context.Context, client temporalclient.Client, subscription commerceschema.UserSubscription) {
	workflowID := subscriptionResetWorkflowID(subscription)
	_, err := client.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                platformconfig.GetEnvOrDefaultString("TEMPORAL_TASK_QUEUE_SUBSCRIPTIONS", "workflow-subscriptions"),
		WorkflowExecutionTimeout: time.Hour,
		WorkflowRunTimeout:       time.Hour,
		WorkflowTaskTimeout:      15 * time.Second,
	}, contracts.WorkflowSubscriptionReset, contracts.SubscriptionResetWorkflowInput{
		WorkflowVersion: "v1",
		ResetReason:     fmt.Sprintf("due_at:%d", subscription.NextResetTime),
		SubscriptionID:  fmt.Sprintf("%d", subscription.Id),
		RequestedBy:     "workflow-worker",
	})
	if err != nil {
		if _, started := err.(*serviceerror.WorkflowExecutionAlreadyStarted); started {
			return
		}
		platformobservability.SysLog(fmt.Sprintf("start subscription reset workflow %d: %v", subscription.Id, err))
	}
}

func subscriptionResetWorkflowID(subscription commerceschema.UserSubscription) string {
	return fmt.Sprintf("subscription-reset-%d-%d", subscription.Id, subscription.NextResetTime)
}

func subscriptionResetTemporalClient() (temporalclient.Client, error) {
	subscriptionResetClientOnce.Do(func() {
		hostPort := platformconfig.GetEnvOrDefaultString("TEMPORAL_HOSTPORT", "")
		if hostPort == "" {
			subscriptionResetClientErr = fmt.Errorf("TEMPORAL_HOSTPORT is required for subscription reset scheduler")
			return
		}
		subscriptionResetClient, subscriptionResetClientErr = temporalclient.Dial(temporalclient.Options{
			HostPort:  hostPort,
			Namespace: platformconfig.GetEnvOrDefaultString("TEMPORAL_NAMESPACE", "default"),
		})
	})
	return subscriptionResetClient, subscriptionResetClientErr
}
