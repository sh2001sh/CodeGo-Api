package bootstrap

import defaultweb "github.com/sh2001sh/CodeGo-Api/web/default"

func buildIndexPage() []byte {
	return defaultweb.DefaultIndexPage()
}
