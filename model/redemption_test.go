package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSearchRedemptionsFiltersAndPaginates(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	})

	now := common.GetTimestamp()
	redemptions := []Redemption{
		{Id: 1, Name: "alpha-active", Key: "00000000000000000000000000000001", Status: common.RedemptionCodeStatusEnabled, ExpiredTime: 0},
		{Id: 2, Name: "alpha-future", Key: "00000000000000000000000000000002", Status: common.RedemptionCodeStatusEnabled, ExpiredTime: now + 3600},
		{Id: 3, Name: "alpha-expired", Key: "00000000000000000000000000000003", Status: common.RedemptionCodeStatusEnabled, ExpiredTime: now - 10},
		{Id: 4, Name: "beta-disabled", Key: "00000000000000000000000000000004", Status: common.RedemptionCodeStatusDisabled, ExpiredTime: 0},
		{Id: 5, Name: "beta-used", Key: "00000000000000000000000000000005", Status: common.RedemptionCodeStatusUsed, ExpiredTime: 0},
	}
	require.NoError(t, DB.Create(&redemptions).Error)

	tests := []struct {
		name      string
		keyword   string
		status    string
		startIdx  int
		num       int
		wantTotal int64
		wantIds   []int
	}{
		{
			name:      "no filters returns all rows",
			num:       10,
			wantTotal: 5,
			wantIds:   []int{5, 4, 3, 2, 1},
		},
		{
			name:      "keyword filters by name prefix",
			keyword:   "alpha",
			num:       10,
			wantTotal: 3,
			wantIds:   []int{3, 2, 1},
		},
		{
			name:      "enabled status excludes expired rows",
			status:    "1",
			num:       10,
			wantTotal: 2,
			wantIds:   []int{2, 1},
		},
		{
			name:      "expired status returns enabled expired rows",
			status:    "expired",
			num:       10,
			wantTotal: 1,
			wantIds:   []int{3},
		},
		{
			name:      "disabled status",
			status:    "2",
			num:       10,
			wantTotal: 1,
			wantIds:   []int{4},
		},
		{
			name:      "used status",
			status:    "3",
			num:       10,
			wantTotal: 1,
			wantIds:   []int{5},
		},
		{
			name:      "pagination keeps unpaged total",
			startIdx:  1,
			num:       2,
			wantTotal: 5,
			wantIds:   []int{4, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := SearchRedemptions(tt.keyword, tt.status, tt.startIdx, tt.num)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)
			gotIds := make([]int, 0, len(rows))
			for _, row := range rows {
				gotIds = append(gotIds, row.Id)
			}
			assert.Equal(t, tt.wantIds, gotIds)
		})
	}
}

