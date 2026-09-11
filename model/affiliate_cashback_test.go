package model

import (
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAffiliateAmountValidationRejectsLedgerMutations(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:                userId,
		Enabled:               true,
		CommissionRateBps:     AffiliateAgentMaxRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        1_000,
		TotalCommissionQuota:  1_000,
	}).Error)

	invalidAmounts := []int64{0, -1, int64(common.MaxQuota) + 1}
	for _, amount := range invalidAmounts {
		_, err := ConvertAffiliateCashbackToBalance(userId, amount)
		require.ErrorIs(t, err, ErrAffiliateAmountInvalid)

		_, err = CreateAffiliateWithdrawal(userId, amount, "")
		require.ErrorIs(t, err, ErrAffiliateAmountInvalid)
	}

	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(1_000), agent.AvailableQuota)
	assert.Zero(t, agent.PendingWithdrawalQuota)
	assert.Zero(t, agent.ConvertedQuota)

	var conversionCount int64
	require.NoError(t, DB.Model(&AffiliateConversion{}).Count(&conversionCount).Error)
	assert.Zero(t, conversionCount)
	var withdrawalCount int64
	require.NoError(t, DB.Model(&AffiliateWithdrawal{}).Count(&withdrawalCount).Error)
	assert.Zero(t, withdrawalCount)
}

func TestDisabledAgentUsesOrdinaryRuleAndKeepsHistoricalCashbackUsable(t *testing.T) {
	firstInviteeId, firstKey := setupRedeemFixture(t, 10_000_000)
	agentUser := &User{
		Username: "lifecycle-agent",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "lifecycle-agent-code",
	}
	require.NoError(t, DB.Create(agentUser).Error)
	require.NoError(t, DB.Model(&User{}).
		Where("id = ?", firstInviteeId).
		Update("inviter_id", agentUser.Id).Error)
	_, err := UpsertAffiliateAgentConfig(
		agentUser.Id,
		true,
		AffiliateAgentMaxRateBps,
		true,
	)
	require.NoError(t, err)

	_, err = Redeem(firstKey, firstInviteeId)
	require.NoError(t, err)

	disabled, err := UpsertAffiliateAgentConfig(agentUser.Id, false, 0, true)
	require.NoError(t, err)
	assert.False(t, disabled.Enabled)
	assert.Equal(t, int64(1_000_000), disabled.AvailableQuota)

	secondInvitee := &User{
		Username:  "disabled-agent-invitee",
		Password:  "password",
		Status:    common.UserStatusEnabled,
		AffCode:   "disabled-agent-invitee-code",
		InviterId: agentUser.Id,
	}
	require.NoError(t, DB.Create(secondInvitee).Error)
	secondRedemption := &Redemption{
		Name:        "disabled-agent-redemption",
		Key:         "30000000000000000000000000000001",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       20_000_000,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(secondRedemption).Error)
	_, err = Redeem(secondRedemption.Key, secondInvitee.Id)
	require.NoError(t, err)

	var userAfterOrdinaryReward User
	require.NoError(t, DB.First(&userAfterOrdinaryReward, "id = ?", agentUser.Id).Error)
	assert.Equal(t, 1_000_000, userAfterOrdinaryReward.Quota)

	var agentAfterOrdinaryReward AffiliateAgent
	require.NoError(t, DB.First(&agentAfterOrdinaryReward, "user_id = ?", agentUser.Id).Error)
	assert.Equal(t, int64(1_000_000), agentAfterOrdinaryReward.AvailableQuota)
	assert.Equal(t, int64(1_000_000), agentAfterOrdinaryReward.TotalCommissionQuota)

	var commissions []AffiliateCommission
	require.NoError(t, DB.Order("id ASC").Find(&commissions).Error)
	require.Len(t, commissions, 2)
	assert.Equal(t, AffiliateRewardTypeSelectedAgent, commissions[0].RewardType)
	assert.Equal(t, AffiliateRewardDestinationCashback, commissions[0].Destination)
	assert.Equal(t, AffiliateRewardTypeOrdinaryFirst, commissions[1].RewardType)
	assert.Equal(t, AffiliateRewardDestinationBalance, commissions[1].Destination)

	conversion, err := ConvertAffiliateCashbackToBalance(agentUser.Id, 400_000)
	require.NoError(t, err)
	assert.Equal(t, int64(400_000), conversion.AmountQuota)
	withdrawal, err := CreateAffiliateWithdrawal(agentUser.Id, 200_000, "offline payout")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPending, withdrawal.Status)

	reenabled, err := UpsertAffiliateAgentConfig(
		agentUser.Id,
		true,
		800,
		true,
	)
	require.NoError(t, err)
	assert.True(t, reenabled.Enabled)
	assert.Equal(t, 800, reenabled.CommissionRateBps)
	assert.Equal(t, int64(400_000), reenabled.AvailableQuota)
	assert.Equal(t, int64(200_000), reenabled.PendingWithdrawalQuota)
	assert.Equal(t, int64(400_000), reenabled.ConvertedQuota)
	assert.Equal(t, int64(1_000_000), reenabled.TotalCommissionQuota)
}

