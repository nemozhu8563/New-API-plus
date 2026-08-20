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

func TestStripeSubscriptionAdjustmentAggregatesLossAcrossPaymentReferences(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 805, 0)
	now := time.Now()
	order := SubscriptionOrder{
		UserId: 805, PlanId: 1, PlanTitle: "Multi-payment", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, ProviderCustomerId: "cus_subscription_multi_payment",
		ProviderSubscriptionId: common.GetPointer("sub_multi_payment"), Status: common.TopUpStatusSuccess,
		TradeNo: "stripe-subscription-multi-payment", CreateTime: now.Unix(),
	}
	require.NoError(t, DB.Create(&order).Error)
	subscription := UserSubscription{
		UserId: 805, PlanId: 1, AmountTotal: 1000, StartTime: now.Add(-time.Hour).Unix(),
		EndTime: now.Add(time.Hour).Unix(), Status: "active", Source: "stripe_invoice",
		Provider: PaymentProviderStripe, ProviderSubscriptionId: "sub_multi_payment",
		ProviderInvoiceId: "in_multi_payment",
	}
	require.NoError(t, DB.Create(&subscription).Error)
	settlement := StripeSubscriptionSettlement{
		InvoiceId: "in_multi_payment", SubscriptionOrderId: order.Id, UserSubscriptionId: subscription.Id,
		ProviderCustomerId: "cus_subscription_multi_payment", ProviderSubscriptionId: "sub_multi_payment",
		ProviderProductId: "price_subscription_multi_payment", Quantity: 1, UnitAmountMinor: 100,
		InvoiceTotalMinor: 100, AmountPaidMinor: 100, Currency: "CNY",
		PeriodStart: subscription.StartTime, PeriodEnd: subscription.EndTime, CreatedAt: now.Unix(),
	}
	require.NoError(t, DB.Create(&settlement).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return registerStripeSubscriptionPaymentsTx(tx, &settlement, &subscription, []StripePaymentSnapshot{
			{PaymentIntentId: "pi_multi_payment_1", ChargeId: "ch_multi_payment_1", AmountMinor: 60},
			{PaymentIntentId: "pi_multi_payment_2", ChargeId: "ch_multi_payment_2", AmountMinor: 40},
		})
	}))

	testCases := []struct {
		name             string
		objectType       string
		objectID         string
		paymentIntentID  string
		chargeID         string
		amountMinor      int64
		expectedRecovery int64
		expectedTotal    int64
		expectedStatus   string
	}{
		{
			name: "first refund on first payment", objectType: StripeAdjustmentRefund, objectID: "re_multi_1a",
			paymentIntentID: "pi_multi_payment_1", chargeID: "ch_multi_payment_1", amountMinor: 40,
			expectedRecovery: 400, expectedTotal: 600, expectedStatus: "active",
		},
		{
			name: "refund sum is capped at first payment", objectType: StripeAdjustmentRefund, objectID: "re_multi_1b",
			paymentIntentID: "pi_multi_payment_1", chargeID: "ch_multi_payment_1", amountMinor: 30,
			expectedRecovery: 600, expectedTotal: 400, expectedStatus: "active",
		},
		{
			name: "dispute does not stack on refunds for first payment", objectType: StripeAdjustmentDispute, objectID: "dp_multi_1",
			paymentIntentID: "pi_multi_payment_1", chargeID: "ch_multi_payment_1", amountMinor: 50,
			expectedRecovery: 600, expectedTotal: 400, expectedStatus: "active",
		},
		{
			name: "loss from second payment is added", objectType: StripeAdjustmentRefund, objectID: "re_multi_2a",
			paymentIntentID: "pi_multi_payment_2", chargeID: "ch_multi_payment_2", amountMinor: 10,
			expectedRecovery: 700, expectedTotal: 300, expectedStatus: "active",
		},
		{
			name: "larger dispute wins within second payment", objectType: StripeAdjustmentDispute, objectID: "dp_multi_2",
			paymentIntentID: "pi_multi_payment_2", chargeID: "ch_multi_payment_2", amountMinor: 30,
			expectedRecovery: 900, expectedTotal: 100, expectedStatus: "active",
		},
		{
			name: "invoice loss is capped at invoice total", objectType: StripeAdjustmentRefund, objectID: "re_multi_2b",
			paymentIntentID: "pi_multi_payment_2", chargeID: "ch_multi_payment_2", amountMinor: 35,
			expectedRecovery: 1000, expectedTotal: 0, expectedStatus: "cancelled",
		},
	}

	for i, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := ApplyStripePaymentAdjustment(StripePaymentAdjustmentInput{
				ObjectType: testCase.objectType, ObjectId: testCase.objectID,
				EventId: fmt.Sprintf("evt_multi_payment_%d", i), PaymentIntentId: testCase.paymentIntentID,
				ChargeId: testCase.chargeID, AmountMinor: testCase.amountMinor, Currency: "CNY",
				Status: "active", Active: true, EventCreated: int64(i + 1), EventPriority: 3,
			})
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedRecovery, result.RecoveredQuota)
			require.NoError(t, DB.Where("id = ?", subscription.Id).First(&subscription).Error)
			assert.Equal(t, testCase.expectedTotal, subscription.AmountTotal)
			assert.Equal(t, testCase.expectedStatus, subscription.Status)
		})
	}
}

func TestUnlimitedStripeSubscriptionOnlyRevokesOnFullPaymentLoss(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 804, 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 804).Update("group", "vip").Error)
	order := SubscriptionOrder{
		UserId: 804, PlanId: 1, PlanTitle: "Unlimited", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, ProviderCustomerId: "cus_subscription_adjustment",
		ProviderSubscriptionId: common.GetPointer("sub_adjustment"), Status: common.TopUpStatusSuccess,
		TradeNo: "stripe-subscription-adjustment", CreateTime: time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&order).Error)
	subscription := UserSubscription{
		UserId: 804, PlanId: 1, AmountTotal: 0, StartTime: time.Now().Add(-time.Hour).Unix(),
		EndTime: time.Now().Add(time.Hour).Unix(), Status: "active", Source: "stripe_invoice",
		UpgradeGroup: "vip", PrevUserGroup: "default", Provider: PaymentProviderStripe,
		ProviderSubscriptionId: "sub_adjustment", ProviderInvoiceId: "in_adjustment",
	}
	require.NoError(t, DB.Create(&subscription).Error)
	settlement := StripeSubscriptionSettlement{
		InvoiceId: "in_adjustment", SubscriptionOrderId: order.Id, UserSubscriptionId: subscription.Id,
		ProviderCustomerId: "cus_subscription_adjustment", ProviderSubscriptionId: "sub_adjustment",
		ProviderProductId: "price_subscription_adjustment", Quantity: 1, UnitAmountMinor: 100,
		InvoiceTotalMinor: 100, AmountPaidMinor: 100, Currency: "CNY",
		PeriodStart: subscription.StartTime, PeriodEnd: subscription.EndTime, CreatedAt: time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&settlement).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return registerStripePaymentReferenceTx(tx, StripePaymentReference{
			PaymentIntentId: common.GetPointer("pi_subscription_adjustment"),
			ChargeId:        common.GetPointer("ch_subscription_adjustment"),
			TargetKind:      StripePaymentTargetSubscription, TargetId: settlement.Id, UserId: 804,
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
