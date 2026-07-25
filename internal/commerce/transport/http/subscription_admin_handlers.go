package http

import (
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"strconv"

	"github.com/gin-gonic/gin"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	"gorm.io/gorm"
)

func listAdminSubscriptionPlans(c *gin.Context) {
	payload, err := commerceapp.ListAdminSubscriptionPlans()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func createAdminSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req commerceapp.AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	plan, err := commerceapp.CreateAdminSubscriptionPlan(req)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, plan)
}

func updateAdminSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	planID, err := strconv.Atoi(c.Param("id"))
	if err != nil || planID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid id")
		return
	}

	var req commerceapp.AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	if err := commerceapp.UpdateAdminSubscriptionPlan(planID, req); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func updateAdminSubscriptionPlanStatus(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	planID, err := strconv.Atoi(c.Param("id"))
	if err != nil || planID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid id")
		return
	}

	var req commerceapp.AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	if err := commerceapp.UpdateAdminSubscriptionPlanStatus(planID, *req.Enabled); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func deleteAdminSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	planID, err := strconv.Atoi(c.Param("id"))
	if err != nil || planID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid id")
		return
	}
	message, err := commerceapp.DeleteAdminSubscriptionPlan(planID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, subscriptionAdminMessagePayload(message))
}

func bindAdminSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req commerceapp.AdminBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	message, err := commerceapp.BindAdminSubscription(req)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, subscriptionAdminMessagePayload(message))
}

func listAdminUserSubscriptions(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid user id")
		return
	}
	payload, err := commerceapp.ListAdminUserSubscriptions(userID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func createAdminUserSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid user id")
		return
	}

	var req commerceapp.AdminCreateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	message, err := commerceapp.CreateAdminUserSubscription(userID, req)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, subscriptionAdminMessagePayload(message))
}

func updateAdminUserSubscription(c *gin.Context) {
	subscriptionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid subscription id")
		return
	}

	var req commerceapp.AdminUpdateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	if err := validateAdminUserSubscriptionRequest(req); err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
		return
	}
	message, err := commerceapp.UpdateAdminUserSubscription(subscriptionID, req)
	if err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			httpapi.ApiErrorMsg(c, err.Error())
			return
		}
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, subscriptionAdminMessagePayload(message))
}

func invalidateAdminUserSubscription(c *gin.Context) {
	subscriptionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid subscription id")
		return
	}
	message, err := commerceapp.InvalidateAdminUserSubscription(subscriptionID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, subscriptionAdminMessagePayload(message))
}

func deleteAdminUserSubscription(c *gin.Context) {
	subscriptionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid subscription id")
		return
	}
	message, err := commerceapp.DeleteAdminUserSubscription(subscriptionID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, subscriptionAdminMessagePayload(message))
}

func resetAdminUserSubscriptionQuota(c *gin.Context) {
	subscriptionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid subscription id")
		return
	}

	var req commerceapp.AdminResetUserSubscriptionQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, gorm.ErrInvalidData) {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	payload, err := commerceapp.ResetAdminUserSubscriptionQuota(subscriptionID, req, c.GetInt("id"), c.GetString("username"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func subscriptionAdminMessagePayload(message string) any {
	if message == "" {
		return nil
	}
	return map[string]any{"message": message}
}

func validateAdminUserSubscriptionRequest(req commerceapp.AdminUpdateUserSubscriptionRequest) error {
	switch {
	case req.StartTime <= 0 || req.EndTime <= 0 || req.EndTime <= req.StartTime:
		return errors.New("invalid subscription time range")
	case req.AmountTotal < 0 || req.AmountUsed < 0 || req.PeriodAmount < 0 || req.PeriodUsed < 0:
		return errors.New("quota values must be >= 0")
	case req.AmountTotal > 0 && req.AmountUsed > req.AmountTotal:
		return errors.New("amount_used cannot exceed amount_total")
	case req.PeriodAmount > 0 && req.PeriodUsed > req.PeriodAmount:
		return errors.New("period_used cannot exceed period_amount")
	default:
		return nil
	}
}