func setupRedeemFixture(t *testing.T, quota int) (userId int, key string) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&Redemption{},
		&AffiliateAgent{},
		&AffiliateFirstRewardClaim{},
		&AffiliateCommission{},
		&AffiliateConversion{},
		&AffiliateWithdrawal{},
	))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateWithdrawal{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateConversion{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateCommission{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateFirstRewardClaim{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateAgent{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&UserSubscription{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SubscriptionPlan{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateWithdrawal{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateConversion{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateCommission{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateFirstRewardClaim{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateAgent{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&UserSubscription{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SubscriptionPlan{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
		require.NoError(t, DB.Exec("DELETE FROM users").Error)
		require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	})

	user := &User{Username: "redeem-user", Password: "password", Status: common.UserStatusEnabled, Quota: 0}
	require.NoError(t, DB.Create(user).Error)

	key = "10000000000000000000000000000001"
	redemption := &Redemption{
		Name:        "redeem-test",
		Key:         key,
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       quota,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(redemption).Error)
	return user.Id, key
}

func TestRedeemCreditsQuotaExactlyOnce(t *testing.T) {
	userId, key := setupRedeemFixture(t, 500)

	quota, err := Redeem(key, userId)
	require.NoError(t, err)
	assert.Equal(t, 500, quota)

	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 500, user.Quota)

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, "name = ?", "redeem-test").Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, redemption.Status)
	assert.Equal(t, userId, redemption.UsedUserId)

	// Redeeming the same code again must fail and must not credit quota.
	_, err = Redeem(key, userId)
	require.Error(t, err)
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 500, user.Quota)
}

func TestUsedRedemptionUpdateCannotReenable(t *testing.T) {
	tests := []struct {
		name        string
		benefitType string
	}{
		{name: "quota code", benefitType: RedemptionBenefitQuota},
		{name: "subscription code", benefitType: RedemptionBenefitSubscription},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userId, quotaKey := setupRedeemFixture(t, 500)
			key := quotaKey
			if test.benefitType == RedemptionBenefitSubscription {
				plan := &SubscriptionPlan{
					Title:            "Immutable used-code plan",
					Currency:         SubscriptionCurrencyCNY,
					DurationUnit:     SubscriptionDurationDay,
					DurationValue:    7,
					Enabled:          true,
					TotalAmount:      1000,
					QuotaResetPeriod: SubscriptionResetNever,
				}
				require.NoError(t, DB.Create(plan).Error)

				redemption := &Redemption{
					Name:        "immutable-used-subscription-code",
					Key:         "40000000000000000000000000000001",
					CreatedTime: common.GetTimestamp(),
				}
				require.NoError(t, redemption.FreezeSubscriptionPlan(plan.Id))
				require.NoError(t, redemption.Insert())
				key = redemption.Key
			}

			_, err := RedeemCode(key, userId)
			require.NoError(t, err)

			var used Redemption
			require.NoError(t, DB.First(&used, "key = ?", key).Error)
			require.Equal(t, common.RedemptionCodeStatusUsed, used.Status)
			redeemedTime := used.RedeemedTime

			used.Status = common.RedemptionCodeStatusEnabled
			used.RedeemedTime = 0
			require.NoError(t, used.Update())

			var stored Redemption
			require.NoError(t, DB.First(&stored, "key = ?", key).Error)
			assert.Equal(t, common.RedemptionCodeStatusUsed, stored.Status)
			assert.Equal(t, redeemedTime, stored.RedeemedTime)

			stored.Status = common.RedemptionCodeStatusEnabled
			require.ErrorIs(t, stored.UpdateStatus(), ErrRedemptionStatusImmutable)
			require.NoError(t, DB.First(&stored, "key = ?", key).Error)
			assert.Equal(t, common.RedemptionCodeStatusUsed, stored.Status)
		})
	}
}

func TestRedemptionUpdateStatusOnlyAllowsEnabledAndDisabled(t *testing.T) {
	_, key := setupRedeemFixture(t, 500)

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, "key = ?", key).Error)
	redemption.Status = common.RedemptionCodeStatusUsed
	require.ErrorIs(t, redemption.UpdateStatus(), ErrRedemptionStatusInvalid)

	redemption.Status = common.RedemptionCodeStatusDisabled
	require.NoError(t, redemption.UpdateStatus())
	require.NoError(t, DB.First(&redemption, "key = ?", key).Error)
	assert.Equal(t, common.RedemptionCodeStatusDisabled, redemption.Status)

	redemption.Status = common.RedemptionCodeStatusEnabled
	require.NoError(t, redemption.UpdateStatus())
	require.NoError(t, DB.First(&redemption, "key = ?", key).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redemption.Status)
}

// Exactly one of several concurrent redeems of the same code may win, and
// quota must be credited exactly once.
func TestRedeemConcurrentSingleSuccess(t *testing.T) {
	userId, key := setupRedeemFixture(t, 300)

	const goroutines = 5
	successes := make([]bool, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if _, err := Redeem(key, userId); err == nil {
				successes[idx] = true
			}
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, ok := range successes {
		if ok {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount, "exactly one concurrent redeem should succeed")

	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 300, user.Quota, "quota must be credited exactly once")
}

func TestRedeemCodeCreatesParallelSubscriptionFromFrozenSnapshotWithoutCashback(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)

	inviter := &User{
		Username: "subscription-redemption-inviter",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "subscription-redemption-inviter-code",
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Update("inviter_id", inviter.Id).Error)

	allowWalletOverflow := false
	plan := &SubscriptionPlan{
		Title:                   "Frozen monthly plan",
		Currency:                SubscriptionCurrencyCNY,
		DurationUnit:            SubscriptionDurationMonth,
		DurationValue:           1,
		Enabled:                 true,
		AllowWalletOverflow:     &allowWalletOverflow,
		TotalAmount:             12345,
		QuotaResetPeriod:        SubscriptionResetCustom,
		QuotaResetCustomSeconds: 3600,
	}
	require.NoError(t, DB.Create(plan).Error)

	var existing *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		existing, err = CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	}))

	redemption := &Redemption{
		Name:        "frozen-subscription-code",
		Key:         "30000000000000000000000000000001",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.FreezeSubscriptionPlan(plan.Id))
	require.NoError(t, redemption.Insert())

	var storedBeforeRedeem Redemption
	require.NoError(t, DB.First(&storedBeforeRedeem, "key = ?", redemption.Key).Error)
	assert.Zero(t, storedBeforeRedeem.Quota, "subscription codes must not inherit the legacy quota default")

	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"title":                      "Changed plan",
		"total_amount":               99999,
		"quota_reset_custom_seconds": 7200,
		"allow_wallet_overflow":      true,
		"enabled":                    false,
	}).Error)

	result, err := RedeemCode(redemption.Key, userId)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionBenefitSubscription, result.Type)
	assert.Equal(t, plan.Id, result.PlanId)
	assert.Equal(t, "Frozen monthly plan", result.PlanTitle)
	assert.NotZero(t, result.SubscriptionId)

	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Zero(t, user.Quota, "subscription redemption must not credit wallet quota")

	var subscriptions []UserSubscription
	require.NoError(t, DB.Where("user_id = ?", userId).Order("id ASC").Find(&subscriptions).Error)
	require.Len(t, subscriptions, 2, "an existing subscription must remain alongside the redeemed one")
	assert.Equal(t, existing.Id, subscriptions[0].Id)
	redeemedSubscription := subscriptions[1]
	assert.Equal(t, int64(12345), redeemedSubscription.AmountTotal)
	assert.Equal(t, "Frozen monthly plan", redeemedSubscription.PlanTitle)
	assert.Equal(t, int64(3600), redeemedSubscription.QuotaResetCustomSeconds)
	assert.False(t, redeemedSubscription.AllowWalletOverflow)
	assert.Equal(t, "redemption", redeemedSubscription.Source)

	var storedAfterRedeem Redemption
	require.NoError(t, DB.First(&storedAfterRedeem, "key = ?", redemption.Key).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, storedAfterRedeem.Status)
	assert.Equal(t, userId, storedAfterRedeem.UsedUserId)
	assert.Equal(t, redeemedSubscription.Id, storedAfterRedeem.UsedSubscriptionId)

	var commissionCount int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Count(&commissionCount).Error)
	assert.Zero(t, commissionCount, "subscription redemption must not create affiliate cashback")
	var inviterAfter User
	require.NoError(t, DB.First(&inviterAfter, "id = ?", inviter.Id).Error)
	assert.Zero(t, inviterAfter.Quota)
	assert.Zero(t, inviterAfter.AffQuota)

	summary, err := GetAffiliateSummary(inviter.Id)
	require.NoError(t, err)
	assert.Zero(t, summary.RedemptionCount, "subscription codes do not participate in affiliate redemption metrics")

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	invitees, inviteeTotal, err := ListAffiliateInvitees(inviter.Id, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inviteeTotal)
	require.Len(t, invitees, 1)
	assert.Zero(t, invitees[0].RedemptionCount)
	assert.Zero(t, invitees[0].RedeemedQuota)
	assert.Zero(t, invitees[0].RewardQuota)

	invitations, invitationTotal, err := ListAllAffiliateInvitations("", pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), invitationTotal)
	require.Len(t, invitations, 1)
	assert.Zero(t, invitations[0].RedemptionCount)
	assert.Zero(t, invitations[0].RedeemedQuota)
	assert.Zero(t, invitations[0].RewardQuota)

	affiliateRedemptions, affiliateRedemptionTotal, err := ListAffiliateRedemptionsForInviter(inviter.Id, pageInfo)
	require.NoError(t, err)
	assert.Zero(t, affiliateRedemptionTotal)
	assert.Empty(t, affiliateRedemptions)
}

