package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func completeStripeTopUpForAdjustmentTest(t *testing.T, userID int, tradeNo string, quota int64, amountMinor int64) *TopUp {
	t.Helper()
	topUp := &TopUp{
		UserId:              userID,
		Amount:              quota,
		Money:               float64(amountMinor) / 100,
		TradeNo:             tradeNo,
		PaymentMethod:       PaymentMethodStripe,
		PaymentProvider:     PaymentProviderStripe,
		ProviderOrderId:     "cs_" + tradeNo,
		ProviderProductId:   "price_" + tradeNo,
		CreditedQuota:       quota,
		ExpectedAmountMinor: amountMinor,
		ExpectedCurrency:    "CNY",
		Status:              common.TopUpStatusPending,
		CreateTime:          time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())
	require.NoError(t, Recharge(tradeNo, StripeTopUpSettlement{
		CustomerId:      "cus_" + tradeNo,
		PaymentIntentId: "pi_" + tradeNo,
		ChargeId:        "ch_" + tradeNo,
		AmountMinor:     amountMinor,
		Currency:        "CNY",
	}, "127.0.0.1"))
	return GetTopUpByTradeNo(tradeNo)
}

func applyStripeTopUpAdjustmentForTest(t *testing.T, topUp *TopUp, objectType string, objectID string, amountMinor int64, active bool, priority int, created int64) *StripePaymentAdjustmentResult {
	t.Helper()
	require.NotNil(t, topUp)
	result, err := ApplyStripePaymentAdjustment(StripePaymentAdjustmentInput{
		ObjectType:      objectType,
		ObjectId:        objectID,
		EventId:         fmt.Sprintf("evt_%s_%d_%d", objectID, priority, created),
		PaymentIntentId: topUp.ProviderPaymentIntent,
		ChargeId:        topUp.ProviderChargeId,
		AmountMinor:     amountMinor,
		Currency:        topUp.ExpectedCurrency,
		Livemode:        topUp.ProviderLivemode,
		Status:          map[bool]string{true: "active", false: "inactive"}[active],
		Active:          active,
		EventCreated:    created,
		EventPriority:   priority,
	})
	require.NoError(t, err)
	return result
}

func getStripeAdjustmentUserForTest(t *testing.T, userID int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.Where("id = ?", userID).First(&user).Error)
	return user
}

func getStripeRecoveryForTest(t *testing.T, targetKind string, targetID int) StripePaymentRecovery {
	t.Helper()
	var recovery StripePaymentRecovery
	require.NoError(t, DB.Where("target_kind = ? AND target_id = ?", targetKind, targetID).First(&recovery).Error)
	return recovery
}

func TestApplyStripePaymentAdjustmentUsesLargestRefundOrDisputeLoss(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 801, 0)
	topUp := completeStripeTopUpForAdjustmentTest(t, 801, "stripe-adjustment-overlap", 100, 100)

	result := applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentRefund, "re_overlap", 40, true, 3, 100)
	assert.Equal(t, int64(40), result.RecoveredQuota)
	assert.Equal(t, 60, getStripeAdjustmentUserForTest(t, 801).Quota)

	result = applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentDispute, "dp_overlap", 70, true, 2, 200)
	assert.Equal(t, int64(70), result.RecoveredQuota)
	assert.Equal(t, 30, getStripeAdjustmentUserForTest(t, 801).Quota)

	result = applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentRefund, "re_overlap_2", 60, true, 3, 300)
	assert.Equal(t, int64(100), result.RecoveredQuota)
	assert.Zero(t, getStripeAdjustmentUserForTest(t, 801).Quota)

	result = applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentRefund, "re_overlap_2", 60, true, 3, 300)
	assert.Equal(t, int64(100), result.RecoveredQuota)
	assert.Zero(t, getStripeAdjustmentUserForTest(t, 801).Quota)
}

