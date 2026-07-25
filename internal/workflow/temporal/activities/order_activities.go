package activities

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
)

type OrderActivities struct{}

func (a *OrderActivities) CreateOrderRecord(ctx context.Context, input contracts.OrderFulfillmentWorkflowInput) error {
	_, err := loadSubscriptionOrder(ctx, input.OrderID)
	return err
}

func (a *OrderActivities) ValidatePaymentCallback(ctx context.Context, input contracts.OrderFulfillmentWorkflowInput) (*contracts.OrderCallbackValidationResult, error) {
	order, err := loadSubscriptionOrder(ctx, input.OrderID)
	if err != nil {
		return nil, err
	}
	if input.PaymentProvider != "" && order.PaymentProvider != input.PaymentProvider {
		return nil, fmt.Errorf("payment provider mismatch")
	}
	return &contracts.OrderCallbackValidationResult{Valid: order.Status == constant.TopUpStatusSuccess}, nil
}

func (a *OrderActivities) MarkOrderPaid(ctx context.Context, input contracts.OrderFulfillmentWorkflowInput) error {
	order, err := loadSubscriptionOrder(ctx, input.OrderID)
	if err != nil {
		return err
	}
	if order.Status != constant.TopUpStatusSuccess {
		return fmt.Errorf("subscription order %q is not paid", input.OrderID)
	}
	return nil
}

func (a *OrderActivities) GrantOrderBenefits(ctx context.Context, input contracts.OrderFulfillmentWorkflowInput) error {
	order, err := loadSubscriptionOrder(ctx, input.OrderID)
	if err != nil {
		return err
	}
	if order.Status != constant.TopUpStatusSuccess {
		return fmt.Errorf("subscription order %q benefits are not available", input.OrderID)
	}
	return commerceapp.FulfillPaidSubscriptionOrder(order.TradeNo)
}

func (a *OrderActivities) PublishOrderPaidEvent(ctx context.Context, input contracts.OrderFulfillmentWorkflowInput) error {
	order, err := loadSubscriptionOrder(ctx, input.OrderID)
	if err != nil {
		return err
	}
	if order.Status != constant.TopUpStatusSuccess {
		return nil
	}
	auditapp.RecordLog(order.UserId, auditschema.LogTypeSystem, "order fulfillment workflow projected: "+order.TradeNo)
	return nil
}

func loadSubscriptionOrder(ctx context.Context, orderID string) (*commerceschema.SubscriptionOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("order id is required")
	}
	var order commerceschema.SubscriptionOrder
	if err := platformdb.DB.WithContext(ctx).Where("trade_no = ?", orderID).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (a *OrderActivities) FindResettableSubscriptions(ctx context.Context, input contracts.SubscriptionResetWorkflowInput) error {
	subscription, err := loadWorkflowSubscription(ctx, input.SubscriptionID)
	if err != nil {
		return err
	}
	if subscription.NextResetTime <= 0 || subscription.NextResetTime > platformruntime.GetTimestamp() {
		return fmt.Errorf("subscription %d is not due for reset", subscription.Id)
	}
	return nil
}

func (a *OrderActivities) ResetUsageProjection(ctx context.Context, input contracts.SubscriptionResetWorkflowInput) error {
	id, err := parseWorkflowSubscriptionID(input.SubscriptionID)
	if err != nil {
		return err
	}
	_, err = commerceapp.ResetSubscriptionPeriodProjection(int(id))
	return err
}

func (a *OrderActivities) PublishResetAuditEvents(ctx context.Context, input contracts.SubscriptionResetWorkflowInput) error {
	subscription, err := loadWorkflowSubscription(ctx, input.SubscriptionID)
	if err != nil {
		return err
	}
	auditapp.RecordLog(subscription.UserId, auditschema.LogTypeManage, "subscription reset workflow completed: "+input.SubscriptionID)
	return nil
}

func loadWorkflowSubscription(ctx context.Context, value string) (*commerceschema.UserSubscription, error) {
	id, err := parseWorkflowSubscriptionID(value)
	if err != nil {
		return nil, err
	}
	var subscription commerceschema.UserSubscription
	if err := platformdb.DB.WithContext(ctx).Where("id = ?", id).First(&subscription).Error; err != nil {
		return nil, err
	}
	if subscription.Status != "active" {
		return nil, fmt.Errorf("subscription %d is not active", id)
	}
	return &subscription, nil
}

func parseWorkflowSubscriptionID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid subscription id")
	}
	return id, nil
}