func TestRedeemCodeUsesFrozenSnapshotAfterPlanDeletion(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	allowWalletOverflow := false
	plan := &SubscriptionPlan{
		Title:                   "Deleted snapshot plan",
		Currency:                SubscriptionCurrencyCNY,
		DurationUnit:            SubscriptionDurationDay,
		DurationValue:           14,
		Enabled:                 true,
		AllowWalletOverflow:     &allowWalletOverflow,
		TotalAmount:             4321,
		QuotaResetPeriod:        SubscriptionResetCustom,
		QuotaResetCustomSeconds: 7200,
	}
	require.NoError(t, DB.Create(plan).Error)

	redemption := &Redemption{
		Name:        "deleted-plan-subscription-code",
		Key:         "30000000000000000000000000000005",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.FreezeSubscriptionPlan(plan.Id))
	require.NoError(t, redemption.Insert())
	require.NoError(t, DB.Delete(&SubscriptionPlan{}, plan.Id).Error)

	var planCount int64
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Count(&planCount).Error)
	require.Zero(t, planCount)

	result, err := RedeemCode(redemption.Key, userId)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionBenefitSubscription, result.Type)
	assert.Equal(t, plan.Id, result.PlanId)
	assert.Equal(t, "Deleted snapshot plan", result.PlanTitle)

	var subscription UserSubscription
	require.NoError(t, DB.First(&subscription, result.SubscriptionId).Error)
	assert.Equal(t, int64(4321), subscription.AmountTotal)
	assert.Equal(t, int64(7200), subscription.QuotaResetCustomSeconds)
	assert.False(t, subscription.AllowWalletOverflow)
	assert.Equal(t, "redemption", subscription.Source)

	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Zero(t, user.Quota)
}

