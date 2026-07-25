package tokenx

import (
	"errors"
	"fmt"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"log"
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/filex"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func getImageToken(c *gin.Context, fileMeta *types.FileMeta, model string, stream bool) (int, error) {
	if fileMeta == nil || fileMeta.Source == nil {
		return 0, fmt.Errorf("image_url_is_nil")
	}

	baseTokens := 85
	tileTokens := 170
	lowerModel := strings.ToLower(model)

	if strings.HasPrefix(lowerModel, "glm-4") {
		return 1047, nil
	}

	isPatchBased := false
	multiplier := 1.0
	switch {
	case strings.Contains(lowerModel, "gpt-4.1-mini"):
		isPatchBased = true
		multiplier = 1.62
	case strings.Contains(lowerModel, "gpt-4.1-nano"):
		isPatchBased = true
		multiplier = 2.46
	case strings.HasPrefix(lowerModel, "o4-mini"):
		isPatchBased = true
		multiplier = 1.72
	case strings.HasPrefix(lowerModel, "gpt-5-mini"):
		isPatchBased = true
		multiplier = 1.62
	case strings.HasPrefix(lowerModel, "gpt-5-nano"):
		isPatchBased = true
		multiplier = 2.46
	}

	if !isPatchBased {
		if strings.HasPrefix(lowerModel, "gpt-4o-mini") {
			baseTokens = 2833
			tileTokens = 5667
		} else if strings.HasPrefix(lowerModel, "gpt-5-chat-latest") || (strings.HasPrefix(lowerModel, "gpt-5") && !strings.Contains(lowerModel, "mini") && !strings.Contains(lowerModel, "nano")) {
			baseTokens = 70
			tileTokens = 140
		} else if strings.HasPrefix(lowerModel, "o1") || strings.HasPrefix(lowerModel, "o3") || strings.HasPrefix(lowerModel, "o1-pro") {
			baseTokens = 75
			tileTokens = 150
		} else if strings.Contains(lowerModel, "computer-use-preview") {
			baseTokens = 65
			tileTokens = 129
		} else if strings.Contains(lowerModel, "4.1") || strings.Contains(lowerModel, "4o") || strings.Contains(lowerModel, "4.5") {
			baseTokens = 85
			tileTokens = 170
		}
	}

	if fileMeta.Detail == "low" && !isPatchBased {
		return baseTokens, nil
	}

	if !constant.GetMediaToken {
		return 3 * baseTokens, nil
	}
	if !constant.GetMediaTokenNotStream && !stream {
		return 3 * baseTokens, nil
	}

	if fileMeta.Detail == "auto" || fileMeta.Detail == "" {
		fileMeta.Detail = "high"
	}

	config, format, err := filex.GetImageConfig(c, fileMeta.Source)
	if err != nil {
		return 0, err
	}
	if config.Width == 0 || config.Height == 0 {
		if format != "" {
			return 3 * baseTokens, nil
		}
		return 0, errors.New(fmt.Sprintf("fail to decode image config: %s", fileMeta.GetIdentifier()))
	}

	width := config.Width
	height := config.Height
	log.Printf("format: %s, width: %d, height: %d", format, width, height)

	if isPatchBased {
		ceilDiv := func(a, b int) int { return (a + b - 1) / b }
		rawPatchesW := ceilDiv(width, 32)
		rawPatchesH := ceilDiv(height, 32)
		rawPatches := rawPatchesW * rawPatchesH
		if rawPatches > 1536 {
			area := float64(width * height)
			r := math.Sqrt(float64(32*32*1536) / area)
			wScaled := float64(width) * r
			hScaled := float64(height) * r
			adjW := math.Floor(wScaled/32.0) / (wScaled / 32.0)
			adjH := math.Floor(hScaled/32.0) / (hScaled / 32.0)
			adj := math.Min(adjW, adjH)
			if !math.IsNaN(adj) && adj > 0 {
				r = r * adj
			}
			wScaled = float64(width) * r
			hScaled = float64(height) * r
			patchesW := math.Ceil(wScaled / 32.0)
			patchesH := math.Ceil(hScaled / 32.0)
			imageTokens := int(patchesW * patchesH)
			if imageTokens > 1536 {
				imageTokens = 1536
			}
			return int(math.Round(float64(imageTokens) * multiplier)), nil
		}
		return int(math.Round(float64(rawPatches) * multiplier)), nil
	}

	maxSide := math.Max(float64(width), float64(height))
	fitScale := 1.0
	if maxSide > 2048 {
		fitScale = maxSide / 2048.0
	}
	fitW := int(math.Round(float64(width) / fitScale))
	fitH := int(math.Round(float64(height) / fitScale))

	minSide := math.Min(float64(fitW), float64(fitH))
	if minSide == 0 {
		return baseTokens, nil
	}
	shortScale := 768.0 / minSide
	finalW := int(math.Round(float64(fitW) * shortScale))
	finalH := int(math.Round(float64(fitH) * shortScale))

	tilesW := (finalW + 512 - 1) / 512
	tilesH := (finalH + 512 - 1) / 512
	tiles := tilesW * tilesH

	if platformconfig.DebugEnabled {
		log.Printf("scaled to: %dx%d, tiles: %d", finalW, finalH, tiles)
	}

	return tiles*tileTokens + baseTokens, nil
}

