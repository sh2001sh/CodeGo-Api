package app

import (
	"errors"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	"net/url"
	"strings"
)

var (
	ErrInvalidSettingType   = errors.New(i18n.MsgSettingInvalidType)
	ErrQuotaThresholdGtZero = errors.New(i18n.MsgQuotaThresholdGtZero)
	ErrWebhookEmpty         = errors.New(i18n.MsgSettingWebhookEmpty)
	ErrWebhookInvalid       = errors.New(i18n.MsgSettingWebhookInvalid)
	ErrEmailInvalid         = errors.New(i18n.MsgSettingEmailInvalid)
	ErrBarkURLEmpty         = errors.New(i18n.MsgSettingBarkUrlEmpty)
	ErrBarkURLInvalid       = errors.New(i18n.MsgSettingBarkUrlInvalid)
	ErrURLMustHTTP          = errors.New(i18n.MsgSettingUrlMustHttp)
	ErrGotifyURLEmpty       = errors.New(i18n.MsgSettingGotifyUrlEmpty)
	ErrGotifyTokenEmpty     = errors.New(i18n.MsgSettingGotifyTokenEmpty)
	ErrGotifyURLInvalid     = errors.New(i18n.MsgSettingGotifyUrlInvalid)
	ErrUpdateFailed         = errors.New(i18n.MsgUpdateFailed)
)

// UpdateUserSettingsRequest captures the writable user alerting and preference settings.
type UpdateUserSettingsRequest struct {
	QuotaWarningType                 string  `json:"notify_type"`
	QuotaWarningThreshold            float64 `json:"quota_warning_threshold"`
	WebhookURL                       string  `json:"webhook_url,omitempty"`
	WebhookSecret                    string  `json:"webhook_secret,omitempty"`
	NotificationEmail                string  `json:"notification_email,omitempty"`
	BarkURL                          string  `json:"bark_url,omitempty"`
	GotifyURL                        string  `json:"gotify_url,omitempty"`
	GotifyToken                      string  `json:"gotify_token,omitempty"`
	GotifyPriority                   int     `json:"gotify_priority,omitempty"`
	UpstreamModelUpdateNotifyEnabled *bool   `json:"upstream_model_update_notify_enabled,omitempty"`
	AcceptUnsetModelRatioModel       bool    `json:"accept_unset_model_ratio_model"`
	RecordIPLog                      bool    `json:"record_ip_log"`
}

// UpdateUserSettings validates and persists the authenticated user's alerting preferences.
func UpdateUserSettings(userID int, userRole int, req UpdateUserSettingsRequest) error {
	if err := validateUserSettingsRequest(req); err != nil {
		return err
	}

	user, err := LoadUserByID(userID, true)
	if err != nil {
		return err
	}

	existingSettings := identitydomain.GetSetting(user)
	upstreamNotifyEnabled := existingSettings.UpstreamModelUpdateNotifyEnabled
	if userRole >= constant.RoleAdminUser && req.UpstreamModelUpdateNotifyEnabled != nil {
		upstreamNotifyEnabled = *req.UpstreamModelUpdateNotifyEnabled
	}

	settings := existingSettings
	settings.NotifyType = req.QuotaWarningType
	settings.QuotaWarningThreshold = req.QuotaWarningThreshold
	settings.UpstreamModelUpdateNotifyEnabled = upstreamNotifyEnabled
	settings.AcceptUnsetRatioModel = req.AcceptUnsetModelRatioModel
	settings.RecordIpLog = req.RecordIPLog
	settings.WebhookUrl = ""
	settings.WebhookSecret = ""
	settings.NotificationEmail = ""
	settings.BarkUrl = ""
	settings.GotifyUrl = ""
	settings.GotifyToken = ""
	settings.GotifyPriority = 0

	switch req.QuotaWarningType {
	case dto.NotifyTypeWebhook:
		settings.WebhookUrl = req.WebhookURL
		if req.WebhookSecret != "" {
			settings.WebhookSecret = req.WebhookSecret
		}
	case dto.NotifyTypeEmail:
		if req.NotificationEmail != "" {
			settings.NotificationEmail = req.NotificationEmail
		}
	case dto.NotifyTypeBark:
		settings.BarkUrl = req.BarkURL
	case dto.NotifyTypeGotify:
		settings.GotifyUrl = req.GotifyURL
		settings.GotifyToken = req.GotifyToken
		if req.GotifyPriority < 0 || req.GotifyPriority > 10 {
			settings.GotifyPriority = 5
		} else {
			settings.GotifyPriority = req.GotifyPriority
		}
	}

	identitydomain.SetSetting(user, settings)
	if err := identitystore.UpdateUser(user, false); err != nil {
		return ErrUpdateFailed
	}
	return nil
}

func validateUserSettingsRequest(req UpdateUserSettingsRequest) error {
	switch req.QuotaWarningType {
	case dto.NotifyTypeEmail, dto.NotifyTypeWebhook, dto.NotifyTypeBark, dto.NotifyTypeGotify:
	default:
		return ErrInvalidSettingType
	}

	if req.QuotaWarningThreshold <= 0 {
		return ErrQuotaThresholdGtZero
	}

	if req.QuotaWarningType == dto.NotifyTypeWebhook {
		if req.WebhookURL == "" {
			return ErrWebhookEmpty
		}
		if _, err := url.ParseRequestURI(req.WebhookURL); err != nil {
			return ErrWebhookInvalid
		}
	}

	if req.QuotaWarningType == dto.NotifyTypeEmail && req.NotificationEmail != "" {
		if !strings.Contains(req.NotificationEmail, "@") {
			return ErrEmailInvalid
		}
	}

	if req.QuotaWarningType == dto.NotifyTypeBark {
		if req.BarkURL == "" {
			return ErrBarkURLEmpty
		}
		if _, err := url.ParseRequestURI(req.BarkURL); err != nil {
			return ErrBarkURLInvalid
		}
		if !strings.HasPrefix(req.BarkURL, "https://") && !strings.HasPrefix(req.BarkURL, "http://") {
			return ErrURLMustHTTP
		}
	}

	if req.QuotaWarningType == dto.NotifyTypeGotify {
		if req.GotifyURL == "" {
			return ErrGotifyURLEmpty
		}
		if req.GotifyToken == "" {
			return ErrGotifyTokenEmpty
		}
		if _, err := url.ParseRequestURI(req.GotifyURL); err != nil {
			return ErrGotifyURLInvalid
		}
		if !strings.HasPrefix(req.GotifyURL, "https://") && !strings.HasPrefix(req.GotifyURL, "http://") {
			return ErrURLMustHTTP
		}
	}

	return nil
}