func TestLegacyTopUpRejectsSubscriptionCodeWithoutConsumingIt(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	plan := &SubscriptionPlan{
		Title:            "Subscription only",
		Currency:         SubscriptionCurrencyCNY,
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    7,
		Enabled:          true,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	redemption := &Redemption{
		Name:        "subscription-only-code",
		Key:         "30000000000000000000000000000002",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.FreezeSubscriptionPlan(plan.Id))
	require.NoError(t, redemption.Insert())

	_, err := Redeem(redemption.Key, userId)
	require.Error(t, err)

	var stored Redemption
	require.NoError(t, DB.First(&stored, "key = ?", redemption.Key).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, stored.Status)
	assert.Zero(t, stored.UsedUserId)
	var subscriptionCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", userId).Count(&subscriptionCount).Error)
	assert.Zero(t, subscriptionCount)
}

func TestSubscriptionRedeemRollsBackCodeWhenSubscriptionCreationFails(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	plan := &SubscriptionPlan{
		Title:            "Rollback plan",
		Currency:         SubscriptionCurrencyCNY,
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	redemption := &Redemption{
		Name:        "invalid-snapshot-code",
		Key:         "30000000000000000000000000000003",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.FreezeSubscriptionPlan(plan.Id))
	var snapshot redemptionSubscriptionSnapshot
	require.NoError(t, common.UnmarshalJsonStr(redemption.SubscriptionPlanSnapshot, &snapshot))
	snapshot.DurationUnit = "invalid"
	data, err := common.Marshal(snapshot)
	require.NoError(t, err)
	redemption.SubscriptionPlanSnapshot = string(data)
	require.NoError(t, redemption.Insert())

	_, err = RedeemCode(redemption.Key, userId)
	require.Error(t, err)

	var stored Redemption
	require.NoError(t, DB.First(&stored, "key = ?", redemption.Key).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, stored.Status)
	assert.Zero(t, stored.UsedSubscriptionId)
	var subscriptionCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", userId).Count(&subscriptionCount).Error)
	assert.Zero(t, subscriptionCount)
}

func TestSubscriptionRedeemConcurrentSingleSuccess(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	plan := &SubscriptionPlan{
		Title:            "Concurrent plan",
		Currency:         SubscriptionCurrencyCNY,
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	redemption := &Redemption{
		Name:        "concurrent-subscription-code",
		Key:         "30000000000000000000000000000004",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.FreezeSubscriptionPlan(plan.Id))
	require.NoError(t, redemption.Insert())

	const goroutines = 5
	successes := make([]bool, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if _, err := RedeemCode(redemption.Key, userId); err == nil {
				successes[idx] = true
			}
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, success := range successes {
		if success {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount)
	var subscriptionCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", userId).Count(&subscriptionCount).Error)
	assert.Equal(t, int64(1), subscriptionCount)
}

func TestCalculateAffiliateCommissionUsesBasisPoints(t *testing.T) {
	const hundredDollarsQuota = 50_000_000
	const tenDollarsQuota = 5_000_000

	commission, err := CalculateAffiliateCommission(hundredDollarsQuota, 1000)
	require.NoError(t, err)
	assert.Equal(t, tenDollarsQuota, commission)

	commission, err = CalculateAffiliateCommission(5, 1000)
	require.NoError(t, err)
	assert.Equal(t, 1, commission, "half quota units round away from zero")

	_, err = CalculateAffiliateCommission(hundredDollarsQuota, 0)
	require.ErrorIs(t, err, ErrAffiliateRateInvalid)
	_, err = CalculateAffiliateCommission(hundredDollarsQuota, AffiliateAgentMaxRateBps+1)
	require.ErrorIs(t, err, ErrAffiliateRateInvalid)
	_, err = CalculateAffiliateCommission(-1, 1000)
	require.ErrorIs(t, err, ErrAffiliateAmountInvalid)
}

func TestRedeemCreatesCashbackForSelectedAgent(t *testing.T) {
	inviteeId, key := setupRedeemFixture(t, 50_000_000)

	agent := &User{
		Username: "selected-agent",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "selected-agent-code",
	}
	require.NoError(t, DB.Create(agent).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviteeId).Update("inviter_id", agent.Id).Error)
	_, err := UpsertAffiliateAgentConfig(agent.Id, true, 1000, true)
	require.NoError(t, err)

	quota, err := Redeem(key, inviteeId)
	require.NoError(t, err)
	assert.Equal(t, 50_000_000, quota)

	var commission AffiliateCommission
	require.NoError(t, DB.First(&commission).Error)
	assert.Equal(t, agent.Id, commission.InviterUserId)
	assert.Equal(t, inviteeId, commission.InviteeUserId)
	assert.Equal(t, 50_000_000, commission.SourceQuota)
	assert.Equal(t, 1000, commission.RateBps)
	assert.Equal(t, 5_000_000, commission.CommissionQuota)
	assert.Equal(t, AffiliateRewardTypeSelectedAgent, commission.RewardType)
	assert.Equal(t, AffiliateRewardDestinationCashback, commission.Destination)

	var agentUser User
	require.NoError(t, DB.First(&agentUser, "id = ?", agent.Id).Error)
	assert.Zero(t, agentUser.Quota, "selected-agent reward must not also credit site balance")

	var agentAccount AffiliateAgent
	require.NoError(t, DB.First(&agentAccount, "user_id = ?", agent.Id).Error)
	assert.Equal(t, int64(5_000_000), agentAccount.AvailableQuota)
	assert.Equal(t, int64(5_000_000), agentAccount.TotalCommissionQuota)

	// Retrying the same code neither credits the invitee nor creates a second
	// cashback record.
	_, err = Redeem(key, inviteeId)
	require.Error(t, err)
	var commissionCount int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Count(&commissionCount).Error)
	assert.Equal(t, int64(1), commissionCount)
	require.NoError(t, DB.First(&agentAccount, "user_id = ?", agent.Id).Error)
	assert.Equal(t, int64(5_000_000), agentAccount.AvailableQuota)
}

func TestRedeemCreditsFivePercentOnceForOrdinaryInviter(t *testing.T) {
	inviteeId, key := setupRedeemFixture(t, 50_000_000)

	inviter := &User{
		Username: "ordinary-inviter",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "ordinary-inviter-code",
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviteeId).Update("inviter_id", inviter.Id).Error)

	_, err := Redeem(key, inviteeId)
	require.NoError(t, err)

	var inviterAfterFirst User
	require.NoError(t, DB.First(&inviterAfterFirst, "id = ?", inviter.Id).Error)
	assert.Equal(t, 2_500_000, inviterAfterFirst.Quota)
	assert.Zero(t, inviterAfterFirst.AffQuota, "ordinary redemption rewards are not withdrawable affiliate quota")

	var commission AffiliateCommission
	require.NoError(t, DB.First(&commission).Error)
	assert.Equal(t, inviter.Id, commission.InviterUserId)
	assert.Equal(t, AffiliateRewardTypeOrdinaryFirst, commission.RewardType)
	assert.Equal(t, AffiliateRewardDestinationBalance, commission.Destination)
	assert.Equal(t, AffiliateOrdinaryFirstRateBps, commission.RateBps)
	assert.Equal(t, 2_500_000, commission.CommissionQuota)

	second := &Redemption{
		Name:        "ordinary-second-redeem",
		Key:         "20000000000000000000000000000003",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       10_000_000,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(second).Error)
	_, err = Redeem(second.Key, inviteeId)
	require.NoError(t, err)

	var inviterAfterSecond User
	require.NoError(t, DB.First(&inviterAfterSecond, "id = ?", inviter.Id).Error)
	assert.Equal(t, 2_500_000, inviterAfterSecond.Quota)

	var commissionCount int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Count(&commissionCount).Error)
	assert.Equal(t, int64(1), commissionCount)
	var claimCount int64
	require.NoError(t, DB.Model(&AffiliateFirstRewardClaim{}).Count(&claimCount).Error)
	assert.Equal(t, int64(1), claimCount)
}

func TestConcurrentRedemptionsGrantOneOrdinaryFirstReward(t *testing.T) {
	inviteeId, firstKey := setupRedeemFixture(t, 10_000_000)
	inviter := &User{
		Username: "concurrent-ordinary-inviter",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "concurrent-ordinary-code",
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviteeId).Update("inviter_id", inviter.Id).Error)

	second := &Redemption{
		Name:        "concurrent-ordinary-second",
		Key:         "20000000000000000000000000000005",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       20_000_000,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(second).Error)

	keys := []string{firstKey, second.Key}
	errorsByRedemption := make([]error, len(keys))
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(keys))
	for index, redemptionKey := range keys {
		go func(resultIndex int, key string) {
			defer waitGroup.Done()
			_, errorsByRedemption[resultIndex] = Redeem(key, inviteeId)
		}(index, redemptionKey)
	}
	waitGroup.Wait()
	for _, redeemErr := range errorsByRedemption {
		require.NoError(t, redeemErr)
	}

	var inviterAfter User
	require.NoError(t, DB.First(&inviterAfter, "id = ?", inviter.Id).Error)
	assert.Contains(t, []int{500_000, 1_000_000}, inviterAfter.Quota)

	var claimCount int64
	require.NoError(t, DB.Model(&AffiliateFirstRewardClaim{}).Count(&claimCount).Error)
	assert.Equal(t, int64(1), claimCount)
	var commissionCount int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Count(&commissionCount).Error)
	assert.Equal(t, int64(1), commissionCount)
}

