package projection

import "github.com/sh2001sh/CodeGo-Api/setting/config"

type perfMetricsSetting struct {
	Enabled       bool   `json:"enabled"`
	FlushInterval int    `json:"flush_interval"`
	BucketTime    string `json:"bucket_time"`
	RetentionDays int    `json:"retention_days"`
}

var projectionPerfMetricsSetting = perfMetricsSetting{
	Enabled:       true,
	FlushInterval: 5,
	BucketTime:    "hour",
	RetentionDays: 0,
}

func init() {
	config.GlobalConfig.Register("perf_metrics_setting", &projectionPerfMetricsSetting)
}

func getPerfMetricsSetting() perfMetricsSetting {
	return projectionPerfMetricsSetting
}

func getPerfMetricsBucketSeconds() int64 {
	switch projectionPerfMetricsSetting.BucketTime {
	case "minute":
		return 60
	case "5min":
		return 300
	case "hour":
		return 3600
	default:
		return 3600
	}
}

func getPerfMetricsFlushIntervalMinutes() int {
	if projectionPerfMetricsSetting.FlushInterval < 1 {
		return 1
	}
	return projectionPerfMetricsSetting.FlushInterval
}