func TestConcurrentWithdrawalReviewsSettleBalanceExactlyOnce(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:                userId,
		Enabled:               true,
		CommissionRateBps:     AffiliateAgentMaxRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        1_000_000,
		TotalCommissionQuota:  1_000_000,
	}).Error)
	withdrawal, err := CreateAffiliateWithdrawal(userId, 1_000_000, "")
	require.NoError(t, err)

	reviewErrors := make([]error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		_, reviewErrors[0] = ReviewAffiliateWithdrawal(withdrawal.Id, 101, true, "paid")
	}()
	go func() {
		defer waitGroup.Done()
		_, reviewErrors[1] = ReviewAffiliateWithdrawal(withdrawal.Id, 102, false, "rejected")
	}()
	waitGroup.Wait()

	successCount := 0
	notPendingCount := 0
	for _, reviewErr := range reviewErrors {
		if reviewErr == nil {
			successCount++
			continue
		}
		if assert.ErrorIs(t, reviewErr, ErrAffiliateWithdrawalNotPending) {
			notPendingCount++
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, notPendingCount)

	var storedWithdrawal AffiliateWithdrawal
	require.NoError(t, DB.First(&storedWithdrawal, withdrawal.Id).Error)
	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Zero(t, agent.PendingWithdrawalQuota)
	assert.Equal(
		t,
		agent.TotalCommissionQuota,
		agent.AvailableQuota+agent.WithdrawnQuota,
	)
	if storedWithdrawal.Status == AffiliateWithdrawalStatusPaid {
		assert.Zero(t, agent.AvailableQuota)
		assert.Equal(t, int64(1_000_000), agent.WithdrawnQuota)
		assert.NotZero(t, storedWithdrawal.PaidAt)
	} else {
		assert.Equal(t, AffiliateWithdrawalStatusRejected, storedWithdrawal.Status)
		assert.Equal(t, int64(1_000_000), agent.AvailableQuota)
		assert.Zero(t, agent.WithdrawnQuota)
		assert.Zero(t, storedWithdrawal.PaidAt)
	}
}