func TestAffiliateAgentRateMustBeBetweenFiveAndTenPercentWhenEnabled(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)

	_, err := UpsertAffiliateAgentConfig(userId, true, AffiliateAgentMinRateBps-1, true)
	require.ErrorIs(t, err, ErrAffiliateRateInvalid)
	_, err = UpsertAffiliateAgentConfig(userId, true, AffiliateAgentMaxRateBps+1, true)
	require.ErrorIs(t, err, ErrAffiliateRateInvalid)

	agent, err := UpsertAffiliateAgentConfig(userId, true, AffiliateAgentMinRateBps, true)
	require.NoError(t, err)
	assert.Equal(t, AffiliateAgentMinRateBps, agent.CommissionRateBps)

	disabled, err := UpsertAffiliateAgentConfig(userId, false, 0, false)
	require.NoError(t, err)
	assert.False(t, disabled.Enabled)
}

func TestAffiliateInviteeSummaryCountsAllRedemptionsButOnlyOneOrdinaryReward(t *testing.T) {
	inviteeId, firstKey := setupRedeemFixture(t, 50_000_000)
	inviter := &User{
		Username: "summary-inviter",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "summary-inviter-code",
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviteeId).Update("inviter_id", inviter.Id).Error)

	_, err := Redeem(firstKey, inviteeId)
	require.NoError(t, err)
	second := &Redemption{
		Name:        "summary-second-redeem",
		Key:         "20000000000000000000000000000004",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       10_000_000,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(second).Error)
	_, err = Redeem(second.Key, inviteeId)
	require.NoError(t, err)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	invitees, total, err := ListAffiliateInvitees(inviter.Id, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, invitees, 1)
	assert.Equal(t, int64(2), invitees[0].RedemptionCount)
	assert.Equal(t, int64(60_000_000), invitees[0].RedeemedQuota)
	assert.Equal(t, int64(2_500_000), invitees[0].RewardQuota)

	summary, err := GetAffiliateSummary(inviter.Id)
	require.NoError(t, err)
	assert.False(t, summary.IsAgent)
	assert.Equal(t, int64(1), summary.InviteeCount)
	assert.Equal(t, int64(2_500_000), summary.OrdinaryRewardQuota)
	assert.Equal(t, int64(2_500_000), summary.TotalRewardQuota)
}

