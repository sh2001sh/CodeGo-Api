package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"net/http"
	"strings"

	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

// WorkerRequest describes a proxied worker download request.
type WorkerRequest struct {
	URL     string            `json:"url"`
	Key     string            `json:"key"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// DoWorkerRequest sends a fetch request through the configured worker.
func DoWorkerRequest(req *WorkerRequest) (*http.Response, error) {
	if !platformconfig.EnableWorker() {
		return nil, fmt.Errorf("worker not enabled")
	}
	if !platformconfig.WorkerAllowHttpImageRequestEnabled && !strings.HasPrefix(req.URL, "https") {
		return nil, fmt.Errorf("only support https url")
	}

	fetchSetting := platformstore.GetFetchSetting()
	if err := platformsecurity.ValidateURLWithFetchSetting(req.URL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return nil, fmt.Errorf("request reject: %v", err)
	}

	workerURL := platformconfig.WorkerUrl
	if !strings.HasSuffix(workerURL, "/") {
		workerURL += "/"
	}

	workerPayload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal worker payload: %v", err)
	}

	return GetHTTPClient().Post(workerURL, "application/json", bytes.NewBuffer(workerPayload))
}

// DoDownloadRequest fetches a remote URL with worker and SSRF rules applied.
func DoDownloadRequest(originURL string, reason ...string) (*http.Response, error) {
	if platformconfig.EnableWorker() {
		platformobservability.SysLog(fmt.Sprintf("downloading file from worker: %s, reason: %s", originURL, strings.Join(reason, ", ")))
		req := &WorkerRequest{
			URL: originURL,
			Key: platformconfig.WorkerValidKey,
		}
		return DoWorkerRequest(req)
	}

	fetchSetting := platformstore.GetFetchSetting()
	if err := platformsecurity.ValidateURLWithFetchSetting(originURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return nil, fmt.Errorf("request reject: %v", err)
	}

	platformobservability.SysLog(fmt.Sprintf("downloading from origin: %s, reason: %s", originURL, strings.Join(reason, ", ")))
	return GetHTTPClient().Get(originURL)
}