func TestConcurrentWithdrawalRequestsCannotOverspendCashback(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:                userId,
		Enabled:               true,
		CommissionRateBps:     AffiliateAgentMaxRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        1_000_000,
		TotalCommissionQuota:  1_000_000,
	}).Error)

	requestErrors := make([]error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	for index := range requestErrors {
		go func(resultIndex int) {
			defer waitGroup.Done()
			_, requestErrors[resultIndex] = CreateAffiliateWithdrawal(
				userId,
				750_000,
				"concurrent request",
			)
		}(index)
	}
	waitGroup.Wait()

	successCount := 0
	insufficientCount := 0
	for _, requestErr := range requestErrors {
		if requestErr == nil {
			successCount++
			continue
		}
		if assert.ErrorIs(t, requestErr, ErrAffiliateBalanceInsufficient) {
			insufficientCount++
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, insufficientCount)

	var agent AffiliateAgent
	require.NoError(t, DB.First(&agent, "user_id = ?", userId).Error)
	assert.Equal(t, int64(250_000), agent.AvailableQuota)
	assert.Equal(t, int64(750_000), agent.PendingWithdrawalQuota)
	var withdrawalCount int64
	require.NoError(t, DB.Model(&AffiliateWithdrawal{}).Count(&withdrawalCount).Error)
	assert.Equal(t, int64(1), withdrawalCount)
}

func TestAffiliateSelfListsIsolateAndMaskInvitedAccounts(t *testing.T) {
	_, _ = setupRedeemFixture(t, 500)
	agentA := createAffiliateTestUser(t, "privacy-agent-a", "privacy-agent-a-code", 0)
	agentB := createAffiliateTestUser(t, "privacy-agent-b", "privacy-agent-b-code", 0)
	inviteeA1 := createAffiliateTestUser(t, "alice-one", "privacy-a1-code", agentA.Id)
	inviteeA2 := createAffiliateTestUser(t, "alice-two", "privacy-a2-code", agentA.Id)
	inviteeB := createAffiliateTestUser(t, "bob-other", "privacy-b-code", agentB.Id)

	redemptionA1 := createUsedAffiliateTestRedemption(t, "privacy-a1", "40000000000000000000000000000001", inviteeA1.Id, 100)
	redemptionA2 := createUsedAffiliateTestRedemption(t, "privacy-a2", "40000000000000000000000000000002", inviteeA2.Id, 200)
	redemptionB := createUsedAffiliateTestRedemption(t, "privacy-b", "40000000000000000000000000000003", inviteeB.Id, 300)
	createAffiliateTestCommission(t, agentA.Id, inviteeA1.Id, redemptionA1.Id, 5)
	createAffiliateTestCommission(t, agentA.Id, inviteeA2.Id, redemptionA2.Id, 10)
	createAffiliateTestCommission(t, agentB.Id, inviteeB.Id, redemptionB.Id, 15)

	firstPage := &common.PageInfo{Page: 1, PageSize: 1}
	invitees, total, err := ListAffiliateInvitees(agentA.Id, firstPage)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, invitees, 1)
	assert.NotContains(t, invitees[0].Username, "alice")
	assert.Contains(t, invitees[0].Username, "***")

	secondPage := &common.PageInfo{Page: 2, PageSize: 1}
	nextInvitees, nextTotal, err := ListAffiliateInvitees(agentA.Id, secondPage)
	require.NoError(t, err)
	assert.Equal(t, int64(2), nextTotal)
	require.Len(t, nextInvitees, 1)
	assert.NotEqual(t, invitees[0].UserId, nextInvitees[0].UserId)
	assert.ElementsMatch(
		t,
		[]int{inviteeA1.Id, inviteeA2.Id},
		[]int{invitees[0].UserId, nextInvitees[0].UserId},
	)

	recordsPage := &common.PageInfo{Page: 1, PageSize: 10}
	redemptions, redemptionTotal, err := ListAffiliateRedemptionsForInviter(agentA.Id, recordsPage)
	require.NoError(t, err)
	assert.Equal(t, int64(2), redemptionTotal)
	require.Len(t, redemptions, 2)
	for _, redemption := range redemptions {
		assert.Equal(t, agentA.Id, redemption.InviterUserId)
		assert.NotEqual(t, inviteeB.Id, redemption.InviteeUserId)
		assert.Contains(t, redemption.InviteeUsername, "***")
	}

	commissions, commissionTotal, err := ListAffiliateCommissionsForAgent(agentA.Id, recordsPage)
	require.NoError(t, err)
	assert.Equal(t, int64(2), commissionTotal)
	require.Len(t, commissions, 2)
	for _, commission := range commissions {
		assert.Equal(t, agentA.Id, commission.InviterUserId)
		assert.NotEqual(t, inviteeB.Id, commission.InviteeUserId)
		assert.Contains(t, commission.InviteeUsername, "***")
		assert.Empty(t, commission.RedemptionName)
	}
}

