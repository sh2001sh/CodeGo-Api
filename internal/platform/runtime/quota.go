package runtime

// QuotaPerUnit converts USD-denominated pricing into internal quota units.
var QuotaPerUnit = 500 * 1000.0 // $0.002 / 1K tokens

// GetTrustQuota returns the default quota threshold for trusted session flows.
func GetTrustQuota() int {
	return int(10 * QuotaPerUnit)
}
