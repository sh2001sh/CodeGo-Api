package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type weChatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

var (
	ErrWeChatAuthDisabled        = errors.New("管理员未开启通过微信登录以及注册")
	ErrTelegramOAuthDisabled     = errors.New("管理员未开启通过 Telegram 登录以及注册")
	ErrMessagingOAuthInvalid     = errors.New("无效的请求")
	ErrWeChatAlreadyBound        = errors.New("该微信账号已被绑定")
	ErrTelegramAlreadyBound      = errors.New("该 Telegram 账户已被绑定")
	ErrMessagingUserDeleted      = errors.New("用户已注销")
	ErrMessagingRegisterDisabled = errors.New("管理员关闭了新用户注册")
	ErrMessagingUserBanned       = errors.New("用户已被封禁")
)

var weChatHTTPClient = &http.Client{Timeout: 5 * time.Second}

// SetWeChatHTTPClientForTest overrides the WeChat HTTP client for transport tests.
func SetWeChatHTTPClientForTest(client *http.Client) {
	if client == nil {
		weChatHTTPClient = &http.Client{Timeout: 5 * time.Second}
		return
	}
	weChatHTTPClient = client
}

// CompleteWeChatLogin completes the WeChat login or registration flow.
func CompleteWeChatLogin(ctx context.Context, code string) (*AuthenticatedSessionUser, error) {
	if !platformconfig.WeChatAuthEnabled {
		return nil, ErrWeChatAuthDisabled
	}

	wechatID, err := getWeChatIDByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	user := &identityschema.User{WeChatId: wechatID}
	if identitystore.IsWeChatIDTaken(wechatID) {
		loadedUser, err := identitystore.LoadUserByWeChatID(wechatID)
		if err != nil {
			return nil, err
		}
		user = loadedUser
		if user.Id == 0 {
			return nil, ErrMessagingUserDeleted
		}
	} else {
		if !platformconfig.RegisterEnabled {
			return nil, ErrMessagingRegisterDisabled
		}
		user.Username = "wechat_" + strconv.Itoa(identitystore.LoadMaxUserID()+1)
		user.DisplayName = "WeChat User"
		user.Role = constant.RoleCommonUser
		user.Status = constant.UserStatusEnabled
		if err := createUserAndRecordRegistration(user); err != nil {
			return nil, err
		}
	}

	if user.Status != constant.UserStatusEnabled {
		return nil, ErrMessagingUserBanned
	}
	return BuildAuthenticatedSessionUser(user), nil
}

// BindWeChatAccount binds a WeChat account to an existing authenticated user.
func BindWeChatAccount(ctx context.Context, userID int, code string) error {
	if !platformconfig.WeChatAuthEnabled {
		return ErrWeChatAuthDisabled
	}
	if userID == 0 {
		return ErrMessagingOAuthInvalid
	}

	wechatID, err := getWeChatIDByCode(ctx, code)
	if err != nil {
		return err
	}
	if identitystore.IsWeChatIDTaken(wechatID) {
		return ErrWeChatAlreadyBound
	}

	user, err := identitystore.LoadUserByID(userID, true)
	if err != nil {
		return err
	}
	if user.Id == 0 {
		return ErrMessagingUserDeleted
	}
	user.WeChatId = wechatID
	return identitystore.UpdateUser(user, false)
}

// CompleteTelegramLogin validates the Telegram payload and logs the user in.
func CompleteTelegramLogin(params map[string][]string) (*AuthenticatedSessionUser, error) {
	if !platformconfig.TelegramOAuthEnabled {
		return nil, ErrTelegramOAuthDisabled
	}

	telegramID, err := validatedTelegramID(params, platformconfig.TelegramBotToken)
	if err != nil {
		return nil, err
	}

	user, err := identitystore.LoadUserByTelegramID(telegramID)
	if err != nil {
		return nil, err
	}
	if user.Id == 0 {
		return nil, ErrMessagingUserDeleted
	}
	if user.Status != constant.UserStatusEnabled {
		return nil, ErrMessagingUserBanned
	}
	return BuildAuthenticatedSessionUser(user), nil
}

// BindTelegramAccount validates the Telegram payload and binds it to the user.
func BindTelegramAccount(userID int, params map[string][]string) error {
	if !platformconfig.TelegramOAuthEnabled {
		return ErrTelegramOAuthDisabled
	}
	if userID == 0 {
		return ErrMessagingOAuthInvalid
	}

	telegramID, err := validatedTelegramID(params, platformconfig.TelegramBotToken)
	if err != nil {
		return err
	}
	if identitystore.IsTelegramIDTaken(telegramID) {
		return ErrTelegramAlreadyBound
	}

	user, err := identitystore.LoadUserByID(userID, true)
	if err != nil {
		return err
	}
	if user.Id == 0 {
		return ErrMessagingUserDeleted
	}
	user.TelegramId = telegramID
	return identitystore.UpdateUser(user, false)
}

func getWeChatIDByCode(ctx context.Context, code string) (string, error) {
	if code == "" {
		return "", errors.New("无效的参数")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/wechat/user?code=%s", platformconfig.WeChatServerAddress, url.QueryEscape(code)),
		nil,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", platformconfig.WeChatServerToken)

	resp, err := weChatHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload weChatLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if !payload.Success {
		return "", errors.New(payload.Message)
	}
	if payload.Data == "" {
		return "", errors.New("验证码错误或已过期")
	}
	return payload.Data, nil
}

func validatedTelegramID(params map[string][]string, token string) (string, error) {
	if token == "" {
		return "", ErrMessagingOAuthInvalid
	}
	ids := params["id"]
	hashes := params["hash"]
	if len(ids) == 0 || len(hashes) == 0 {
		return "", ErrMessagingOAuthInvalid
	}
	if !checkTelegramAuthorization(params, token) {
		return "", ErrMessagingOAuthInvalid
	}
	return ids[0], nil
}

func checkTelegramAuthorization(params map[string][]string, token string) bool {
	lines := make([]string, 0, len(params))
	hash := ""
	for key, values := range params {
		if len(values) == 0 {
			return false
		}
		if key == "hash" {
			hash = values[0]
			continue
		}
		lines = append(lines, key+"="+values[0])
	}
	if hash == "" {
		return false
	}

	sort.Strings(lines)
	joined := ""
	for _, line := range lines {
		if joined != "" {
			joined += "\n"
		}
		joined += line
	}

	sha256Hash := sha256.New()
	io.WriteString(sha256Hash, token)
	hmacHash := hmac.New(sha256.New, sha256Hash.Sum(nil))
	io.WriteString(hmacHash, joined)
	return hash == hex.EncodeToString(hmacHash.Sum(nil))
}
