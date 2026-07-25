package http

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
)

type optionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// GetOptions returns visible runtime options and derived completion ratio metadata.
func GetOptions(c *gin.Context) {
	httpapi.ApiSuccess(c, adminopsapp.ListOptions())
}

// UpdateOption validates and persists a runtime option change.
func UpdateOption(c *gin.Context) {
	var req optionUpdateRequest
	if err := platformencoding.DecodeJSON(c.Request.Body, &req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": adminopsapp.ErrOptionInvalidPayload.Error(),
		})
		return
	}

	if err := adminopsapp.UpdateOption(adminopsapp.AdminOptionUpdateInput{
		Key:   req.Key,
		Value: req.Value,
	}); err != nil {
		handleOptionError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// MigrateConsoleSetting migrates legacy console options to console_setting.* keys.
func MigrateConsoleSetting(c *gin.Context) {
	if err := adminopsapp.MigrateConsoleSetting(); err != nil {
		platformobservability.SysError("failed to migrate console settings: " + err.Error())
		c.JSON(stdhttp.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取配置失败，请稍后重试",
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "migrated",
	})
}

func handleOptionError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, adminopsapp.ErrOptionPaymentComplianceRequired):
		httpapi.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
	case errors.Is(err, adminopsapp.ErrOptionComplianceFieldImmutable),
		errors.Is(err, adminopsapp.ErrOptionGitHubOAuthMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionDiscordOAuthMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionOIDCMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionLinuxDOMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionEmailDomainMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionWeChatMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionTurnstileMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionTelegramMissingConfig),
		errors.Is(err, adminopsapp.ErrOptionThemeFrontendInvalid):
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiErrorMsg(c, err.Error())
	}
}
