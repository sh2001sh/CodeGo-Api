package app

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayproviders "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"gorm.io/gorm"
	"io"
	stdhttp "net/http"
	"strings"
)

type ChannelSortOptions = gatewaystore.ChannelSortOptions

// ChannelListParams describes admin list filters for channels.
type ChannelListParams struct {
	Page       int
	PageSize   int
	EnableTag  bool
	Group      string
	Status     int
	TypeFilter int
	Sort       ChannelSortOptions
}

// ChannelListResult describes the paginated admin channel list response.
type ChannelListResult struct {
	Items      []*gatewayschema.Channel
	Total      int64
	Page       int
	PageSize   int
	TypeCounts map[int64]int64
}

// ChannelSearchParams describes admin search filters for channels.
type ChannelSearchParams struct {
	Keyword    string
	Group      string
	Model      string
	Status     int
	TypeFilter int
	EnableTag  bool
	Page       int
	PageSize   int
	IDSort     bool
	Sort       ChannelSortOptions
}

// ChannelSearchResult describes the admin channel search response.
type ChannelSearchResult struct {
	Items      []*gatewayschema.Channel
	Total      int
	TypeCounts map[int64]int64
}

// FetchModelsRequest describes the ad-hoc model discovery request.
type FetchModelsRequest struct {
	BaseURL string `json:"base_url"`
	Type    int    `json:"type"`
	Key     string `json:"key"`
}

func parseChannelStatusFilter(statusParam string) int {
	switch strings.ToLower(statusParam) {
	case "enabled", "1":
		return constant.ChannelStatusEnabled
	case "disabled", "0":
		return 0
	default:
		return -1
	}
}

func sanitizeChannel(channel *gatewayschema.Channel) {
	if channel == nil {
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		channel.ChannelInfo.MultiKeyDisabledReason = nil
		channel.ChannelInfo.MultiKeyDisabledTime = nil
	}
}

func sanitizeChannels(channels []*gatewayschema.Channel) {
	for _, channel := range channels {
		sanitizeChannel(channel)
	}
}

func buildChannelListQuery(group string, statusFilter int, typeFilter int) *gorm.DB {
	query := platformdb.DB.Model(&gatewayschema.Channel{})
	query = gatewaystore.ApplyChannelGroupFilter(query, group)
	if statusFilter == constant.ChannelStatusEnabled {
		query = query.Where("status = ?", constant.ChannelStatusEnabled)
	} else if statusFilter == 0 {
		query = query.Where("status != ?", constant.ChannelStatusEnabled)
	}
	if typeFilter >= 0 {
		query = query.Where("type = ?", typeFilter)
	}
	return query
}

func countChannelTypes(group string, statusFilter int) (map[int64]int64, error) {
	countQuery := buildChannelListQuery(group, statusFilter, -1)
	var results []struct {
		Type  int64
		Count int64
	}
	if err := countQuery.Select("type, count(*) as count").Group("type").Find(&results).Error; err != nil {
		return nil, err
	}

	typeCounts := make(map[int64]int64, len(results))
	for _, result := range results {
		typeCounts[result.Type] = result.Count
	}
	return typeCounts, nil
}

func normalizeModelNames(models []string) []string {
	return lo.Uniq(lo.FilterMap(models, func(modelName string, _ int) (string, bool) {
		trimmed := strings.TrimSpace(modelName)
		return trimmed, trimmed != ""
	}))
}

