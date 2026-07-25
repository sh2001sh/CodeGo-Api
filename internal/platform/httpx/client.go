package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	"golang.org/x/net/proxy"
)

var (
	httpClient      *http.Client
	proxyClientLock sync.Mutex
	proxyClients    = make(map[string]*http.Client)
)

func relayResponseHeaderTimeout() time.Duration {
	if platformconfig.RelayResponseHeaderTimeout <= 0 {
		return 0
	}
	return time.Duration(platformconfig.RelayResponseHeaderTimeout) * time.Second
}

func newOutboundTransport(proxyFunc func(*http.Request) (*url.URL, error), dialContext func(context.Context, string, string) (net.Conn, error)) *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:        platformconfig.RelayMaxIdleConns,
		MaxIdleConnsPerHost: platformconfig.RelayMaxIdleConnsPerHost,
		ForceAttemptHTTP2:   true,
		Proxy:               proxyFunc,
		DialContext:         dialContext,
	}
	if timeout := relayResponseHeaderTimeout(); timeout > 0 {
		transport.ResponseHeaderTimeout = timeout
		transport.TLSHandshakeTimeout = timeout
		if transport.DialContext == nil {
			transport.DialContext = (&net.Dialer{Timeout: timeout}).DialContext
		}
	}
	if platformconfig.TLSInsecureSkipVerify {
		transport.TLSClientConfig = platformconfig.InsecureTLSConfig
	}
	return transport
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := platformstore.GetFetchSetting()
	urlStr := req.URL.String()
	if err := platformsecurity.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

// InitHTTPClient initializes the shared outbound HTTP client.
func InitHTTPClient() {
	transport := newOutboundTransport(http.ProxyFromEnvironment, nil)

	if platformconfig.RelayTimeout == 0 {
		httpClient = &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
		return
	}

	httpClient = &http.Client{
		Transport:     transport,
		Timeout:       time.Duration(platformconfig.RelayTimeout) * time.Second,
		CheckRedirect: checkRedirect,
	}
}

// GetHTTPClient returns the shared outbound HTTP client.
func GetHTTPClient() *http.Client {
	return httpClient
}

// GetHTTPClientWithProxy returns the shared client or a proxy-enabled client.
func GetHTTPClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHTTPClient(), nil
	}
	return NewProxyHTTPClient(proxyURL)
}

// ResetProxyClientCache clears cached proxy-specific HTTP clients.
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	for _, client := range proxyClients {
		if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
			transport.CloseIdleConnections()
		}
	}
	proxyClients = make(map[string]*http.Client)
}

// NewProxyHTTPClient creates or reuses a proxy-specific HTTP client.
func NewProxyHTTPClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHTTPClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}

	proxyClientLock.Lock()
	if client, ok := proxyClients[proxyURL]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport := newOutboundTransport(http.ProxyURL(parsedURL), nil)
		client := &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
		client.Timeout = time.Duration(platformconfig.RelayTimeout) * time.Second
		proxyClientLock.Lock()
		proxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := newOutboundTransport(nil, func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		})

		client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
		client.Timeout = time.Duration(platformconfig.RelayTimeout) * time.Second
		proxyClientLock.Lock()
		proxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
