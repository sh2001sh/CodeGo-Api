package notifyx

import (
	"bytes"
	"encoding/json"
	"fmt"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"net/http"
	"net/url"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/dto"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

func NotifyRootUser(t string, subject string, content string) {
	user, err := loadRootUserNotificationTarget()
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to load root user for notification: %s", err.Error()))
		return
	}
	err = NotifyUser(user.Id, user.Email, identitydomain.GetBaseSetting(user), dto.NewNotify(t, subject, content, nil))
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to notify root user: %s", err.Error()))
	}
}

func NotifyUser(userID int, userEmail string, userSetting dto.UserSetting, data dto.Notify) error {
	notifyType := userSetting.NotifyType
	if notifyType == "" {
		notifyType = dto.NotifyTypeEmail
	}

	canSend, err := CheckNotificationLimit(userID, data.Type)
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to check notification limit: %s", err.Error()))
		return err
	}
	if !canSend {
		return fmt.Errorf("notification limit exceeded for user %d with type %s", userID, notifyType)
	}

	switch notifyType {
	case dto.NotifyTypeEmail:
		emailToUse := userSetting.NotificationEmail
		if emailToUse == "" {
			emailToUse = userEmail
		}
		if emailToUse == "" {
			platformobservability.SysLog(fmt.Sprintf("user %d has no email, skip sending email", userID))
			return nil
		}
		return sendEmailNotify(emailToUse, data)
	case dto.NotifyTypeWebhook:
		webhookURLStr := userSetting.WebhookUrl
		if webhookURLStr == "" {
			platformobservability.SysLog(fmt.Sprintf("user %d has no webhook url, skip sending webhook", userID))
			return nil
		}
		return SendWebhookNotify(webhookURLStr, userSetting.WebhookSecret, data)
	case dto.NotifyTypeBark:
		barkURL := userSetting.BarkUrl
		if barkURL == "" {
			platformobservability.SysLog(fmt.Sprintf("user %d has no bark url, skip sending bark", userID))
			return nil
		}
		return sendBarkNotify(barkURL, data)
	case dto.NotifyTypeGotify:
		gotifyURL := userSetting.GotifyUrl
		gotifyToken := userSetting.GotifyToken
		if gotifyURL == "" || gotifyToken == "" {
			platformobservability.SysLog(fmt.Sprintf("user %d has no gotify url or token, skip sending gotify", userID))
			return nil
		}
		return sendGotifyNotify(gotifyURL, gotifyToken, userSetting.GotifyPriority, data)
	}
	return nil
}

func sendEmailNotify(userEmail string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = strings.Replace(content, dto.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}
	return SendEmail(data.Title, userEmail, content)
}

func sendBarkNotify(barkURL string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = strings.Replace(content, dto.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}

	finalURL := strings.ReplaceAll(barkURL, "{{title}}", url.QueryEscape(data.Title))
	finalURL = strings.ReplaceAll(finalURL, "{{content}}", url.QueryEscape(content))

	var req *http.Request
	var resp *http.Response
	var err error
	if platformconfig.EnableWorker() {
		workerReq := &platformhttpx.WorkerRequest{
			URL:    finalURL,
			Key:    platformconfig.WorkerValidKey,
			Method: http.MethodGet,
			Headers: map[string]string{
				"User-Agent": "OneAPI-Bark-Notify/1.0",
			},
		}

		resp, err = platformhttpx.DoWorkerRequest(workerReq)
		if err != nil {
			return fmt.Errorf("failed to send bark request through worker: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("bark request failed with status code: %d", resp.StatusCode)
		}
		return nil
	}

	fetchSetting := platformstore.GetFetchSetting()
	if err := platformsecurity.ValidateURLWithFetchSetting(finalURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("request reject: %v", err)
	}

	req, err = http.NewRequest(http.MethodGet, finalURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create bark request: %v", err)
	}
	req.Header.Set("User-Agent", "OneAPI-Bark-Notify/1.0")

	resp, err = platformhttpx.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send bark request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bark request failed with status code: %d", resp.StatusCode)
	}
	return nil
}

func sendGotifyNotify(gotifyURL string, gotifyToken string, priority int, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = strings.Replace(content, dto.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}

	finalURL := strings.TrimSuffix(gotifyURL, "/") + "/message?token=" + url.QueryEscape(gotifyToken)
	if priority < 0 || priority > 10 {
		priority = 5
	}

	type gotifyMessage struct {
		Title    string `json:"title"`
		Message  string `json:"message"`
		Priority int    `json:"priority"`
	}

	payloadBytes, err := json.Marshal(gotifyMessage{
		Title:    data.Title,
		Message:  content,
		Priority: priority,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal gotify payload: %v", err)
	}

	var req *http.Request
	var resp *http.Response
	if platformconfig.EnableWorker() {
		workerReq := &platformhttpx.WorkerRequest{
			URL:    finalURL,
			Key:    platformconfig.WorkerValidKey,
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
				"User-Agent":   "OneAPI-Gotify-Notify/1.0",
			},
			Body: payloadBytes,
		}

		resp, err = platformhttpx.DoWorkerRequest(workerReq)
		if err != nil {
			return fmt.Errorf("failed to send gotify request through worker: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("gotify request failed with status code: %d", resp.StatusCode)
		}
		return nil
	}

	fetchSetting := platformstore.GetFetchSetting()
	if err := platformsecurity.ValidateURLWithFetchSetting(finalURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("request reject: %v", err)
	}

	req, err = http.NewRequest(http.MethodPost, finalURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create gotify request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "codego-api-gotify/1.0")

	resp, err = platformhttpx.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send gotify request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gotify request failed with status code: %d", resp.StatusCode)
	}
	return nil
}