// EstimateRequestToken counts prompt-side tokens for a relay request.
func EstimateRequestToken(c *gin.Context, meta *types.TokenCountMeta, info *relaycommon.RelayInfo) (int, error) {
	if !constant.CountToken {
		return 0, nil
	}
	if meta == nil {
		return 0, errors.New("token count meta is nil")
	}
	if info.RelayFormat == types.RelayFormatOpenAIRealtime {
		return 0, nil
	}
	if info.RelayMode == gatewaycontract.RelayModeAudioTranscription || info.RelayMode == gatewaycontract.RelayModeAudioTranslation {
		multiForm, err := platformhttpx.ParseMultipartFormReusable(c)
		if err != nil {
			return 0, fmt.Errorf("error parsing multipart form: %v", err)
		}
		fileHeaders := multiForm.File["file"]
		totalAudioToken := 0
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil {
				return 0, fmt.Errorf("error opening audio file: %v", err)
			}
			defer file.Close()

			ext := filepath.Ext(fileHeader.Filename)
			duration, err := filex.GetAudioDuration(c.Request.Context(), file, ext)
			if err != nil {
				return 0, fmt.Errorf("error getting audio duration: %v", err)
			}
			totalAudioToken += int(math.Round(math.Ceil(duration) / 60.0 * 1000))
		}
		return totalAudioToken, nil
	}

	model := httpctx.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	tkm := 0
	if meta.TokenType == types.TokenTypeTextNumber {
		tkm += utf8.RuneCountInString(meta.CombineText)
	} else {
		tkm += CountTextToken(meta.CombineText, model)
	}

	if info.RelayFormat == types.RelayFormatOpenAI {
		tkm += meta.ToolsCount * 8
		tkm += meta.MessagesCount * 3
		tkm += meta.NameCount * 3
		tkm += 3
	}

	shouldFetchFiles := true
	if info.RelayFormat == types.RelayFormatGemini {
		shouldFetchFiles = false
	}
	if !constant.GetMediaToken {
		shouldFetchFiles = false
	}
	if !constant.GetMediaTokenNotStream && !info.IsStream {
		shouldFetchFiles = false
	}

	for _, file := range meta.Files {
		if file.Source == nil {
			continue
		}

		if file.FileType == "" || (file.Source.IsURL() && shouldFetchFiles) {
			cachedData, err := filex.LoadFileSource(c, file.Source, "token_counter")
			if err != nil {
				if shouldFetchFiles {
					return 0, fmt.Errorf("error getting file type: %v", err)
				}
				continue
			}
			file.FileType = filex.DetectFileType(cachedData.MimeType)
		}
	}

	for i, file := range meta.Files {
		switch file.FileType {
		case types.FileTypeImage:
			if gatewaycontract.IsOpenAITextModel(model) {
				token, err := getImageToken(c, file, model, info.IsStream)
				if err != nil {
					return 0, fmt.Errorf("error counting image token, media index[%d], identifier[%s], err: %v", i, file.GetIdentifier(), err)
				}
				tkm += token
			} else {
				tkm += 520
			}
		case types.FileTypeAudio:
			tkm += 256
		case types.FileTypeVideo:
			tkm += 4096 * 2
		case types.FileTypeFile:
			tkm += 4096
		default:
			tkm += 4096
		}
	}

	httpctx.SetContextKey(c, constant.ContextKeyPromptTokens, tkm)
	return tkm, nil
}

