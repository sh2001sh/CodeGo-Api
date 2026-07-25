package bootstrap

import gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"

func loadBootstrapPricing() {
	gatewaystore.LoadPricing()
}
