package app

import (
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/shopspring/decimal"
	"net/http"
	"strconv"
	"time"
)

func updateChannelAIProxyBalance(channel *gatewayschema.Channel) (float64, error) {
	body, err := getResponseBody(http.MethodGet, "https://aiproxy.io/api/report/getUserOverview", channel, http.Header{
		"Api-Key": []string{channel.Key},
	})
	if err != nil {
		return 0, err
	}

	var response aiProxyUserOverviewResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}
	if !response.Success {
		return 0, fmt.Errorf("code: %d, message: %s", response.ErrorCode, response.Message)
	}

	gatewaystore.UpdateChannelBalance(channel, response.Data.TotalPoints)
	return response.Data.TotalPoints, nil
}

func updateChannelAPI2GPTBalance(channel *gatewayschema.Channel) (float64, error) {
	body, err := getResponseBody(
		http.MethodGet,
		"https://api.api2gpt.com/dashboard/billing/credit_grants",
		channel,
		getAuthHeader(channel.Key),
	)
	if err != nil {
		return 0, err
	}

	var response api2GPTUsageResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}

	gatewaystore.UpdateChannelBalance(channel, response.TotalRemaining)
	return response.TotalRemaining, nil
}

func updateChannelSiliconFlowBalance(channel *gatewayschema.Channel) (float64, error) {
	body, err := getResponseBody(
		http.MethodGet,
		"https://api.siliconflow.cn/v1/user/info",
		channel,
		getAuthHeader(channel.Key),
	)
	if err != nil {
		return 0, err
	}

	var response siliconFlowUsageResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}
	if response.Code != 20000 {
		return 0, fmt.Errorf("code: %d, message: %s", response.Code, response.Message)
	}

	balance, err := strconv.ParseFloat(response.Data.TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	gatewaystore.UpdateChannelBalance(channel, balance)
	return balance, nil
}

func updateChannelDeepSeekBalance(channel *gatewayschema.Channel) (float64, error) {
	body, err := getResponseBody(
		http.MethodGet,
		"https://api.deepseek.com/user/balance",
		channel,
		getAuthHeader(channel.Key),
	)
	if err != nil {
		return 0, err
	}

	var response deepSeekUsageResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}

	for _, balanceInfo := range response.BalanceInfos {
		if balanceInfo.Currency != "CNY" {
			continue
		}
		balance, parseErr := strconv.ParseFloat(balanceInfo.TotalBalance, 64)
		if parseErr != nil {
			return 0, parseErr
		}
		gatewaystore.UpdateChannelBalance(channel, balance)
		return balance, nil
	}

	return 0, errors.New("currency CNY not found")
}

func updateChannelAIGC2DBalance(channel *gatewayschema.Channel) (float64, error) {
	body, err := getResponseBody(
		http.MethodGet,
		"https://api.aigc2d.com/dashboard/billing/credit_grants",
		channel,
		getAuthHeader(channel.Key),
	)
	if err != nil {
		return 0, err
	}

	var response aigc2DUsageResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}

	gatewaystore.UpdateChannelBalance(channel, response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelOpenRouterBalance(channel *gatewayschema.Channel) (float64, error) {
	body, err := getResponseBody(
		http.MethodGet,
		"https://openrouter.ai/api/v1/credits",
		channel,
		getAuthHeader(channel.Key),
	)
	if err != nil {
		return 0, err
	}

	var response openRouterCreditResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}

	balance := response.Data.TotalCredits - response.Data.TotalUsage
	gatewaystore.UpdateChannelBalance(channel, balance)
	return balance, nil
}

func updateChannelMoonshotBalance(channel *gatewayschema.Channel) (float64, error) {
	type moonshotBalanceResponse struct {
		Code int `json:"code"`
		Data struct {
			AvailableBalance float64 `json:"available_balance"`
		} `json:"data"`
		Scode  string `json:"scode"`
		Status bool   `json:"status"`
	}

	body, err := getResponseBody(
		http.MethodGet,
		"https://api.moonshot.cn/v1/users/me/balance",
		channel,
		getAuthHeader(channel.Key),
	)
	if err != nil {
		return 0, err
	}

	var response moonshotBalanceResponse
	if err = platformencoding.Unmarshal(body, &response); err != nil {
		return 0, err
	}
	if !response.Status || response.Code != 0 {
		return 0, fmt.Errorf("failed to update moonshot balance, status: %v, code: %d, scode: %s", response.Status, response.Code, response.Scode)
	}

	availableBalanceUSD := decimal.NewFromFloat(response.Data.AvailableBalance).
		Div(decimal.NewFromFloat(commercestore.Price)).
		InexactFloat64()
	gatewaystore.UpdateChannelBalance(channel, availableBalanceUSD)
	return availableBalanceUSD, nil
}

func updateDefaultOpenAIBalance(channel *gatewayschema.Channel, baseURL string) (float64, error) {
	subscriptionURL := fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL)
	body, err := getResponseBody(http.MethodGet, subscriptionURL, channel, getAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}

	var subscription openAISubscriptionResponse
	if err = platformencoding.Unmarshal(body, &subscription); err != nil {
		return 0, err
	}

	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}

	usageURL := fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate)
	body, err = getResponseBody(http.MethodGet, usageURL, channel, getAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}

	var usage openAIUsageResponse
	if err = platformencoding.Unmarshal(body, &usage); err != nil {
		return 0, err
	}

	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	gatewaystore.UpdateChannelBalance(channel, balance)
	return balance, nil
}

func refreshChannelBalance(channel *gatewayschema.Channel) (float64, error) {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() == "" {
		channel.BaseURL = &baseURL
	}

	switch channel.Type {
	case constant.ChannelTypeOpenAI:
		if channel.GetBaseURL() != "" {
			baseURL = channel.GetBaseURL()
		}
	case constant.ChannelTypeAzure:
		return 0, errors.New("尚未实现")
	case constant.ChannelTypeCustom:
		baseURL = channel.GetBaseURL()
	case constant.ChannelTypeAIProxy:
		return updateChannelAIProxyBalance(channel)
	case constant.ChannelTypeAPI2GPT:
		return updateChannelAPI2GPTBalance(channel)
	case constant.ChannelTypeAIGC2D:
		return updateChannelAIGC2DBalance(channel)
	case constant.ChannelTypeSiliconFlow:
		return updateChannelSiliconFlowBalance(channel)
	case constant.ChannelTypeDeepSeek:
		return updateChannelDeepSeekBalance(channel)
	case constant.ChannelTypeOpenRouter:
		return updateChannelOpenRouterBalance(channel)
	case constant.ChannelTypeMoonshot:
		return updateChannelMoonshotBalance(channel)
	default:
		return 0, errors.New("尚未实现")
	}

	return updateDefaultOpenAIBalance(channel, baseURL)
}
