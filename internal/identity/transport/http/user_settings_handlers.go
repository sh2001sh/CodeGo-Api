package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
)

func UpdateUserSetting(c *gin.Context) {
	var req identityapp.UpdateUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := identityapp.UpdateUserSettings(c.GetInt("id"), c.GetInt("role"), req); err != nil {
		switch err {
		case identityapp.ErrInvalidSettingType:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingInvalidType)
		case identityapp.ErrQuotaThresholdGtZero:
			httpapi.ApiErrorI18n(c, i18n.MsgQuotaThresholdGtZero)
		case identityapp.ErrWebhookEmpty:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingWebhookEmpty)
		case identityapp.ErrWebhookInvalid:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingWebhookInvalid)
		case identityapp.ErrEmailInvalid:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingEmailInvalid)
		case identityapp.ErrBarkURLEmpty:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingBarkUrlEmpty)
		case identityapp.ErrBarkURLInvalid:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingBarkUrlInvalid)
		case identityapp.ErrURLMustHTTP:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingUrlMustHttp)
		case identityapp.ErrGotifyURLEmpty:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingGotifyUrlEmpty)
		case identityapp.ErrGotifyTokenEmpty:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingGotifyTokenEmpty)
		case identityapp.ErrGotifyURLInvalid:
			httpapi.ApiErrorI18n(c, i18n.MsgSettingGotifyUrlInvalid)
		case identityapp.ErrUpdateFailed:
			httpapi.ApiErrorI18n(c, i18n.MsgUpdateFailed)
		default:
			httpapi.ApiError(c, err)
		}
		return
	}

	httpapi.ApiSuccessI18n(c, i18n.MsgSettingSaved, nil)
}
