package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// CloseResponseBodyGracefully closes an upstream response body and logs failures.

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	if err := httpResponse.Body.Close(); err != nil {
		platformobservability.SysError("failed to close response body: " + err.Error())
	}
}

// RelayErrorHandler normalizes non-2xx upstream responses into the shared APIError format.
func RelayErrorHandler(ctx context.Context, resp *http.Response, showBodyWhenFail bool) (apiErr *types.APIError) {
	apiErr = types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	CloseResponseBodyGracefully(resp)
	var errResponse dto.GeneralErrorResponse
	responseBodyText := string(responseBody)
	buildErrWithBody := func(message string) error {
		if message == "" {
			return fmt.Errorf("bad response status code %d, body: %s", resp.StatusCode, responseBodyText)
		}
		return fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, message, responseBodyText)
	}

	err = platformencoding.Unmarshal(responseBody, &errResponse)
	if err != nil {
		if showBodyWhenFail {
			apiErr.Err = buildErrWithBody("")
		} else {
			logger.LogError(ctx, fmt.Sprintf("bad response status code %d, body: %s", resp.StatusCode, responseBodyText))
			apiErr.Err = fmt.Errorf("bad response status code %d", resp.StatusCode)
		}
		return
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		apiErr.Err = fmt.Errorf("status_code=429")
		return
	}

	if platformencoding.GetJSONType(errResponse.Error) == "object" {
		oaiError := errResponse.TryToOpenAIError()
		if oaiError != nil {
			apiErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
			if showBodyWhenFail {
				apiErr.Err = buildErrWithBody(apiErr.Error())
			}
			return
		}
	}
	apiErr = types.NewOpenAIError(errors.New(errResponse.ToMessage()), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	if showBodyWhenFail {
		apiErr.Err = buildErrWithBody(apiErr.Error())
	}
	return
}

// ResetStatusCode applies per-channel status code remapping to upstream errors.
func ResetStatusCode(apiErr *types.APIError, statusCodeMappingStr string) {
	if apiErr == nil {
		return
	}
	if statusCodeMappingStr == "" || statusCodeMappingStr == "{}" {
		return
	}
	statusCodeMapping := make(map[string]any)
	if err := platformencoding.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping); err != nil {
		return
	}
	if apiErr.StatusCode == http.StatusOK {
		return
	}
	codeStr := strconv.Itoa(apiErr.StatusCode)
	if value, ok := statusCodeMapping[codeStr]; ok {
		intCode, ok := parseStatusCodeMappingValue(value)
		if !ok {
			return
		}
		apiErr.StatusCode = intCode
	}
}

func parseStatusCodeMappingValue(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		statusCode, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	case float64:
		if v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		statusCode, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, false
		}
		return statusCode, true
	default:
		return 0, false
	}
}

// ShouldCopyUpstreamHeader reports whether one upstream header should be copied downstream.
func ShouldCopyUpstreamHeader(c *gin.Context, key string, values []string) bool {
	if strings.EqualFold(key, "Content-Length") {
		return false
	}
	if strings.EqualFold(key, constant.RequestIdKey) {
		if c != nil && len(values) > 0 {
			c.Set(constant.UpstreamRequestIdKey, values[0])
		}
		return false
	}
	return true
}

// IOCopyBytesGracefully writes buffered upstream payloads to the downstream response.
func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c == nil || c.Writer == nil {
		return
	}
	captureImageWorkspaceResponse(c, data)

	body := io.NopCloser(bytes.NewBuffer(data))

	if src != nil {
		for key, values := range src.Header {
			if !ShouldCopyUpstreamHeader(c, key, values) || len(values) == 0 {
				continue
			}
			c.Writer.Header().Set(key, values[0])
		}
	}

	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	written, err := io.Copy(c.Writer, body)
	if written > 0 {
		httpctx.SetContextKey(c, constant.ContextKeyResponseBodyDelivered, true)
	}
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}

func captureImageWorkspaceResponse(c *gin.Context, data []byte) {
	if c == nil || !c.GetBool(string(constant.ContextKeyImageWorkspaceCaptureResponse)) {
		return
	}
	c.Set(string(constant.ContextKeyImageWorkspaceResponseBody), append([]byte(nil), data...))
}