func TestInviteUserOnlyIncrementsInviteCount(t *testing.T) {
	_, _ = setupRedeemFixture(t, 500)
	inviter := &User{
		Username:        "registration-inviter",
		Password:        "password",
		Status:          common.UserStatusEnabled,
		AffCode:         "registration-inviter-code",
		AffQuota:        123,
		AffHistoryQuota: 456,
	}
	require.NoError(t, DB.Create(inviter).Error)

	require.NoError(t, inviteUser(inviter.Id))

	var updated User
	require.NoError(t, DB.First(&updated, "id = ?", inviter.Id).Error)
	assert.Equal(t, 1, updated.AffCount)
	assert.Equal(t, 123, updated.AffQuota)
	assert.Equal(t, 456, updated.AffHistoryQuota)
	assert.Zero(t, updated.Quota)
}

func TestRedeemSnapshotsAgentRate(t *testing.T) {
	inviteeId, firstKey := setupRedeemFixture(t, 10_000_000)

	agent := &User{
		Username: "rate-agent",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "rate-agent-code",
	}
	require.NoError(t, DB.Create(agent).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviteeId).Update("inviter_id", agent.Id).Error)
	_, err := UpsertAffiliateAgentConfig(agent.Id, true, 500, true)
	require.NoError(t, err)

	_, err = Redeem(firstKey, inviteeId)
	require.NoError(t, err)

	_, err = UpsertAffiliateAgentConfig(agent.Id, true, 1000, true)
	require.NoError(t, err)
	second := &Redemption{
		Name:        "second-rate-test",
		Key:         "20000000000000000000000000000002",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       10_000_000,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(second).Error)
	_, err = Redeem(second.Key, inviteeId)
	require.NoError(t, err)

	var commissions []AffiliateCommission
	require.NoError(t, DB.Order("id ASC").Find(&commissions).Error)
	require.Len(t, commissions, 2)
	assert.Equal(t, 500, commissions[0].RateBps)
	assert.Equal(t, 500_000, commissions[0].CommissionQuota)
	assert.Equal(t, 1000, commissions[1].RateBps)
	assert.Equal(t, 1_000_000, commissions[1].CommissionQuota)
}

