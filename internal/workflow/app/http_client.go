package app

import (
	"net/http"

	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

// GetHttpClient returns the shared protected outbound client. Workflow fetches
// use the same SSRF and redirect rules as every other user-controlled fetch.
func GetHttpClient() *http.Client {
	return platformhttpx.GetHTTPClient()
}

// GetHttpClientWithProxy returns a protected shared client with optional proxy support.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	return platformhttpx.GetHTTPClientWithProxy(proxyURL)
}
