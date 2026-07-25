package app

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestBillingSessionRefundSyncRestoresWalletAndTokenPreConsume(t *testing.T) {
	truncate(t)
	seedUser(t, 1001, 10000)
	seedToken(t, 2001, 1001, "sk-refund", 10000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          1001,
		UserQuota:       10000,
		TokenId:         2001,
		TokenKey:        "sk-refund",
		OriginModelName: "gpt-5",
		RequestId:       "req-refund",
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 3000)
	require.Nil(t, apiErr)
	require.Equal(t, 3000, session.GetPreConsumedQuota())

	userQuota, err := identitystore.LoadUserQuota(1001, false)
	require.NoError(t, err)
	require.Equal(t, 7000, userQuota)
	require.Equal(t, int64(7000), loadBillingSnapshot(t, 1001, "wallet").AvailableBalance)

	require.NoError(t, session.RefundSync(ctx))

	userQuota, err = identitystore.LoadUserQuota(1001, false)
	require.NoError(t, err)
	require.Equal(t, 10000, userQuota)
	require.Equal(t, int64(10000), loadBillingSnapshot(t, 1001, "wallet").AvailableBalance)
}

func TestBillingSessionSettleAdjustsWalletAndTokenToActualUsage(t *testing.T) {
	truncate(t)
	seedUser(t, 1002, 10000)
	seedToken(t, 2002, 1002, "sk-settle", 10000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          1002,
		UserQuota:       10000,
		TokenId:         2002,
		TokenKey:        "sk-settle",
		OriginModelName: "gpt-5",
		RequestId:       "req-settle",
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 3000)
	require.Nil(t, apiErr)

	require.NoError(t, session.Settle(4500))

	userQuota, err := identitystore.LoadUserQuota(1002, false)
	require.NoError(t, err)
	require.Equal(t, 5500, userQuota)
	snapshot := loadBillingSnapshot(t, 1002, "wallet")
	require.Equal(t, int64(5500), snapshot.AvailableBalance)
	require.Equal(t, int64(4500), snapshot.ConsumedTotal)
	require.NoError(t, session.RefundSync(ctx))
	userQuota, err = identitystore.LoadUserQuota(1002, false)
	require.NoError(t, err)
	require.Equal(t, 5500, userQuota)
}

func TestBillingSessionUsesOneReservationForRelayLifecycle(t *testing.T) {
	truncate(t)
	seedUser(t, 1012, 10000)
	seedToken(t, 2012, 1012, "sk-ledger-lifecycle", 10000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          1012,
		TokenId:         2012,
		TokenKey:        "sk-ledger-lifecycle",
		OriginModelName: "gpt-5",
		RequestId:       "req-ledger-lifecycle",
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	session, apiErr := NewBillingSession(ctx, info, 3000)
	require.Nil(t, apiErr)
	funding, ok := session.funding.(*LedgerRelayFunding)
	require.True(t, ok)
	require.NotEmpty(t, funding.ReservationID())

	var reservation billingschema.BillingReservation
	require.NoError(t, platformdb.DB.Where("reservation_id = ?", funding.ReservationID()).First(&reservation).Error)
	require.Equal(t, "req-ledger-lifecycle", reservation.RequestID)
	require.Equal(t, billingschema.BillingReservationStatusOpen, reservation.Status)

	require.NoError(t, session.Settle(2500))
	require.NotEmpty(t, funding.SettlementID())
	require.NoError(t, platformdb.DB.Where("reservation_id = ?", funding.ReservationID()).First(&reservation).Error)
	require.Equal(t, billingschema.BillingReservationStatusSettled, reservation.Status)

	var settlement billingschema.BillingSettlement
	require.NoError(t, platformdb.DB.Where("settlement_id = ?", funding.SettlementID()).First(&settlement).Error)
	require.Equal(t, funding.ReservationID(), settlement.ReservationID)
	require.EqualValues(t, 2500, settlement.ActualAmount)
}

func TestBillingSessionSettlesExactPreConsumeIntoLedger(t *testing.T) {
	truncate(t)
	seedUser(t, 1014, 10_000)
	seedToken(t, 2014, 1014, "sk-exact-ledger-settle", 10_000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 1014, TokenId: 2014, TokenKey: "sk-exact-ledger-settle", OriginModelName: "gpt-5",
		RequestId: "req-exact-ledger-settle", IsPlayground: true, ForcePreConsume: true,
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 3_000)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(3_000))

	funding := session.funding.(*LedgerRelayFunding)
	var reservation billingschema.BillingReservation
	require.NoError(t, platformdb.DB.Where("reservation_id = ?", funding.ReservationID()).First(&reservation).Error)
	require.Equal(t, billingschema.BillingReservationStatusSettled, reservation.Status)
	var settlement billingschema.BillingSettlement
	require.NoError(t, platformdb.DB.Where("reservation_id = ?", funding.ReservationID()).First(&settlement).Error)
	require.EqualValues(t, 3_000, settlement.ActualAmount)
}

