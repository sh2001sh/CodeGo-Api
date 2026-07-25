package app

import (
	"fmt"
	"io"
	"net/http"

	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

func getAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

func getClaudeAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("x-api-key", token)
	h.Add("anthropic-version", "2023-06-01")
	return h
}

func getResponseBody(method string, url string, channel *gatewayschema.Channel, headers http.Header) ([]byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for key := range headers {
		req.Header.Add(key, headers.Get(key))
	}
	client, err := platformhttpx.NewProxyHTTPClient(gatewaydomain.GetSettings(channel).Proxy)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if err = res.Body.Close(); err != nil {
		return nil, err
	}
	return body, nil
}
