package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLedgerSyncRequestIDFitsReservationColumn(t *testing.T) {
	operationID := "wallet-debit:" + strings.Repeat("a", 64) + ":debit"

	requestID := ledgerSyncRequestID(operationID)

	require.LessOrEqual(t, len(requestID), billingReservationRequestIDMax)
	require.Equal(t, requestID, ledgerSyncRequestID(operationID))
	require.NotEqual(t, "ledger-sync:"+operationID, requestID)
}