func TestAffiliateCashbackConversionIsAtomic(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:               userId,
		Enabled:              true,
		CommissionRateBps:    1000,
		AvailableQuota:       5_000_000,
		TotalCommissionQuota: 5_000_000,
	}).Error)

	conversion, err := ConvertAffiliateCashbackToBalance(userId, 2_000_000)
	require.NoError(t, err)
	assert.Equal(t, int64(2_000_000), conversion.AmountQuota)

	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 2_000_000, user.Quota)
	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(3_000_000), agent.AvailableQuota)
	assert.Equal(t, int64(2_000_000), agent.ConvertedQuota)

	_, err = ConvertAffiliateCashbackToBalance(userId, 4_000_000)
	require.ErrorIs(t, err, ErrAffiliateBalanceInsufficient)
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 2_000_000, user.Quota)
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(3_000_000), agent.AvailableQuota)

	var conversionCount int64
	require.NoError(t, DB.Model(&AffiliateConversion{}).Count(&conversionCount).Error)
	assert.Equal(t, int64(1), conversionCount)
}

func TestAffiliateCashbackConversionRollsBackOnUserQuotaOverflow(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Update("quota", common.MaxQuota-100).Error)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:               userId,
		Enabled:              true,
		CommissionRateBps:    1000,
		AvailableQuota:       200,
		TotalCommissionQuota: 200,
	}).Error)

	_, err := ConvertAffiliateCashbackToBalance(userId, 200)
	require.ErrorIs(t, err, ErrAffiliateBalanceOverflow)

	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(200), agent.AvailableQuota)
	assert.Zero(t, agent.ConvertedQuota)
	var conversionCount int64
	require.NoError(t, DB.Model(&AffiliateConversion{}).Count(&conversionCount).Error)
	assert.Zero(t, conversionCount)
}

