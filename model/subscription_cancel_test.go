package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdminCancelSubscriptionEndsAccessWithoutRefundingPayment(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 806, 50)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 806).Update("group", "vip").Error)
	now := time.Now().Unix()
	sub := UserSubscription{
		UserId: 806, PlanId: 1, Status: "active", Source: "order",
		StartTime: now - 60, EndTime: now + 3600, AmountTotal: 1000, AmountUsed: 100,
		UpgradeGroup: "vip", PrevUserGroup: "default", DowngradeGroup: "default",
	}
	require.NoError(t, DB.Create(&sub).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return registerStripeSubscriptionEntitlementPaymentTx(tx, &sub, StripeOneTimeSubscriptionPayment{
			PaymentIntentId: "pi_admin_cancel", ChargeId: "ch_admin_cancel", AmountMinor: 100, Currency: "CNY",
		})
	}))

	_, err := AdminInvalidateUserSubscription(sub.Id)
	require.NoError(t, err)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	assert.Equal(t, "cancelled", sub.Status)
	assert.LessOrEqual(t, sub.EndTime, common.GetTimestamp())
	assert.Equal(t, int64(1000), sub.AmountTotal)
	assert.Equal(t, int64(100), sub.AmountUsed)
	active, err := GetAllActiveUserSubscriptions(806)
	require.NoError(t, err)
	assert.Empty(t, active)
	user := getStripeAdjustmentUserForTest(t, 806)
	assert.Equal(t, "default", user.Group)
	assert.Equal(t, 50, user.Quota)
	assert.Zero(t, user.BillingDebt)
	var references, adjustments int64
	require.NoError(t, DB.Model(&StripePaymentReference{}).Where("target_id = ?", sub.Id).Count(&references).Error)
	require.NoError(t, DB.Model(&StripePaymentAdjustment{}).Count(&adjustments).Error)
	assert.Equal(t, int64(1), references)
	assert.Zero(t, adjustments)

	// A later refund reversal must not reactivate an administrator-cancelled entitlement.
	input := StripePaymentAdjustmentInput{
		ObjectType: StripeAdjustmentRefund, ObjectId: "re_admin_cancel", EventId: "evt_admin_cancel_refund",
		PaymentIntentId: "pi_admin_cancel", ChargeId: "ch_admin_cancel", AmountMinor: 100, Currency: "CNY",
		Status: "succeeded", Active: true, EventCreated: 100, EventPriority: 3,
	}
	_, err = ApplyStripePaymentAdjustment(input)
	require.NoError(t, err)
	input.EventId, input.Status, input.Active = "evt_admin_cancel_reversed", "failed", false
	input.EventCreated, input.EventPriority = 200, 5
	_, err = ApplyStripePaymentAdjustment(input)
	require.NoError(t, err)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	assert.Equal(t, "cancelled", sub.Status)
	assert.Equal(t, "default", getStripeAdjustmentUserForTest(t, 806).Group)
}
