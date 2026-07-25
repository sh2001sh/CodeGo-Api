package projection

import (
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
	// perfMetricRecord stores aggregated relay performance metrics for audit read models.
)

type perfMetricRecord struct {
	ID             int    `json:"id" gorm:"primaryKey"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_model_group_bucket,priority:1"`
	Group          string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_model_group_bucket,priority:2"`
	BucketTs       int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_perf_model_group_bucket,priority:3;index:idx_perf_bucket_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	TtftSumMs      int64  `json:"-" gorm:"default:0"`
	TtftCount      int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
}

func (perfMetricRecord) TableName() string {
	return "perf_metrics"
}

type perfMetricSummaryRow struct {
	ModelName      string `json:"model_name"`
	Group          string `json:"group"`
	RequestCount   int64  `json:"request_count"`
	SuccessCount   int64  `json:"success_count"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	OutputTokens   int64  `json:"output_tokens"`
	GenerationMs   int64  `json:"generation_ms"`
}

func EnsureSchema() error {
	if !platformconfig.IsMasterNode || platformdb.DB == nil {
		return nil
	}
	return platformdb.DB.AutoMigrate(&perfMetricRecord{})
}

func upsertMetric(record *perfMetricRecord) error {
	if record == nil || record.RequestCount == 0 {
		return nil
	}
	return platformdb.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":    gorm.Expr("perf_metrics.request_count + ?", record.RequestCount),
			"success_count":    gorm.Expr("perf_metrics.success_count + ?", record.SuccessCount),
			"total_latency_ms": gorm.Expr("perf_metrics.total_latency_ms + ?", record.TotalLatencyMs),
			"ttft_sum_ms":      gorm.Expr("perf_metrics.ttft_sum_ms + ?", record.TtftSumMs),
			"ttft_count":       gorm.Expr("perf_metrics.ttft_count + ?", record.TtftCount),
			"output_tokens":    gorm.Expr("perf_metrics.output_tokens + ?", record.OutputTokens),
			"generation_ms":    gorm.Expr("perf_metrics.generation_ms + ?", record.GenerationMs),
		}),
	}).Create(record).Error
}

func getPerfMetrics(modelName string, group string, startTs int64, endTs int64) ([]perfMetricRecord, error) {
	var metrics []perfMetricRecord
	query := platformdb.DB.Model(&perfMetricRecord{}).
		Where("model_name = ? AND bucket_ts >= ? AND bucket_ts <= ?", modelName, startTs, endTs)
	if group != "" {
		query = query.Where(groupColumnName()+" = ?", group)
	}
	err := query.Order("bucket_ts ASC").Find(&metrics).Error
	return metrics, err
}

func getPerfMetricsSummaryAll(startTs int64, endTs int64) ([]perfMetricSummaryRow, error) {
	var summaries []perfMetricSummaryRow
	err := platformdb.DB.Model(&perfMetricRecord{}).
		Select("model_name, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs).
		Group("model_name").
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func getPerfMetricsSummaryByGroups(startTs int64, endTs int64, groups []string) ([]perfMetricSummaryRow, error) {
	var summaries []perfMetricSummaryRow
	query := platformdb.DB.Model(&perfMetricRecord{}).
		Select(groupColumnName()+" as "+groupColumnName()+", SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if len(groups) > 0 {
		query = query.Where(groupColumnName()+" IN ?", groups)
	}
	err := query.Group(groupColumnName()).
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func getPerfMetricsSummaryByGroupModels(startTs int64, endTs int64, groups []string) ([]perfMetricSummaryRow, error) {
	var summaries []perfMetricSummaryRow
	query := platformdb.DB.Model(&perfMetricRecord{}).
		Select("model_name, "+groupColumnName()+" as "+groupColumnName()+", SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if len(groups) > 0 {
		query = query.Where(groupColumnName()+" IN ?", groups)
	}
	err := query.Group(groupColumnName() + ", model_name").
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func getPerfMetricsBucketsByGroups(startTs int64, endTs int64, groups []string) ([]perfMetricRecord, error) {
	var metrics []perfMetricRecord
	query := platformdb.DB.Model(&perfMetricRecord{}).
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if len(groups) > 0 {
		query = query.Where(groupColumnName()+" IN ?", groups)
	}
	err := query.Order(groupColumnName() + " ASC, model_name ASC, bucket_ts ASC").
		Find(&metrics).Error
	return metrics, err
}

func deletePerfMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return platformdb.DB.Where("bucket_ts < ?", cutoffTs).Delete(&perfMetricRecord{}).Error
}

func perfMetricStartTime(hours int) int64 {
	if hours <= 0 {
		hours = 24
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
}

func groupColumnName() string {
	if platformdb.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}