func TestAffiliateWithdrawalReserveRejectAndPay(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:                userId,
		Enabled:               true,
		CommissionRateBps:     1000,
		CashWithdrawalEnabled: true,
		AvailableQuota:        5_000_000,
		TotalCommissionQuota:  5_000_000,
	}).Error)

	first, err := CreateAffiliateWithdrawal(userId, 2_000_000, "PayPal")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPending, first.Status)

	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(3_000_000), agent.AvailableQuota)
	assert.Equal(t, int64(2_000_000), agent.PendingWithdrawalQuota)

	rejected, err := ReviewAffiliateWithdrawal(first.Id, 999, false, "资料不完整")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusRejected, rejected.Status)
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(5_000_000), agent.AvailableQuota)
	assert.Zero(t, agent.PendingWithdrawalQuota)

	_, err = ReviewAffiliateWithdrawal(first.Id, 999, true, "重复处理")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalNotPending)

	second, err := CreateAffiliateWithdrawal(userId, 3_000_000, "Bank transfer")
	require.NoError(t, err)
	paid, err := ReviewAffiliateWithdrawal(second.Id, 999, true, "offline transfer #123")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, paid.Status)
	assert.NotZero(t, paid.PaidAt)
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(2_000_000), agent.AvailableQuota)
	assert.Zero(t, agent.PendingWithdrawalQuota)
	assert.Equal(t, int64(3_000_000), agent.WithdrawnQuota)
}

func TestAffiliateWithdrawalRequiresPermission(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:               userId,
		Enabled:              true,
		CommissionRateBps:    1000,
		AvailableQuota:       1_000_000,
		TotalCommissionQuota: 1_000_000,
	}).Error)

	_, err := CreateAffiliateWithdrawal(userId, 500_000, "")
	require.ErrorIs(t, err, ErrAffiliateCashWithdrawalDisabled)

	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(1_000_000), agent.AvailableQuota)
	assert.Zero(t, agent.PendingWithdrawalQuota)
}
