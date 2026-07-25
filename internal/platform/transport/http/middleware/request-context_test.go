package middleware

import "testing"

func TestIsHeavyGlobalAPIRateLimitedRequest(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{method: "GET", path: "/api/user/self", want: true},
		{method: "GET", path: "/api/user/topup/info", want: true},
		{method: "GET", path: "/api/subscription/self", want: true},
		{method: "POST", path: "/api/user/amount", want: true},
		{method: "POST", path: "/api/user/topup", want: false},
		{method: "GET", path: "/api/user/topup", want: false},
	}

	for _, tt := range tests {
		if got := IsHeavyGlobalAPIRateLimitedRequest(tt.method, tt.path); got != tt.want {
			t.Fatalf("IsHeavyGlobalAPIRateLimitedRequest(%q, %q) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestIsAuthenticatedAPIRoute(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{method: "GET", path: "/api/packages/public", want: true},
		{method: "POST", path: "/api/user/login", want: false},
		{method: "GET", path: "/api/user/self", want: true},
	}

	for _, tt := range tests {
		if got := isAuthenticatedAPIRoute(tt.method, tt.path); got != tt.want {
			t.Fatalf("isAuthenticatedAPIRoute(%q, %q) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}
