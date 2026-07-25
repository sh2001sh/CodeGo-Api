package codex

import (
	"github.com/samber/lo"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

var baseModelList = []string{
	"gpt-5", "gpt-5-codex", "gpt-5-codex-mini",
	"gpt-5.1", "gpt-5.1-codex", "gpt-5.1-codex-max", "gpt-5.1-codex-mini",
	"gpt-5.2", "gpt-5.2-codex", "gpt-5.3-codex", "gpt-5.3-codex-spark",
	"gpt-5.4",
}

var ModelList = withCompactModelSuffix(baseModelList)

const ChannelName = "codex"

func withCompactModelSuffix(models []string) []string {
	out := make([]string, 0, len(models)*2)
	out = append(out, models...)
	out = append(out, lo.Map(models, func(model string, _ int) string {
		return gatewaystore.WithCompactModelSuffix(model)
	})...)
	return lo.Uniq(out)
}
