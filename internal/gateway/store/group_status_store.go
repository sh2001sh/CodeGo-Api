package store

import (
	"fmt"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"

	"os"
	"strings"
)

func ListGroupStatusGroups() ([]string, error) {
	groupColumn := abilityGroupColumn()

	var groups []string
	err := platformdb.DB.Table("abilities").
		Select(groupColumn).
		Distinct().
		Where(groupColumn+" <> ''").
		Order(groupColumn+" ASC").
		Pluck(groupColumn, &groups).Error
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(groups))
	for _, groupName := range groups {
		if strings.TrimSpace(groupName) == "" || groupName == "auto" {
			continue
		}
		filtered = append(filtered, groupName)
	}
	return filtered, nil
}

func LoadGroupModelRequestBuckets(startTime int64, endTime int64, bucketSize int64, groups []string) ([]GroupModelRequestBucket, error) {
	if endTime <= startTime {
		return []GroupModelRequestBucket{}, nil
	}
	if bucketSize <= 0 {
		bucketSize = 60
	}

	filteredGroups := make([]string, 0, len(groups))
	for _, groupName := range groups {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" || groupName == "auto" {
			continue
		}
		filteredGroups = append(filteredGroups, groupName)
	}

	groupColumn := logGroupColumn()
	bucketExpr := logBucketIndexExpr(startTime, bucketSize)
	selectExpr := fmt.Sprintf(
		"%s as group_name, model_name, %s as bucket_index, COUNT(*) as request_count, SUM(CASE WHEN type = %d THEN 1 ELSE 0 END) as success_count",
		groupColumn,
		bucketExpr,
		auditschema.LogTypeConsume,
	)

	query := platformdb.LogDB.Table("logs").
		Select(selectExpr).
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Where("type IN ?", []int{auditschema.LogTypeConsume, auditschema.LogTypeError}).
		Where("model_name <> ''").
		Where(groupColumn + " <> ''")

	if len(filteredGroups) > 0 {
		query = query.Where(groupColumn+" IN ?", filteredGroups)
	}

	var rows []GroupModelRequestBucket
	err := query.
		Group(fmt.Sprintf("%s, model_name, %s", groupColumn, bucketExpr)).
		Order("group_name ASC, model_name ASC, bucket_index ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func logGroupColumn() string {
	if os.Getenv("LOG_SQL_DSN") != "" {
		if platformdb.LogSQLType == platformdb.DatabaseTypePostgreSQL {
			return `"group"`
		}
		return "`group`"
	}
	return abilityGroupColumn()
}

func logBucketIndexExpr(startTime int64, bucketSize int64) string {
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch platformdb.LogSQLType {
		case platformdb.DatabaseTypeMySQL:
			return fmt.Sprintf("FLOOR((created_at - %d) / %d)", startTime, bucketSize)
		case platformdb.DatabaseTypePostgreSQL:
			return fmt.Sprintf("CAST((created_at - %d) / %d AS BIGINT)", startTime, bucketSize)
		default:
			return fmt.Sprintf("CAST((created_at - %d) / %d AS INTEGER)", startTime, bucketSize)
		}
	}

	if platformdb.UsingMySQL {
		return fmt.Sprintf("FLOOR((created_at - %d) / %d)", startTime, bucketSize)
	}
	if platformdb.UsingPostgreSQL {
		return fmt.Sprintf("CAST((created_at - %d) / %d AS BIGINT)", startTime, bucketSize)
	}
	return fmt.Sprintf("CAST((created_at - %d) / %d AS INTEGER)", startTime, bucketSize)
}