func TestStripeAdjustmentRecoveryNetsDebtAndIgnoresLateLowerPriorityEvents(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 802, 0)
	topUp := completeStripeTopUpForAdjustmentTest(t, 802, "stripe-adjustment-debt", 100, 100)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 802).Update("quota", 20).Error)

	result := applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentRefund, "re_debt", 80, true, 3, 100)
	assert.Equal(t, int64(80), result.RecoveredQuota)
	assert.Equal(t, int64(60), result.OutstandingQuota)
	user := getStripeAdjustmentUserForTest(t, 802)
	assert.Zero(t, user.Quota)
	assert.Equal(t, int64(60), user.BillingDebt)

	completeStripeTopUpForAdjustmentTest(t, 802, "stripe-adjustment-debt-payment", 60, 60)
	user = getStripeAdjustmentUserForTest(t, 802)
	assert.Zero(t, user.Quota)
	assert.Zero(t, user.BillingDebt)
	recovery := getStripeRecoveryForTest(t, StripePaymentTargetTopUp, topUp.Id)
	assert.Zero(t, recovery.OutstandingQuota)
	assert.Equal(t, int64(60), recovery.DebtPaidQuota)

	result = applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentRefund, "re_debt", 80, false, 5, 200)
	assert.Zero(t, result.RecoveredQuota)
	user = getStripeAdjustmentUserForTest(t, 802)
	assert.Equal(t, 80, user.Quota)
	assert.Zero(t, user.BillingDebt)

	result = applyStripeTopUpAdjustmentForTest(t, topUp, StripeAdjustmentRefund, "re_debt", 80, true, 3, 300)
	assert.Zero(t, result.RecoveredQuota)
	user = getStripeAdjustmentUserForTest(t, 802)
	assert.Equal(t, 80, user.Quota)
	assert.Zero(t, user.BillingDebt)
	var adjustment StripePaymentAdjustment
	require.NoError(t, DB.Where("object_type = ? AND object_id = ?", StripeAdjustmentRefund, "re_debt").First(&adjustment).Error)
	assert.False(t, adjustment.Active)
	assert.Equal(t, 5, adjustment.EventPriority)
}

func TestRefundingDebtPaymentTopUpRecreatesMatchingDebt(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 803, 0)
	firstTopUp := completeStripeTopUpForAdjustmentTest(t, 803, "stripe-adjustment-original", 100, 100)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 803).Update("quota", 0).Error)
	applyStripeTopUpAdjustmentForTest(t, firstTopUp, StripeAdjustmentRefund, "re_original", 100, true, 3, 100)

	secondTopUp := completeStripeTopUpForAdjustmentTest(t, 803, "stripe-adjustment-debt-topup", 60, 60)
	user := getStripeAdjustmentUserForTest(t, 803)
	assert.Zero(t, user.Quota)
	assert.Equal(t, int64(40), user.BillingDebt)

	result := applyStripeTopUpAdjustmentForTest(t, secondTopUp, StripeAdjustmentRefund, "re_debt_topup", 60, true, 3, 200)
	assert.Equal(t, int64(60), result.RecoveredQuota)
	assert.Equal(t, int64(60), result.OutstandingQuota)
	user = getStripeAdjustmentUserForTest(t, 803)
	assert.Zero(t, user.Quota)
	assert.Equal(t, int64(100), user.BillingDebt)
}

func TestUnlimitedStripeSubscriptionOnlyRevokesOnFullPaymentLoss(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 804, 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 804).Update("group", "vip").Error)
	subscription := UserSubscription{
		UserId: 804, PlanId: 1, AmountTotal: 0, StartTime: time.Now().Add(-time.Hour).Unix(),
		EndTime: time.Now().Add(time.Hour).Unix(), Status: "active", Source: "purchase",
		UpgradeGroup: "vip", PrevUserGroup: "default",
	}
	require.NoError(t, DB.Create(&subscription).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return registerStripePaymentReferenceTx(tx, StripePaymentReference{
			PaymentIntentId: common.GetPointer("pi_subscription_adjustment"),
			ChargeId:        common.GetPointer("ch_subscription_adjustment"),
			TargetKind:      StripePaymentTargetSubscriptionEntitlement, TargetId: subscription.Id, UserId: 804,
			AmountMinor: 100, Currency: "CNY",
		}, 100, 0)
	}))

	apply := func(objectID string, amount int64, active bool, priority int, created int64) {
		t.Helper()
		_, err := ApplyStripePaymentAdjustment(StripePaymentAdjustmentInput{
			ObjectType: StripeAdjustmentRefund, ObjectId: objectID, EventId: fmt.Sprintf("evt_%s_%d", objectID, priority),
			PaymentIntentId: "pi_subscription_adjustment", ChargeId: "ch_subscription_adjustment",
			AmountMinor: amount, Currency: "CNY", Status: map[bool]string{true: "succeeded", false: "failed"}[active],
			Active: active, EventCreated: created, EventPriority: priority,
		})
		require.NoError(t, err)
	}

	apply("re_subscription_partial", 40, true, 3, 100)
	require.NoError(t, DB.Where("id = ?", subscription.Id).First(&subscription).Error)
	assert.Equal(t, "active", subscription.Status)
	assert.Equal(t, "vip", getStripeAdjustmentUserForTest(t, 804).Group)

	apply("re_subscription_remainder", 60, true, 3, 200)
	require.NoError(t, DB.Where("id = ?", subscription.Id).First(&subscription).Error)
	assert.Equal(t, "cancelled", subscription.Status)
	assert.Equal(t, "default", getStripeAdjustmentUserForTest(t, 804).Group)

	apply("re_subscription_partial", 40, false, 5, 300)
	require.NoError(t, DB.Where("id = ?", subscription.Id).First(&subscription).Error)
	assert.Equal(t, "active", subscription.Status)
	assert.Equal(t, "vip", getStripeAdjustmentUserForTest(t, 804).Group)
}
