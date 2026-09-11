package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripeWalletCapacityUsesNetCreditAndKeepsChargeLimit(t *testing.T) {
	for _, tc := range []struct {
		name         string
		balance      int
		debt, credit int64
		want         error
	}{
		{"above int32", common.MaxQuota, 0, 10_000_000, nil},
		{"exact wallet limit", common.MaxWalletQuota - 100, 0, 100, nil},
		{"over wallet limit", common.MaxWalletQuota - 100, 0, 101, ErrTopUpQuotaLimitExceeded},
		{"debt absorbs credit", common.MaxWalletQuota, 100, 100, nil},
		{"partial debt", common.MaxWalletQuota - 50, 50, 100, nil},
		{"negative debt", 0, -1, 100, ErrInvalidTopUpQuota},
		{"oversized charge", 0, 0, int64(common.MaxQuota) + 1, ErrInvalidTopUpQuota},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStripeTopUpQuotaCapacity(&User{Quota: tc.balance, BillingDebt: tc.debt}, tc.credit)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStripeLargeWalletSettlesOnceAndRestoresDispute(t *testing.T) {
	truncateTables(t)
	const balance = 2_500_000_000
	const credit = 10_000_000
	insertUserForPaymentGuardTest(t, 960, balance)
	topUp := completeStripeTopUpForAdjustmentTest(t, 960, "large-wallet", credit, 2000)
	assert.Equal(t, balance+credit, getUserQuotaForPaymentGuardTest(t, 960))
	require.NoError(t, Recharge(topUp.TradeNo, StripeTopUpSettlement{
		CustomerId: topUp.ProviderCustomerId, PaymentIntentId: topUp.ProviderPaymentIntent,
		ChargeId: topUp.ProviderChargeId, AmountMinor: 2000, Currency: "CNY",
	}, "127.0.0.1"))
	assert.Equal(t, balance+credit, getUserQuotaForPaymentGuardTest(t, 960))
	applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentDispute, "dp_large", 2000, true, 2, 100)
	assert.Equal(t, balance, getUserQuotaForPaymentGuardTest(t, 960))
	applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentDispute, "dp_large", 2000, false, 3, 200)
	assert.Equal(t, balance+credit, getUserQuotaForPaymentGuardTest(t, 960))
}