func TestAffiliateWithdrawalListsEnforceOwnerAndStatusFilters(t *testing.T) {
	_, _ = setupRedeemFixture(t, 500)
	agentA := createAffiliateTestUser(t, "withdraw-list-a", "withdraw-list-a-code", 0)
	agentB := createAffiliateTestUser(t, "withdraw-list-b", "withdraw-list-b-code", 0)
	for _, userId := range []int{agentA.Id, agentB.Id} {
		require.NoError(t, DB.Create(&AffiliateAgent{
			UserId:                userId,
			Enabled:               true,
			CommissionRateBps:     AffiliateAgentMinRateBps,
			CashWithdrawalEnabled: true,
			AvailableQuota:        1_000,
			TotalCommissionQuota:  1_000,
		}).Error)
	}

	pendingA, err := CreateAffiliateWithdrawal(agentA.Id, 400, "agent A")
	require.NoError(t, err)
	paidB, err := CreateAffiliateWithdrawal(agentB.Id, 500, "agent B")
	require.NoError(t, err)
	_, err = ReviewAffiliateWithdrawal(paidB.Id, 999, true, "paid offline")
	require.NoError(t, err)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	agentRows, agentTotal, err := ListAffiliateWithdrawalsForAgent(agentA.Id, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), agentTotal)
	require.Len(t, agentRows, 1)
	assert.Equal(t, pendingA.Id, agentRows[0].Id)
	assert.Equal(t, agentA.Id, agentRows[0].AgentUserId)

	paidRows, paidTotal, err := ListAllAffiliateWithdrawals(
		AffiliateWithdrawalStatusPaid,
		pageInfo,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), paidTotal)
	require.Len(t, paidRows, 1)
	assert.Equal(t, paidB.Id, paidRows[0].Id)
	assert.Equal(t, "withdraw-list-b", paidRows[0].AgentUsername)

	_, _, err = ListAllAffiliateWithdrawals("processing", pageInfo)
	require.EqualError(t, err, "无效的提现状态")
}

func TestAffiliateWithdrawalNoteLimitsCountRunes(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 500)
	require.NoError(t, DB.Create(&AffiliateAgent{
		UserId:                userId,
		Enabled:               true,
		CommissionRateBps:     AffiliateAgentMinRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        1_000,
		TotalCommissionQuota:  1_000,
	}).Error)

	acceptedNote := strings.Repeat("好", 500)
	withdrawal, err := CreateAffiliateWithdrawal(userId, 400, acceptedNote)
	require.NoError(t, err)
	assert.Equal(t, acceptedNote, withdrawal.ApplicantNote)

	_, err = CreateAffiliateWithdrawal(userId, 100, strings.Repeat("好", 501))
	require.EqualError(t, err, "提现备注不能超过 500 个字符")

	_, err = ReviewAffiliateWithdrawal(
		withdrawal.Id,
		999,
		true,
		strings.Repeat("审", 501),
	)
	require.EqualError(t, err, "审核备注不能超过 500 个字符")

	var stored AffiliateWithdrawal
	require.NoError(t, DB.First(&stored, withdrawal.Id).Error)
	assert.Equal(t, AffiliateWithdrawalStatusPending, stored.Status)
}

func createAffiliateTestUser(t *testing.T, username string, affCode string, inviterId int) *User {
	t.Helper()
	user := &User{
		Username:  username,
		Password:  "password",
		Status:    common.UserStatusEnabled,
		AffCode:   affCode,
		InviterId: inviterId,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func createUsedAffiliateTestRedemption(
	t *testing.T,
	name string,
	key string,
	inviteeUserId int,
	quota int,
) *Redemption {
	t.Helper()
	redemption := &Redemption{
		Name:         name,
		Key:          key,
		Status:       common.RedemptionCodeStatusUsed,
		Quota:        quota,
		CreatedTime:  common.GetTimestamp(),
		RedeemedTime: common.GetTimestamp(),
		UsedUserId:   inviteeUserId,
	}
	require.NoError(t, DB.Create(redemption).Error)
	return redemption
}

func createAffiliateTestCommission(
	t *testing.T,
	inviterUserId int,
	inviteeUserId int,
	redemptionId int,
	commissionQuota int,
) {
	t.Helper()
	require.NoError(t, DB.Create(&AffiliateCommission{
		InviterUserId:   inviterUserId,
		InviteeUserId:   inviteeUserId,
		RedemptionId:    redemptionId,
		SourceQuota:     commissionQuota * 20,
		RateBps:         AffiliateOrdinaryFirstRateBps,
		CommissionQuota: commissionQuota,
		RewardType:      AffiliateRewardTypeOrdinaryFirst,
		Destination:     AffiliateRewardDestinationBalance,
	}).Error)
}