// CountTokenRealtime counts text and audio tokens for Realtime events.
func CountTokenRealtime(info *relaycommon.RelayInfo, request dto.RealtimeEvent, model string) (int, int, error) {
	audioToken := 0
	textToken := 0
	switch request.Type {
	case dto.RealtimeEventTypeSessionUpdate:
		if request.Session != nil {
			msgTokens := CountTextToken(request.Session.Instructions, model)
			textToken += msgTokens
		}
	case dto.RealtimeEventResponseAudioDelta:
		atk, err := CountAudioTokenOutput(request.Delta, info.OutputAudioFormat)
		if err != nil {
			return 0, 0, fmt.Errorf("error counting audio token: %v", err)
		}
		audioToken += atk
	case dto.RealtimeEventResponseAudioTranscriptionDelta, dto.RealtimeEventResponseFunctionCallArgumentsDelta:
		tkm := CountTextToken(request.Delta, model)
		textToken += tkm
	case dto.RealtimeEventInputAudioBufferAppend:
		atk, err := CountAudioTokenInput(request.Audio, info.InputAudioFormat)
		if err != nil {
			return 0, 0, fmt.Errorf("error counting audio token: %v", err)
		}
		audioToken += atk
	case dto.RealtimeEventConversationItemCreated:
		if request.Item != nil && request.Item.Type == "message" {
			for _, content := range request.Item.Content {
				if content.Type == "input_text" {
					textToken += CountTextToken(content.Text, model)
				}
			}
		}
	case dto.RealtimeEventTypeResponseDone:
		if !info.IsFirstRequest && info.RealtimeTools != nil && len(info.RealtimeTools) > 0 {
			for _, tool := range info.RealtimeTools {
				toolTokens := CountTokenInput(tool, model)
				textToken += 8
				textToken += toolTokens
			}
		}
	}
	return textToken, audioToken, nil
}

// CountTokenInput counts tokens for generic input payloads.
func CountTokenInput(input any, model string) int {
	switch v := input.(type) {
	case string:
		return CountTextToken(v, model)
	case []string:
		text := ""
		for _, s := range v {
			text += s
		}
		return CountTextToken(text, model)
	case []interface{}:
		text := ""
		for _, item := range v {
			text += fmt.Sprintf("%v", item)
		}
		return CountTextToken(text, model)
	}
	return CountTokenInput(fmt.Sprintf("%v", input), model)
}

// CountAudioTokenInput estimates input-side audio tokens from inline audio payloads.
func CountAudioTokenInput(audioBase64 string, audioFormat string) (int, error) {
	if audioBase64 == "" {
		return 0, nil
	}
	duration, err := parseAudio(audioBase64, audioFormat)
	if err != nil {
		return 0, err
	}
	return int(duration / 60 * 100 / 0.06), nil
}

// CountAudioTokenOutput estimates output-side audio tokens from inline audio payloads.
func CountAudioTokenOutput(audioBase64 string, audioFormat string) (int, error) {
	if audioBase64 == "" {
		return 0, nil
	}
	duration, err := parseAudio(audioBase64, audioFormat)
	if err != nil {
		return 0, err
	}
	return int(duration / 60 * 200 / 0.24), nil
}

// CountTextToken counts text tokens using either tokenizer or heuristics.
func CountTextToken(text string, model string) int {
	if text == "" {
		return 0
	}
	if gatewaycontract.IsOpenAITextModel(model) {
		tokenEncoder := getTokenEncoder(model)
		return getTokenNum(tokenEncoder, text)
	}
	return EstimateTokenByModel(model, text)
}
