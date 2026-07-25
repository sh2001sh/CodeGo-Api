package middleware

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/taskx"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type ModelRequest struct {
	Model string `json:"model"`
	Group string `json:"group,omitempty"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		var channel *gatewayschema.Channel
		channelId, ok := httpctx.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId)
		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
			return
		}
		if ok {
			id, err := strconv.Atoi(channelId.(string))
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			channel, err = gatewaystore.LoadChannelByID(id, true)
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			if channel.Status != constant.ChannelStatusEnabled {
				abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorChannelDisabled))
				return
			}
		} else {
			// Select a channel for the user
			// check token model mapping
			modelLimitEnable := httpctx.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
			if modelLimitEnable {
				s, ok := httpctx.GetContextKey(c, constant.ContextKeyTokenModelLimit)
				if !ok {
					// token model limit is empty, all models are not allowed
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenNoModelAccess))
					return
				}
				var tokenModelLimit map[string]bool
				tokenModelLimit, ok = s.(map[string]bool)
				if !ok {
					tokenModelLimit = map[string]bool{}
				}
				matchName := gatewaystore.FormatMatchingModelName(modelRequest.Model) // match gpts & thinking-*
				if _, ok := tokenModelLimit[matchName]; !ok {
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelRequest.Model}))
					return
				}
			}

			if shouldSelectChannel {
				if modelRequest.Model == "" {
					abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorModelNameRequired))
					return
				}
				var selectGroup string
				usingGroup := httpctx.GetContextKeyString(c, constant.ContextKeyUsingGroup)
				gatewayruntime.StartRouteDecision(c, modelRequest.Model, usingGroup)
				// check path is /pg/chat/completions
				if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
					playgroundRequest := &dto.PlayGroundRequest{}
					err = platformhttpx.UnmarshalBodyReusable(c, playgroundRequest)
					if err != nil {
						abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidPlayground, map[string]any{"Error": err.Error()}))
						return
					}
					if playgroundRequest.Group != "" {
						if !gatewayroutingapp.GroupInUserUsableGroups(usingGroup, playgroundRequest.Group) && playgroundRequest.Group != usingGroup {
							abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupAccessDenied))
							return
						}
						usingGroup = playgroundRequest.Group
						httpctx.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
					}
				} else if strings.HasPrefix(c.Request.URL.Path, "/pg/images/generations") || strings.HasPrefix(c.Request.URL.Path, "/pg/images/edits") {
					requestGroup := strings.TrimSpace(c.Query("group"))
					if requestGroup != "" {
						if !gatewayroutingapp.GroupInUserUsableGroups(usingGroup, requestGroup) && requestGroup != usingGroup {
							abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupAccessDenied))
							return
						}
						usingGroup = requestGroup
						httpctx.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
					}
					httpctx.SetContextKey(c, constant.ContextKeyTokenGroup, usingGroup)
				}

				if preferredChannelID, found := gatewayruntime.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
					preferred, err := gatewaystore.GetCachedChannel(preferredChannelID)
					if err == nil && preferred != nil {
						if preferred.Status != constant.ChannelStatusEnabled ||
							gatewayruntime.IsChannelCooling(preferred.Id, modelRequest.Model) ||
							gatewayroutingapp.ShouldMigrateAutomaticPoolAffinity(usingGroup, modelRequest.Model, preferred.Id) {
							gatewayruntime.InvalidateChannelAffinityForCurrentRequest(c)
							gatewayruntime.ExcludeRouteDecisionCandidate(c, "stale_affinity")
						} else if usingGroup == "auto" {
							userGroup := httpctx.GetContextKeyString(c, constant.ContextKeyUserGroup)
							autoGroups := gatewayroutingapp.GetUserAutoGroup(userGroup)
							for _, g := range autoGroups {
								if gatewaystore.IsChannelEnabledForGroupModel(g, modelRequest.Model, preferred.Id) && !gatewayruntime.IsChannelCooling(preferred.Id, modelRequest.Model) {
									selectGroup = g
									httpctx.SetContextKey(c, constant.ContextKeyAutoGroup, g)
									channel = preferred
									gatewayruntime.MarkChannelAffinityUsed(c, g, preferred.Id)
									gatewayruntime.SelectRouteDecisionCandidate(c, g, preferred.Id, true)
									break
								}
							}
						} else if gatewaystore.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) {
							channel = preferred
							selectGroup = usingGroup
							gatewayruntime.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
							gatewayruntime.SelectRouteDecisionCandidate(c, usingGroup, preferred.Id, true)
						}
					}
				}

				if channel == nil {
					channel, selectGroup, err = gatewayroutingapp.CacheGetRandomSatisfiedChannel(&gatewayroutingapp.RetryParam{
						Ctx:        c,
						ModelName:  modelRequest.Model,
						TokenGroup: usingGroup,
						Retry:      platformruntime.GetPointer(0),
					})
					if err != nil {
						showGroup := usingGroup
						if usingGroup == "auto" {
							showGroup = fmt.Sprintf("auto(%s)", selectGroup)
						}
						logger.LogError(c, i18n.T(c, i18n.MsgDistributorGetChannelFailed, map[string]any{"Group": showGroup, "Model": modelRequest.Model, "Error": err.Error()}))
						// 濡傛灉閿欒锛屼絾鏄笭閬撲笉涓虹┖锛岃鏄庢槸鏁版嵁搴撲竴鑷存€ч棶棰?						//if channel != nil {
						//	platformobservability.SysError(fmt.Sprintf("娓犻亾涓嶅瓨鍦細%d", channel.Id))
						//	message = "鏁版嵁搴撲竴鑷存€у凡琚牬鍧忥紝璇疯仈绯荤鐞嗗憳"
						//}
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, platformtext.UpstreamQuotaGenericMessage, types.ErrorCodeModelNotFound)
						return
					}
					if channel == nil {
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, platformtext.UpstreamQuotaGenericMessage, types.ErrorCodeModelNotFound)
						return
					}
				}
			}
		}
		httpctx.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
		gatewayexecutionapp.SetupContextForSelectedChannel(c, channel, modelRequest.Model)
		c.Next()
		if channel != nil && c.Writer != nil && c.Writer.Status() < http.StatusBadRequest {
			gatewayruntime.RecordChannelAffinity(c, channel.Id)
			gatewayroutingapp.RecordAutomaticPoolAffinity(c, channel.Id)
		}
	}
}

// getModelFromRequest 浠庤姹備腑璇诲彇妯″瀷淇℃伅
// 鏍规嵁 Content-Type 鑷姩澶勭悊锛?// - application/json
// - application/x-www-form-urlencoded
// - multipart/form-data
func getModelFromRequest(c *gin.Context) (*ModelRequest, error) {
	var modelRequest ModelRequest
	err := platformhttpx.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
	}
	return &modelRequest, nil
}

func getModelRequest(c *gin.Context) (*ModelRequest, bool, error) {
	var modelRequest ModelRequest
	shouldSelectChannel := true
	if strings.Contains(c.Request.URL.Path, "/suno/") {
		relayMode := gatewaycontract.Path2RelaySuno(c.Request.Method, c.Request.URL.Path)
		if relayMode == gatewaycontract.RelayModeSunoFetch ||
			relayMode == gatewaycontract.RelayModeSunoFetchByID {
			shouldSelectChannel = false
		} else {
			modelName := taskx.CoverTaskActionToModelName(constant.TaskPlatformSuno, c.Param("action"))
			modelRequest.Model = modelName
		}
		c.Set("platform", string(constant.TaskPlatformSuno))
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/v1/videos/") && strings.HasSuffix(c.Request.URL.Path, "/remix") {
		relayMode := gatewaycontract.RelayModeVideoSubmit
		c.Set("relay_mode", relayMode)
		shouldSelectChannel = false
	} else if strings.Contains(c.Request.URL.Path, "/v1/videos") {
		//curl https://api.openai.com/v1/videos \
		//  -H "Authorization: Bearer $OPENAI_API_KEY" \
		//  -F "model=video-model" \
		//  -F "prompt=A calico cat playing a piano on stage"
		//	-F input_reference="@image.jpg"
		relayMode := gatewaycontract.RelayModeUnknown
		if c.Request.Method == http.MethodPost {
			relayMode = gatewaycontract.RelayModeVideoSubmit
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			if req != nil {
				modelRequest.Model = req.Model
			}
		} else if c.Request.Method == http.MethodGet {
			relayMode = gatewaycontract.RelayModeVideoFetchByID
			shouldSelectChannel = false
		}
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/v1/video/generations") {
		relayMode := gatewaycontract.RelayModeUnknown
		if c.Request.Method == http.MethodPost {
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			modelRequest.Model = req.Model
			relayMode = gatewaycontract.RelayModeVideoSubmit
		} else if c.Request.Method == http.MethodGet {
			relayMode = gatewaycontract.RelayModeVideoFetchByID
			shouldSelectChannel = false
		}
		if _, ok := c.Get("relay_mode"); !ok {
			c.Set("relay_mode", relayMode)
		}
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") || strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
		// Gemini API 璺緞澶勭悊: /v1beta/models/gemini-2.0-flash:generateContent
		relayMode := gatewaycontract.RelayModeGemini
		modelName := extractModelNameFromGeminiPath(c.Request.URL.Path)
		if modelName != "" {
			modelRequest.Model = modelName
		}
		c.Set("relay_mode", relayMode)
	} else if !strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") && !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/realtime") {
		//wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01
		modelRequest.Model = c.Query("model")
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "text-moderation-stable"
		}
	}
	if strings.HasSuffix(c.Request.URL.Path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
		modelRequest.Model = platformtext.GetStringIfEmpty(modelRequest.Model, "dall-e")
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1/images/edits") {
		//modelRequest.Model = platformtext.GetStringIfEmpty(c.PostForm("model"), "gpt-image-1")
		contentType := c.ContentType()
		if slices.Contains([]string{gin.MIMEPOSTForm, gin.MIMEMultipartPOSTForm}, contentType) {
			req, err := getModelFromRequest(c)
			if err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/audio") {
		relayMode := gatewaycontract.RelayModeAudioSpeech
		if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/speech") {

			modelRequest.Model = platformtext.GetStringIfEmpty(modelRequest.Model, "tts-1")
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/translations") {
			// 鍏堝皾璇曚粠璇锋眰璇诲彇
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = platformtext.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = gatewaycontract.RelayModeAudioTranslation
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") {
			// 鍏堝皾璇曚粠璇锋眰璇诲彇
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = platformtext.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = gatewaycontract.RelayModeAudioTranscription
		}
		c.Set("relay_mode", relayMode)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
		// playground chat completions
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
		modelRequest.Group = req.Group
		httpctx.SetContextKey(c, constant.ContextKeyTokenGroup, modelRequest.Group)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/pg/images/generations") || strings.HasPrefix(c.Request.URL.Path, "/pg/images/edits") {
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
		modelRequest.Group = strings.TrimSpace(c.Query("group"))
	}

	if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") && modelRequest.Model != "" {
		modelRequest.Model = gatewaystore.WithCompactModelSuffix(modelRequest.Model)
	}
	return &modelRequest, shouldSelectChannel, nil
}

// extractModelNameFromGeminiPath 浠?Gemini API URL 璺緞涓彁鍙栨ā鍨嬪悕
// 杈撳叆鏍煎紡: /v1beta/models/gemini-2.0-flash:generateContent
// 杈撳嚭: gemini-2.0-flash
func extractModelNameFromGeminiPath(path string) string {
	modelsPrefix := "/models/"
	modelsIndex := strings.Index(path, modelsPrefix)
	if modelsIndex == -1 {
		return ""
	}

	startIndex := modelsIndex + len(modelsPrefix)
	if startIndex >= len(path) {
		return ""
	}

	colonIndex := strings.Index(path[startIndex:], ":")
	if colonIndex == -1 {
		return path[startIndex:]
	}

	return path[startIndex : startIndex+colonIndex]
}