func getAuthHeader(token string) stdhttp.Header {
	h := stdhttp.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

func getClaudeAuthHeader(token string) stdhttp.Header {
	h := stdhttp.Header{}
	h.Add("x-api-key", token)
	h.Add("anthropic-version", "2023-06-01")
	return h
}

func getResponseBody(method string, url string, channel *gatewayschema.Channel, headers stdhttp.Header) ([]byte, error) {
	req, err := stdhttp.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for key := range headers {
		req.Header.Add(key, headers.Get(key))
	}
	client, err := platformhttpx.NewProxyHTTPClient(gatewaydomain.GetSettings(channel).Proxy)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != stdhttp.StatusOK {
		return nil, fmt.Errorf("status code: %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if err = res.Body.Close(); err != nil {
		return nil, err
	}
	return body, nil
}

func buildFetchModelsHeaders(channel *gatewayschema.Channel, key string) (stdhttp.Header, error) {
	var headers stdhttp.Header
	switch channel.Type {
	case constant.ChannelTypeAnthropic:
		headers = getClaudeAuthHeader(key)
	default:
		headers = getAuthHeader(key)
	}

	headerOverride := gatewaydomain.GetHeaderOverride(channel)
	for headerKey, value := range headerOverride {
		if gatewayproviders.IsHeaderPassthroughRuleKey(headerKey) {
			continue
		}
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid header override for key %s", headerKey)
		}
		if strings.Contains(str, "{api_key}") {
			str = strings.ReplaceAll(str, "{api_key}", key)
		}
		headers.Set(headerKey, str)
	}
	return headers, nil
}

type openAIModel struct {
	ID string `json:"id"`
}

type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

// ParseChannelStatusFilter parses the admin status filter used by channel management routes.
func ParseChannelStatusFilter(statusParam string) int {
	return parseChannelStatusFilter(statusParam)
}

// NewChannelSortOptions normalizes admin channel sort query options.
func NewChannelSortOptions(sortBy string, sortOrder string, idSort bool) ChannelSortOptions {
	return gatewaystore.NewChannelSortOptions(sortBy, sortOrder, idSort)
}

// ListChannels returns the admin-facing paginated channel catalog.
func ListChannels(params ChannelListParams) (*ChannelListResult, error) {
	channelData := make([]*gatewayschema.Channel, 0)
	groupFilter := gatewaystore.NormalizeChannelGroupFilter(params.Group)
	var total int64

	if params.EnableTag {
		tags, err := gatewaystore.ListPaginatedChannelTags(
			buildChannelListQuery(groupFilter, params.Status, params.TypeFilter),
			(params.Page-1)*params.PageSize,
			params.PageSize,
		)
		if err != nil {
			return nil, err
		}
		total, err = gatewaystore.CountChannelTags(buildChannelListQuery(groupFilter, params.Status, params.TypeFilter))
		if err != nil {
			return nil, err
		}
		for _, tag := range tags {
			if tag == nil || *tag == "" {
				continue
			}
			var tagChannels []*gatewayschema.Channel
			err := params.Sort.Apply(
				buildChannelListQuery(groupFilter, params.Status, params.TypeFilter).Where("tag = ?", *tag),
			).Omit("key").Find(&tagChannels).Error
			if err != nil {
				return nil, err
			}
			channelData = append(channelData, tagChannels...)
		}
	} else {
		if err := buildChannelListQuery(groupFilter, params.Status, params.TypeFilter).Count(&total).Error; err != nil {
			return nil, err
		}
		err := params.Sort.Apply(buildChannelListQuery(groupFilter, params.Status, params.TypeFilter)).
			Limit(params.PageSize).
			Offset((params.Page - 1) * params.PageSize).
			Omit("key").
			Find(&channelData).Error
		if err != nil {
			return nil, err
		}
	}

	sanitizeChannels(channelData)
	typeCounts, err := countChannelTypes(groupFilter, params.Status)
	if err != nil {
		return nil, err
	}

	return &ChannelListResult{
		Items:      channelData,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TypeCounts: typeCounts,
	}, nil
}

// SearchChannels returns the admin-facing channel search result.
func SearchChannels(params ChannelSearchParams) (*ChannelSearchResult, error) {
	channelData := make([]*gatewayschema.Channel, 0)
	if params.EnableTag {
		tags, err := gatewaystore.SearchChannelTags(params.Keyword, params.Group, params.Model, params.IDSort)
		if err != nil {
			return nil, err
		}
		for _, tag := range tags {
			if tag == nil || *tag == "" {
				continue
			}
			var tagChannels []*gatewayschema.Channel
			err := params.Sort.Apply(
				buildChannelListQuery(params.Group, -1, -1).Where("tag = ?", *tag),
			).Omit("key").Find(&tagChannels).Error
			if err != nil {
				return nil, err
			}
			channelData = append(channelData, tagChannels...)
		}
	} else {
		channels, err := gatewaystore.SearchChannels(params.Keyword, params.Group, params.Model, params.IDSort, params.Sort)
		if err != nil {
			return nil, err
		}
		channelData = channels
	}

	if params.Status == constant.ChannelStatusEnabled || params.Status == 0 {
		filtered := make([]*gatewayschema.Channel, 0, len(channelData))
		for _, channel := range channelData {
			if params.Status == constant.ChannelStatusEnabled && channel.Status != constant.ChannelStatusEnabled {
				continue
			}
			if params.Status == 0 && channel.Status == constant.ChannelStatusEnabled {
				continue
			}
			filtered = append(filtered, channel)
		}
		channelData = filtered
	}

	typeCounts := make(map[int64]int64)
	for _, channel := range channelData {
		typeCounts[int64(channel.Type)]++
	}

	if params.TypeFilter >= 0 {
		filtered := make([]*gatewayschema.Channel, 0, len(channelData))
		for _, channel := range channelData {
			if channel.Type == params.TypeFilter {
				filtered = append(filtered, channel)
			}
		}
		channelData = filtered
	}

	total := len(channelData)
	startIdx := (params.Page - 1) * params.PageSize
	if startIdx > total {
		startIdx = total
	}
	endIdx := startIdx + params.PageSize
	if endIdx > total {
		endIdx = total
	}

	pagedData := channelData[startIdx:endIdx]
	sanitizeChannels(pagedData)
	return &ChannelSearchResult{
		Items:      pagedData,
		Total:      total,
		TypeCounts: typeCounts,
	}, nil
}

// GetChannel returns one channel for admin display without exposing its key.
func GetChannel(id int) (*gatewayschema.Channel, error) {
	channel, err := gatewaystore.LoadChannelByID(id, false)
	if err != nil {
		return nil, err
	}
	sanitizeChannel(channel)
	return channel, nil
}

// FetchUpstreamModels returns provider-reported model IDs for one stored channel.
func FetchUpstreamModels(id int) ([]string, error) {
	channel, err := gatewaystore.LoadChannelByID(id, true)
	if err != nil {
		return nil, err
	}
	return FetchChannelUpstreamModelIDs(channel)
}

// FixChannelAbilities rebuilds channel ability rows.
func FixChannelAbilities() (int, int, error) {
	successCount, failCount, err := gatewaystore.RebuildChannelAbilities()
	if err == nil {
		gatewaystore.InitChannelCache()
	}
	return successCount, failCount, err
}

// FetchRemoteModels fetches provider model IDs without persisting a channel.
func FetchRemoteModels(req FetchModelsRequest) ([]string, error) {
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[req.Type]
	}

	key := strings.TrimSpace(req.Key)
	key = strings.Split(key, "\n")[0]

	if req.Type == constant.ChannelTypeOllama {
		models, err := gatewayproviders.FetchOllamaModels(baseURL, key)
		if err != nil {
			return nil, fmt.Errorf("获取Ollama模型失败: %s", err.Error())
		}
		names := make([]string, 0, len(models))
		for _, modelInfo := range models {
			names = append(names, modelInfo.Name)
		}
		return names, nil
	}

	if req.Type == constant.ChannelTypeGemini {
		models, err := gatewayproviders.FetchGeminiModels(baseURL, key, "")
		if err != nil {
			return nil, fmt.Errorf("获取Gemini模型失败: %s", err.Error())
		}
		return models, nil
	}

	client := &stdhttp.Client{}
	url := fmt.Sprintf("%s/v1/models", baseURL)
	request, err := stdhttp.NewRequest(stdhttp.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != stdhttp.StatusOK {
		return nil, fmt.Errorf("Failed to fetch models")
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := platformencoding.DecodeJSON(response.Body, &result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, item := range result.Data {
		models = append(models, item.ID)
	}
	return models, nil
}

// FetchChannelUpstreamModelIDs fetches the latest upstream model ids for a channel.
func FetchChannelUpstreamModelIDs(channel *gatewayschema.Channel) ([]string, error) {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	if channel.Type == constant.ChannelTypeOllama {
		key := strings.TrimSpace(strings.Split(channel.Key, "\n")[0])
		models, err := gatewayproviders.FetchOllamaModels(baseURL, key)
		if err != nil {
			return nil, err
		}
		return normalizeModelNames(lo.Map(models, func(item gatewayproviders.OllamaModel, _ int) string {
			return item.Name
		})), nil
	}

	if channel.Type == constant.ChannelTypeGemini {
		key, _, apiErr := gatewaystore.GetNextEnabledChannelKey(channel)
		if apiErr != nil {
			return nil, fmt.Errorf("获取渠道密钥失败: %w", apiErr)
		}
		key = strings.TrimSpace(key)
		models, err := gatewayproviders.FetchGeminiModels(baseURL, key, gatewaydomain.GetSettings(channel).Proxy)
		if err != nil {
			return nil, err
		}
		return normalizeModelNames(models), nil
	}

	var url string
	switch channel.Type {
	case constant.ChannelTypeAli:
		url = fmt.Sprintf("%s/compatible-mode/v1/models", baseURL)
	case constant.ChannelTypeZhipu_v4:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/api/paas/v4/models", baseURL)
		}
	case constant.ChannelTypeVolcEngine:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/v1/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/v1/models", baseURL)
		}
	case constant.ChannelTypeMoonshot:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/v1/models", baseURL)
		}
	default:
		url = fmt.Sprintf("%s/v1/models", baseURL)
	}

	key, _, apiErr := gatewaystore.GetNextEnabledChannelKey(channel)
	if apiErr != nil {
		return nil, fmt.Errorf("获取渠道密钥失败: %w", apiErr)
	}
	key = strings.TrimSpace(key)

	headers, err := buildFetchModelsHeaders(channel, key)
	if err != nil {
		return nil, err
	}
	body, err := getResponseBody(stdhttp.MethodGet, url, channel, headers)
	if err != nil {
		return nil, err
	}

	var result openAIModelsResponse
	if err := platformencoding.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	ids := lo.Map(result.Data, func(item openAIModel, _ int) string {
		if channel.Type == constant.ChannelTypeGemini {
			return strings.TrimPrefix(item.ID, "models/")
		}
		return item.ID
	})
	return normalizeModelNames(ids), nil
}
