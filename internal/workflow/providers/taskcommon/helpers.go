package taskcommon

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
)

// UnmarshalMetadata converts a map[string]any metadata to a typed struct via JSON round-trip.
func UnmarshalMetadata(metadata map[string]any, target any) error {
	if metadata == nil {
		return nil
	}
	delete(metadata, "model")
	metaBytes, err := platformencoding.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	if err := platformencoding.Unmarshal(metaBytes, target); err != nil {
		return fmt.Errorf("unmarshal metadata failed: %w", err)
	}
	return nil
}

func DefaultString(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

func DefaultInt(val, fallback int) int {
	if val == 0 {
		return fallback
	}
	return val
}

func EncodeLocalTaskID(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

func DecodeLocalTaskID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func BuildProxyURL(taskID string) string {
	return fmt.Sprintf("%s/v1/videos/%s/content", platformconfig.ServerAddress, taskID)
}

const (
	ProgressSubmitted  = "10%"
	ProgressQueued     = "20%"
	ProgressInProgress = "30%"
	ProgressComplete   = "100%"
)

type BaseBilling struct{}

func (BaseBilling) EstimateBilling(_ *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	return nil
}

func (BaseBilling) AdjustBillingOnSubmit(_ *relaycommon.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (BaseBilling) AdjustBillingOnComplete(_ *workflowschema.Task, _ *relaycommon.TaskInfo) int {
	return 0
}
