package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type adminOpsRedemptionPage struct {
	Total int                         `json:"total"`
	Items []commerceschema.Redemption `json:"items"`
}

type adminOpsRatioSyncData struct {
	Differences map[string]map[string]dto.DifferenceItem `json:"differences"`
	TestResults []dto.TestResult                         `json:"test_results"`
}

func TestConfirmPaymentComplianceRejectsAccessToken(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/option/payment_compliance", map[string]any{
		"confirmed": true,
	})
	ctx.Set("use_access_token", true)

	ConfirmPaymentCompliance(ctx)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}

	response := decodeAdminOpsResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected access token confirmation request to fail")
	}
}

func TestConfirmPaymentCompliancePersistsConfirmation(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	originalSetting := *commercestore.GetPaymentSetting()
	t.Cleanup(func() {
		*commercestore.GetPaymentSetting() = originalSetting
	})

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/option/payment_compliance", map[string]any{
		"confirmed": true,
	})
	ctx.Set("id", 42)
	ctx.Request.RemoteAddr = "203.0.113.9:4567"

	ConfirmPaymentCompliance(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected compliance confirmation to succeed, got %#v", response)
	}

	var payload struct {
		Confirmed    bool   `json:"confirmed"`
		TermsVersion string `json:"terms_version"`
		ConfirmedAt  int64  `json:"confirmed_at"`
		ConfirmedBy  int    `json:"confirmed_by"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode payment compliance payload: %v", err)
	}
	if !payload.Confirmed || payload.ConfirmedBy != 42 {
		t.Fatalf("unexpected compliance payload: %#v", payload)
	}
	if payload.TermsVersion != commercestore.CurrentComplianceTermsVersion {
		t.Fatalf("expected terms version %q, got %q", commercestore.CurrentComplianceTermsVersion, payload.TermsVersion)
	}

	var options []platformschema.Option
	if err := platformdb.DB.Find(&options).Error; err != nil {
		t.Fatalf("failed to load persisted options: %v", err)
	}
	if len(options) != 5 {
		t.Fatalf("expected 5 persisted options, got %d", len(options))
	}
	if !commercestore.IsPaymentComplianceConfirmed() {
		t.Fatalf("expected runtime payment compliance setting to be updated")
	}
}

func TestPrefillGroupHandlersCRUD(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	createCtx, createRecorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/prefill_group/", map[string]any{
		"name":        "core-models",
		"type":        "model",
		"items":       []string{"gpt-4o", "claude-3-5-sonnet"},
		"description": "core model presets",
	})
	CreatePrefillGroup(createCtx)

	createResponse := decodeAdminOpsResponse(t, createRecorder)
	if !createResponse.Success {
		t.Fatalf("expected create prefill group to succeed, got %#v", createResponse)
	}

	var created gatewayschema.PrefillGroup
	if err := platformencoding.Unmarshal(createResponse.Data, &created); err != nil {
		t.Fatalf("failed to decode created prefill group: %v", err)
	}

	listCtx, listRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/prefill_group/?type=model", nil)
	GetPrefillGroups(listCtx)

	listResponse := decodeAdminOpsResponse(t, listRecorder)
	if !listResponse.Success {
		t.Fatalf("expected list prefill groups to succeed, got %#v", listResponse)
	}
	var groups []gatewayschema.PrefillGroup
	if err := platformencoding.Unmarshal(listResponse.Data, &groups); err != nil {
		t.Fatalf("failed to decode prefill group list: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "core-models" {
		t.Fatalf("unexpected prefill groups: %#v", groups)
	}

	updateCtx, updateRecorder := newAdminOpsContext(t, stdhttp.MethodPut, "/api/prefill_group/", map[string]any{
		"id":          created.Id,
		"name":        "core-models-v2",
		"type":        "model",
		"items":       []string{"gpt-4.1"},
		"description": "updated",
	})
	UpdatePrefillGroup(updateCtx)

	updateResponse := decodeAdminOpsResponse(t, updateRecorder)
	if !updateResponse.Success {
		t.Fatalf("expected update prefill group to succeed, got %#v", updateResponse)
	}

	deleteCtx, deleteRecorder := newAdminOpsContext(t, stdhttp.MethodDelete, "/api/prefill_group/"+strconv.Itoa(created.Id), nil)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(created.Id)}}
	DeletePrefillGroup(deleteCtx)

	deleteResponse := decodeAdminOpsResponse(t, deleteRecorder)
	if !deleteResponse.Success {
		t.Fatalf("expected delete prefill group to succeed, got %#v", deleteResponse)
	}

	var remaining []gatewayschema.PrefillGroup
	if err := platformdb.DB.Where("type = ?", "model").Find(&remaining).Error; err != nil {
		t.Fatalf("failed to reload prefill groups: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected prefill groups to be deleted, got %#v", remaining)
	}
}

func TestVendorMetaHandlersCRUD(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	createCtx, createRecorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/vendors/", map[string]any{
		"name":        "OpenAI",
		"description": "foundation model vendor",
		"icon":        "openai",
		"status":      1,
	})
	CreateVendorMeta(createCtx)

	createResponse := decodeAdminOpsResponse(t, createRecorder)
	if !createResponse.Success {
		t.Fatalf("expected create vendor to succeed, got %#v", createResponse)
	}

	var created gatewayschema.Vendor
	if err := platformencoding.Unmarshal(createResponse.Data, &created); err != nil {
		t.Fatalf("failed to decode created vendor: %v", err)
	}

	getCtx, getRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/vendors/"+strconv.Itoa(created.Id), nil)
	getCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(created.Id)}}
	GetVendorMeta(getCtx)

	getResponse := decodeAdminOpsResponse(t, getRecorder)
	if !getResponse.Success {
		t.Fatalf("expected get vendor to succeed, got %#v", getResponse)
	}

	searchCtx, searchRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/vendors/search?keyword=OpenAI&p=1&page_size=10", nil)
	SearchVendors(searchCtx)

	searchResponse := decodeAdminOpsResponse(t, searchRecorder)
	if !searchResponse.Success {
		t.Fatalf("expected search vendors to succeed, got %#v", searchResponse)
	}
	var page platformpagination.PageInfo
	if err := platformencoding.Unmarshal(searchResponse.Data, &page); err != nil {
		t.Fatalf("failed to decode vendor page: %v", err)
	}
	items, ok := page.Items.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected vendor page items: %#v", page.Items)
	}

	updateCtx, updateRecorder := newAdminOpsContext(t, stdhttp.MethodPut, "/api/vendors/", map[string]any{
		"id":          created.Id,
		"name":        "OpenAI Updated",
		"description": "updated vendor",
		"icon":        "openai",
		"status":      1,
	})
	UpdateVendorMeta(updateCtx)

	updateResponse := decodeAdminOpsResponse(t, updateRecorder)
	if !updateResponse.Success {
		t.Fatalf("expected update vendor to succeed, got %#v", updateResponse)
	}

	listCtx, listRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/vendors/?p=1&page_size=10", nil)
	GetAllVendors(listCtx)

	listResponse := decodeAdminOpsResponse(t, listRecorder)
	if !listResponse.Success {
		t.Fatalf("expected list vendors to succeed, got %#v", listResponse)
	}

	deleteCtx, deleteRecorder := newAdminOpsContext(t, stdhttp.MethodDelete, "/api/vendors/"+strconv.Itoa(created.Id), nil)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(created.Id)}}
	DeleteVendorMeta(deleteCtx)

	deleteResponse := decodeAdminOpsResponse(t, deleteRecorder)
	if !deleteResponse.Success {
		t.Fatalf("expected delete vendor to succeed, got %#v", deleteResponse)
	}

	var vendorCount int64
	if err := platformdb.DB.Model(&gatewayschema.Vendor{}).Count(&vendorCount).Error; err != nil {
		t.Fatalf("failed to count vendors: %v", err)
	}
	if vendorCount != 0 {
		t.Fatalf("expected vendors to be deleted, got %d", vendorCount)
	}
}

func TestAddRedemptionRequiresPaymentCompliance(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	originalSetting := *commercestore.GetPaymentSetting()
	*commercestore.GetPaymentSetting() = commercestore.PaymentSetting{}
	t.Cleanup(func() {
		*commercestore.GetPaymentSetting() = originalSetting
	})

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/redemption/", map[string]any{
		"name":        "quota-batch",
		"count":       1,
		"quota":       2000,
		"redeem_type": commerceschema.RedemptionTypeQuota,
	})
	ctx.Set("id", 99)

	AddRedemption(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected add redemption to fail without payment compliance")
	}
}

func TestRedemptionHandlersCRUD(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	originalSetting := *commercestore.GetPaymentSetting()
	setting := commercestore.GetPaymentSetting()
	setting.ComplianceConfirmed = true
	setting.ComplianceTermsVersion = commercestore.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		*commercestore.GetPaymentSetting() = originalSetting
	})

	createCtx, createRecorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/redemption/", map[string]any{
		"name":        "quota-batch",
		"count":       2,
		"quota":       2000,
		"redeem_type": commerceschema.RedemptionTypeQuota,
	})
	createCtx.Set("id", 77)
	AddRedemption(createCtx)

	createResponse := decodeAdminOpsResponse(t, createRecorder)
	if !createResponse.Success {
		t.Fatalf("expected create redemption to succeed, got %#v", createResponse)
	}
	var keys []string
	if err := platformencoding.Unmarshal(createResponse.Data, &keys); err != nil {
		t.Fatalf("failed to decode redemption keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 redemption keys, got %#v", keys)
	}

	listCtx, listRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/redemption/?p=1&page_size=10", nil)
	GetAllRedemptions(listCtx)

	listResponse := decodeAdminOpsResponse(t, listRecorder)
	if !listResponse.Success {
		t.Fatalf("expected redemption list to succeed, got %#v", listResponse)
	}
	var page adminOpsRedemptionPage
	if err := platformencoding.Unmarshal(listResponse.Data, &page); err != nil {
		t.Fatalf("failed to decode redemption page: %v", err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected redemption page: %#v", page)
	}

	searchCtx, searchRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/redemption/search?keyword=quota-batch&p=1&page_size=10", nil)
	SearchRedemptions(searchCtx)

	searchResponse := decodeAdminOpsResponse(t, searchRecorder)
	if !searchResponse.Success {
		t.Fatalf("expected redemption search to succeed, got %#v", searchResponse)
	}

	target := page.Items[0]
	getCtx, getRecorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/redemption/"+strconv.Itoa(target.Id), nil)
	getCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(target.Id)}}
	GetRedemption(getCtx)

	getResponse := decodeAdminOpsResponse(t, getRecorder)
	if !getResponse.Success {
		t.Fatalf("expected get redemption to succeed, got %#v", getResponse)
	}

	updateCtx, updateRecorder := newAdminOpsContext(t, stdhttp.MethodPut, "/api/redemption/", map[string]any{
		"id":           target.Id,
		"name":         "quota-batch-updated",
		"quota":        3000,
		"redeem_type":  commerceschema.RedemptionTypeQuota,
		"expired_time": 0,
	})
	UpdateRedemption(updateCtx)

	updateResponse := decodeAdminOpsResponse(t, updateRecorder)
	if !updateResponse.Success {
		t.Fatalf("expected update redemption to succeed, got %#v", updateResponse)
	}

	statusCtx, statusRecorder := newAdminOpsContext(t, stdhttp.MethodPut, "/api/redemption/?status_only=1", map[string]any{
		"id":     target.Id,
		"status": constant.RedemptionCodeStatusDisabled,
	})
	UpdateRedemption(statusCtx)

	statusResponse := decodeAdminOpsResponse(t, statusRecorder)
	if !statusResponse.Success {
		t.Fatalf("expected status-only update to succeed, got %#v", statusResponse)
	}

	deleteCtx, deleteRecorder := newAdminOpsContext(t, stdhttp.MethodDelete, "/api/redemption/"+strconv.Itoa(page.Items[1].Id), nil)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(page.Items[1].Id)}}
	DeleteRedemption(deleteCtx)

	deleteResponse := decodeAdminOpsResponse(t, deleteRecorder)
	if !deleteResponse.Success {
		t.Fatalf("expected delete redemption to succeed, got %#v", deleteResponse)
	}
}

func TestDeleteInvalidRedemptionRemovesDisabledAndExpiredEntries(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	now := platformruntime.GetTimestamp()
	seed := []commerceschema.Redemption{
		{Name: "active", Key: "active-code", Status: constant.RedemptionCodeStatusEnabled, CreatedTime: now, Quota: 100, RedeemType: commerceschema.RedemptionTypeQuota},
		{Name: "disabled", Key: "disabled-code", Status: constant.RedemptionCodeStatusDisabled, CreatedTime: now, Quota: 100, RedeemType: commerceschema.RedemptionTypeQuota},
		{Name: "used", Key: "used-code", Status: constant.RedemptionCodeStatusUsed, CreatedTime: now, Quota: 100, RedeemType: commerceschema.RedemptionTypeQuota},
		{Name: "expired", Key: "expired-code", Status: constant.RedemptionCodeStatusEnabled, CreatedTime: now, Quota: 100, RedeemType: commerceschema.RedemptionTypeQuota, ExpiredTime: now - 60},
	}
	for i := range seed {
		if err := platformdb.DB.Create(&seed[i]).Error; err != nil {
			t.Fatalf("failed to seed redemption: %v", err)
		}
	}

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodDelete, "/api/redemption/invalid", nil)
	DeleteInvalidRedemption(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected delete invalid redemption to succeed, got %#v", response)
	}

	var rows int64
	if err := platformencoding.Unmarshal(response.Data, &rows); err != nil {
		t.Fatalf("failed to decode deleted row count: %v", err)
	}
	if rows != 3 {
		t.Fatalf("expected 3 invalid redemptions to be deleted, got %d", rows)
	}

	var remaining []commerceschema.Redemption
	if err := platformdb.DB.Find(&remaining).Error; err != nil {
		t.Fatalf("failed to load remaining redemptions: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Key != "active-code" {
		t.Fatalf("unexpected remaining redemptions: %#v", remaining)
	}
}

func TestGetSyncableChannelsIncludesPresetsAndSeededChannel(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	baseURL := "https://syncable.example.com"
	channel := &gatewayschema.Channel{
		Name:    "Syncable Channel",
		Key:     "test-key",
		Status:  1,
		BaseURL: &baseURL,
	}
	if err := platformdb.DB.Create(channel).Error; err != nil {
		t.Fatalf("failed to seed channel: %v", err)
	}

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/ratio_sync/channels", nil)
	GetSyncableChannels(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected get syncable channels to succeed, got %#v", response)
	}

	var channels []dto.SyncableChannel
	if err := platformencoding.Unmarshal(response.Data, &channels); err != nil {
		t.Fatalf("failed to decode syncable channels: %v", err)
	}

	foundSeeded := false
	foundOfficial := false
	foundModelsDev := false
	for _, item := range channels {
		if item.ID == channel.Id && item.BaseURL == baseURL {
			foundSeeded = true
		}
		if item.ID == -100 {
			foundOfficial = true
		}
		if item.ID == -101 {
			foundModelsDev = true
		}
	}
	if !foundSeeded || !foundOfficial || !foundModelsDev {
		t.Fatalf("unexpected syncable channels: %#v", channels)
	}
}

func TestFetchUpstreamRatiosReturnsDifferences(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"model_ratio":{"zz-test-model-ratio-sync":1.25},"completion_ratio":{"zz-test-model-ratio-sync":2}}}`))
	}))
	defer upstream.Close()

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/ratio_sync/fetch", map[string]any{
		"upstreams": []map[string]any{
			{
				"name":     "mock-upstream",
				"base_url": upstream.URL,
				"endpoint": "/api/pricing",
			},
		},
		"timeout": 1,
	})
	FetchUpstreamRatios(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected fetch upstream ratios to succeed, got %#v", response)
	}

	var payload adminOpsRatioSyncData
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode ratio sync payload: %v", err)
	}
	if len(payload.TestResults) != 1 || payload.TestResults[0].Status != "success" {
		t.Fatalf("unexpected test results: %#v", payload.TestResults)
	}
	modelDiffs, ok := payload.Differences["zz-test-model-ratio-sync"]
	if !ok {
		t.Fatalf("expected differences for test model, got %#v", payload.Differences)
	}
	if _, ok := modelDiffs["model_ratio"]; !ok {
		t.Fatalf("expected model_ratio difference, got %#v", modelDiffs)
	}
}
