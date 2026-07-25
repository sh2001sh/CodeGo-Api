package activities

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGatewayActivitiesPersistRequestExecutionIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	originalDB := platformdb.DB
	originalSQLite := platformdb.UsingSQLite
	originalPostgreSQL := platformdb.UsingPostgreSQL
	t.Cleanup(func() {
		platformdb.DB = originalDB
		platformdb.UsingSQLite = originalSQLite
		platformdb.UsingPostgreSQL = originalPostgreSQL
	})
	platformdb.DB = db
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false
	require.NoError(t, db.AutoMigrate(&gatewayschema.RequestExecution{}, &gatewayschema.GatewayRoutePlan{}, &gatewayschema.ExecutionAttempt{}, &gatewayschema.UsageEvidence{}))

	input := contracts.RequestSettlementWorkflowInput{RequestID: "request-execution-1", TraceID: "trace-execution-1", UserID: 42, TokenID: 7, AccountID: "account-1", ReservationID: "reservation-1", SettlementID: "settlement-1", ActualAmount: 120}
	activities := &GatewayActivities{}
	first, err := activities.CreateRequestExecution(context.Background(), input)
	require.NoError(t, err)
	second, err := activities.CreateRequestExecution(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first.ExecutionID, second.ExecutionID)
	require.NoError(t, activities.ExecuteProviderRequest(context.Background(), input))
	_, err = activities.CollectUsageEvidence(context.Background(), input)
	require.NoError(t, err)
	_, err = activities.CollectUsageEvidence(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, activities.PublishRequestSettledEvent(context.Background(), input))

	var executions []gatewayschema.RequestExecution
	require.NoError(t, db.Find(&executions).Error)
	require.Len(t, executions, 1)
	require.Equal(t, gatewayschema.RequestExecutionStatusSettled, executions[0].Status)
	require.Equal(t, input.TraceID, executions[0].TraceID)
	var evidence []gatewayschema.UsageEvidence
	require.NoError(t, db.Find(&evidence).Error)
	require.Len(t, evidence, 1)
	require.Equal(t, input.TraceID, evidence[0].TraceID)
}
