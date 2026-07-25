package defaultweb

import (
	"embed"

	platformhttp "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http"
)

//go:embed dist
var buildFS embed.FS

//go:embed dist/index.html
var distIndexPage []byte

func BuildFS() embed.FS {
	return buildFS
}

func DefaultIndexPage() []byte {
	return distIndexPage
}

func ThemeAssets(indexPage []byte) platformhttp.ThemeAssets {
	return platformhttp.ThemeAssets{
		DefaultBuildFS:   buildFS,
		DefaultIndexPage: indexPage,
	}
}
