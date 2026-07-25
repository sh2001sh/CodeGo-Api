package projection

import (
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"gorm.io/gorm"
	"os"
	"strings"
)

func formatUserLogs(logs []*auditschema.Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		otherMap, _ := platformtext.StrToMap(logs[i].Other)
		if otherMap != nil {
			delete(otherMap, "admin_info")
			delete(otherMap, "stream_status")
		}
		logs[i].Other = platformtext.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func logContainsPattern(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}

	replacer := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	return "%" + replacer.Replace(input) + "%", true
}

func applyLogContainsFilter(tx *gorm.DB, column string, value string) *gorm.DB {
	pattern, ok := logContainsPattern(value)
	if !ok {
		return tx
	}
	return tx.Where(column+" LIKE ? ESCAPE '!'", pattern)
}

func logGroupColumn() string {
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch platformdb.LogSQLType {
		case platformdb.DatabaseTypePostgreSQL:
			return `"group"`
		default:
			return "`group`"
		}
	}
	if platformdb.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}
