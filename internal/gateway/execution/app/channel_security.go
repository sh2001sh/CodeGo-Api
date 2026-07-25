package app

import (
	"context"
	"fmt"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	"time"

	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

type codexCredentialRefresher func(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *gatewayschema.Channel, error)

var refreshCodexChannelCredential = RefreshCodexChannelCredential

// SetRefreshCodexCredentialForTest temporarily overrides the Codex credential refresher.
func SetRefreshCodexCredentialForTest(
	refresher func(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *gatewayschema.Channel, error),
) func() {
	original := refreshCodexChannelCredential
	refreshCodexChannelCredential = refresher
	return func() {
		refreshCodexChannelCredential = original
	}
}

// ChannelKeyResult contains the admin-facing channel key payload.
type ChannelKeyResult struct {
	Key string `json:"key"`
}

// CodexCredentialRefreshResult contains the refreshed credential metadata returned to the UI.
type CodexCredentialRefreshResult struct {
	ExpiresAt   string `json:"expires_at"`
	LastRefresh string `json:"last_refresh"`
	AccountID   string `json:"account_id"`
	Email       string `json:"email"`
	ChannelID   int    `json:"channel_id"`
	ChannelType int    `json:"channel_type"`
	ChannelName string `json:"channel_name"`
}

// GetChannelKey returns one channel key and records the audit log.
func GetChannelKey(userID int, channelID int) (*ChannelKeyResult, error) {
	channel, err := gatewaystore.LoadChannelByID(channelID, true)
	if err != nil {
		return nil, fmt.Errorf("获取渠道信息失败: %v", err)
	}
	if channel == nil {
		return nil, fmt.Errorf("渠道不存在")
	}

	auditapp.RecordLog(userID, auditschema.LogTypeSystem, fmt.Sprintf("查看渠道密钥信息 (渠道ID: %d)", channelID))
	return &ChannelKeyResult{Key: channel.Key}, nil
}

// RefreshCodexCredential refreshes one Codex channel credential and returns UI-facing metadata.
func RefreshCodexCredential(channelID int) (*CodexCredentialRefreshResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oauthKey, channel, err := refreshCodexChannelCredential(ctx, channelID, CodexCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		return nil, err
	}

	return &CodexCredentialRefreshResult{
		ExpiresAt:   oauthKey.Expired,
		LastRefresh: oauthKey.LastRefresh,
		AccountID:   oauthKey.AccountID,
		Email:       oauthKey.Email,
		ChannelID:   channel.Id,
		ChannelType: channel.Type,
		ChannelName: channel.Name,
	}, nil
}