func TestTrustedBillingSessionCreatesReservationAtSettlement(t *testing.T) {
	truncate(t)
	const trustedQuota = 6_000_000
	seedUser(t, 1015, trustedQuota)
	require.NoError(t, platformdb.DB.Create(&identityschema.Token{
		Id: 2015, UserId: 1015, Key: "sk-trusted-ledger-settle", Name: "trusted", Status: constant.TokenStatusEnabled,
		UnlimitedQuota: true,
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 1015, UserQuota: trustedQuota, TokenId: 2015, TokenKey: "sk-trusted-ledger-settle", TokenUnlimited: true,
		OriginModelName: "gpt-5", RequestId: "req-trusted-ledger-settle", IsPlayground: true,
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 1_000)
	require.Nil(t, apiErr)
	require.Equal(t, 0, session.GetPreConsumedQuota())
	require.NoError(t, session.Settle(1_000))

	funding := session.funding.(*LedgerRelayFunding)
	var reservation billingschema.BillingReservation
	require.NoError(t, platformdb.DB.Where("reservation_id = ?", funding.ReservationID()).First(&reservation).Error)
	require.Equal(t, billingschema.BillingReservationStatusSettled, reservation.Status)
	var settlement billingschema.BillingSettlement
	require.NoError(t, platformdb.DB.Where("reservation_id = ?", funding.ReservationID()).First(&settlement).Error)
	require.EqualValues(t, 1_000, settlement.ActualAmount)
}

func TestBillingSessionRequestReplayDoesNotProjectWalletTwice(t *testing.T) {
	truncate(t)
	seedUser(t, 1013, 10000)
	seedToken(t, 2013, 1013, "sk-ledger-replay", 10000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          1013,
		TokenId:         2013,
		TokenKey:        "sk-ledger-replay",
		OriginModelName: "gpt-5",
		RequestId:       "req-ledger-replay",
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	first, apiErr := NewBillingSession(ctx, info, 3000)
	require.Nil(t, apiErr)
	second, apiErr := NewBillingSession(ctx, info, 3000)
	require.Nil(t, apiErr)

	firstFunding := first.funding.(*LedgerRelayFunding)
	secondFunding := second.funding.(*LedgerRelayFunding)
	require.Equal(t, firstFunding.ReservationID(), secondFunding.ReservationID())
	userQuota, err := identitystore.LoadUserQuota(1013, false)
	require.NoError(t, err)
	require.Equal(t, 7000, userQuota)
}

func TestPreConsumeRelayBillingReusesSessionAcrossChannelRetry(t *testing.T) {
	truncate(t)
	seedUser(t, 1016, 10_000)
	seedToken(t, 2016, 1016, "sk-retry-billing", 10_000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          1016,
		TokenId:         2016,
		TokenKey:        "sk-retry-billing",
		OriginModelName: "gpt-5",
		RequestId:       "req-retry-billing",
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	require.Nil(t, PreConsumeRelayBilling(ctx, 3_000, info))
	firstSession := info.Billing
	require.NotNil(t, firstSession)
	require.Nil(t, PreConsumeRelayBilling(ctx, 2_000, info))
	require.Same(t, firstSession, info.Billing)

	quota, err := identitystore.LoadUserQuota(1016, false)
	require.NoError(t, err)
	require.Equal(t, 7_000, quota)
	require.NoError(t, RefundRelayBillingSync(ctx, info))

	quota, err = identitystore.LoadUserQuota(1016, false)
	require.NoError(t, err)
	require.Equal(t, 10_000, quota)
}

func TestNewBillingSessionReturnsLocalWalletQuotaMessage(t *testing.T) {
	truncate(t)
	seedUser(t, 1003, 750)
	seedToken(t, 2003, 1003, "sk-wallet-insufficient", 10000)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          1003,
		TokenId:         2003,
		TokenKey:        "sk-wallet-insufficient",
		OriginModelName: "gpt-5",
		RequestId:       "req-wallet-insufficient",
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 2364)
	require.Nil(t, session)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	require.Equal(t,
		"站内余额不足, 当前余额: "+logger.FormatQuota(750)+", 本次所需: "+logger.FormatQuota(2364),
		apiErr.Error(),
	)
}

func TestFundingInsufficientErrorUsesFundingSource(t *testing.T) {
	subscriptionErr := newFundingInsufficientError(BillingSourceSubscription, billingdomain.ErrInsufficientBalance)
	require.NotNil(t, subscriptionErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, subscriptionErr.GetErrorCode())
	require.Contains(t, subscriptionErr.Error(), "subscription quota insufficient")
}

func loadBillingSnapshot(t *testing.T, userID int, accountType string) *billingschema.BillingBalanceSnapshot {
	t.Helper()
	var account billingschema.BillingAccount
	require.NoError(t, platformdb.DB.Where("owner_type = ? AND owner_id = ? AND account_type = ?", "user", userID, accountType).First(&account).Error)

	var snapshot billingschema.BillingBalanceSnapshot
	require.NoError(t, platformdb.DB.Where("account_id = ?", account.AccountID).First(&snapshot).Error)
	return &snapshot
}
