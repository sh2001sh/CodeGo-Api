package runtime

import "net/url"

// BuildURL resolves an endpoint against the provided base URL.
func BuildURL(base string, endpoint string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + endpoint
	}
	end := endpoint
	if end == "" {
		end = "/"
	}
	ref, err := url.Parse(end)
	if err != nil {
		return base + endpoint
	}
	return u.ResolveReference(ref).String()
}
