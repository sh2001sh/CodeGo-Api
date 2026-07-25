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
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/api/serviceerror"
	temporalclient "go.temporal.io/sdk/client"
)

var (
	orderWorkflowClient           temporalclient.Client
	orderWorkflowClientErr        error
	orderWorkflowClientOnce       sync.Once
	orderFulfillmentSchedulerOnce sync.Once
)

const (
	orderFulfillmentScheduleInterval = time.Minute
	orderFulfillmentScheduleBatch    = 300
)

// StartOrderFulfillmentWorkflow records a durable, idempotent workflow for an
// order already committed by the payment callback transaction.
func StartOrderFulfillmentWorkflow(ctx context.Context, order *commerceschema.SubscriptionOrder) error {
	if order == nil || order.TradeNo == "" {
		return fmt.Errorf("paid subscription order is required")
	}
	client, err := orderFulfillmentTemporalClient()
	if err != nil {
		return err
	}
	return startOrderFulfillmentWorkflow(ctx, client, order)
}

func startOrderFulfillmentWorkflow(ctx context.Context, client temporalclient.Client, order *commerceschema.SubscriptionOrder) error {
	if client == nil || order == nil {
		return fmt.Errorf("temporal client and subscription order are required")
	}
	_, err := client.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:                       orderFulfillmentWorkflowID(order.TradeNo),
		TaskQueue:                platformconfig.GetEnvOrDefaultString("TEMPORAL_TASK_QUEUE_ORDERS", "workflow-orders"),
		WorkflowExecutionTimeout: time.Hour,
		WorkflowRunTimeout:       time.Hour,
		WorkflowTaskTimeout:      15 * time.Second,
	}, contracts.WorkflowOrderFulfillment, contracts.OrderFulfillmentWorkflowInput{
		WorkflowVersion: "v1",
		OrderID:         order.TradeNo,
		ProductID:       fmt.Sprintf("%d", order.PlanId),
		PaymentProvider: order.PaymentProvider,
		Amount:          order.Money,
	})
	if _, alreadyStarted := err.(*serviceerror.WorkflowExecutionAlreadyStarted); alreadyStarted {
		return nil
	}
	return err
}

func orderFulfillmentWorkflowID(tradeNo string) string {
	return "order-fulfillment-" + tradeNo
}

// StartPendingOrderFulfillmentScheduler retries durable workflow starts for
// confirmed payments. It makes callback-to-Temporal handoff recoverable.
func StartPendingOrderFulfillmentScheduler(ctx context.Context) {
	orderFulfillmentSchedulerOnce.Do(func() {
		go func() {
			schedulePendingOrderFulfillments(ctx)
			ticker := time.NewTicker(orderFulfillmentScheduleInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					schedulePendingOrderFulfillments(ctx)
				}
			}
		}()
	})
}

func schedulePendingOrderFulfillments(ctx context.Context) {
	var orders []commerceschema.SubscriptionOrder
	if err := platformdb.DB.WithContext(ctx).
		Where("status = ? AND fulfillment_status = ?", "success", commerceschema.SubscriptionOrderFulfillmentPending).
		Order("complete_time asc, id asc").
		Limit(orderFulfillmentScheduleBatch).
		Find(&orders).Error; err != nil {
		platformobservability.SysLog("load pending order fulfillments: " + err.Error())
		return
	}
	for index := range orders {
		if err := StartOrderFulfillmentWorkflow(ctx, &orders[index]); err != nil {
			platformobservability.SysLog(fmt.Sprintf("start pending order fulfillment %s: %v", orders[index].TradeNo, err))
		}
	}
}

func orderFulfillmentTemporalClient() (temporalclient.Client, error) {
	orderWorkflowClientOnce.Do(func() {
		hostPort := platformconfig.GetEnvOrDefaultString("TEMPORAL_HOSTPORT", "")
		if hostPort == "" {
			orderWorkflowClientErr = fmt.Errorf("TEMPORAL_HOSTPORT is required for order fulfillment workflow orchestration")
			return
		}
		orderWorkflowClient, orderWorkflowClientErr = temporalclient.Dial(temporalclient.Options{
			HostPort:  hostPort,
			Namespace: platformconfig.GetEnvOrDefaultString("TEMPORAL_NAMESPACE", "default"),
		})
	})
	return orderWorkflowClient, orderWorkflowClientErr
}
