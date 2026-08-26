package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AffiliateRateBasisPoints      = 10000
	AffiliateOrdinaryFirstRateBps = 500
	AffiliateAgentMinRateBps      = 500
	AffiliateAgentMaxRateBps      = 1000

	AffiliateRewardTypeOrdinaryFirst = "ordinary_first"
	AffiliateRewardTypeSelectedAgent = "selected_agent"

	AffiliateRewardDestinationBalance  = "site_balance"
	AffiliateRewardDestinationCashback = "cashback"

	AffiliateWithdrawalStatusPending  = "pending"
	AffiliateWithdrawalStatusPaid     = "paid"
	AffiliateWithdrawalStatusRejected = "rejected"
)

var (
	ErrAffiliateAgentNotFound          = errors.New("该账号不是精选代理")
	ErrAffiliateRateInvalid            = errors.New("精选代理返现比例必须在 5% 到 10% 之间")
	ErrAffiliateAmountInvalid          = errors.New("返现金额无效")
	ErrAffiliateBalanceInsufficient    = errors.New("可用返现余额不足")
	ErrAffiliateBalanceOverflow        = errors.New("站内余额将超过系统上限")
	ErrAffiliateLedgerOverflow         = errors.New("返现账户金额将超过系统上限")
	ErrAffiliateCashWithdrawalDisabled = errors.New("该账号未开通提现")
	ErrAffiliateWithdrawalNotPending   = errors.New("该提现申请已处理")
)

// AffiliateAgent stores the per-account cashback policy and its materialized
// balances. Ordinary first-redemption rewards go straight to User.Quota;
// selected-agent cashback stays in this separate, optionally withdrawable
// account.
type AffiliateAgent struct {
	UserId                 int   `json:"user_id" gorm:"primaryKey"`
	Enabled                bool  `json:"enabled" gorm:"not null"`
	CommissionRateBps      int   `json:"commission_rate_bps" gorm:"not null"`
	CashWithdrawalEnabled  bool  `json:"cash_withdrawal_enabled" gorm:"not null"`
	AvailableQuota         int64 `json:"available_quota" gorm:"type:bigint;not null"`
	PendingWithdrawalQuota int64 `json:"pending_withdrawal_quota" gorm:"type:bigint;not null"`
	ConvertedQuota         int64 `json:"converted_quota" gorm:"type:bigint;not null"`
	WithdrawnQuota         int64 `json:"withdrawn_quota" gorm:"type:bigint;not null"`
	TotalCommissionQuota   int64 `json:"total_commission_quota" gorm:"type:bigint;not null"`
	CreatedAt              int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt              int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

// AffiliateFirstRewardClaim marks the first successful redemption for an
// invitee. The invitee primary key is the cross-database concurrency guard
// that prevents two different redemption codes from both granting the
// ordinary one-time reward.
type AffiliateFirstRewardClaim struct {
	InviteeUserId   int    `json:"invitee_user_id" gorm:"primaryKey"`
	InviterUserId   int    `json:"inviter_user_id" gorm:"not null;index"`
	RedemptionId    int    `json:"redemption_id" gorm:"not null;uniqueIndex"`
	FirstRewardType string `json:"first_reward_type" gorm:"type:varchar(32);not null"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

// AffiliateCommission is an immutable invitation reward record created from
// a successful redemption. RedemptionId is unique because the ordinary and
// selected-agent rules are mutually exclusive for each redemption.
type AffiliateCommission struct {
	Id              int    `json:"id"`
	InviterUserId   int    `json:"inviter_user_id" gorm:"not null;index"`
	InviteeUserId   int    `json:"invitee_user_id" gorm:"not null;index"`
	RedemptionId    int    `json:"redemption_id" gorm:"not null;uniqueIndex"`
	SourceQuota     int    `json:"source_quota" gorm:"not null"`
	RateBps         int    `json:"rate_bps" gorm:"not null"`
	CommissionQuota int    `json:"commission_quota" gorm:"not null"`
	RewardType      string `json:"reward_type" gorm:"type:varchar(32);not null;index"`
	Destination     string `json:"destination" gorm:"type:varchar(24);not null"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
}

// AffiliateConversion records cashback moved into the agent's site balance.
type AffiliateConversion struct {
	Id          int   `json:"id"`
	AgentUserId int   `json:"agent_user_id" gorm:"not null;index"`
	AmountQuota int64 `json:"amount_quota" gorm:"type:bigint;not null"`
	CreatedAt   int64 `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
}

// AffiliateWithdrawal records an offline payout workflow. The platform only
// reserves the balance and records the administrator's decision; it never
// initiates an external transfer.
type AffiliateWithdrawal struct {
	Id             int    `json:"id"`
	AgentUserId    int    `json:"agent_user_id" gorm:"not null;index"`
	AmountQuota    int64  `json:"amount_quota" gorm:"type:bigint;not null"`
	Status         string `json:"status" gorm:"type:varchar(16);not null;index"`
	ApplicantNote  string `json:"applicant_note" gorm:"type:varchar(500)"`
	AdminNote      string `json:"admin_note" gorm:"type:varchar(500)"`
	ReviewerUserId int    `json:"reviewer_user_id" gorm:"index"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
	ReviewedAt     int64  `json:"reviewed_at" gorm:"column:reviewed_at"`
	PaidAt         int64  `json:"paid_at" gorm:"column:paid_at"`
}

type AffiliateSummary struct {
	IsAgent                bool  `json:"is_agent"`
	Enabled                bool  `json:"enabled"`
	CommissionRateBps      int   `json:"commission_rate_bps"`
	CashWithdrawalEnabled  bool  `json:"cash_withdrawal_enabled"`
	AvailableQuota         int64 `json:"available_quota"`
	PendingWithdrawalQuota int64 `json:"pending_withdrawal_quota"`
	ConvertedQuota         int64 `json:"converted_quota"`
	WithdrawnQuota         int64 `json:"withdrawn_quota"`
	TotalCommissionQuota   int64 `json:"total_commission_quota"`
	InviteeCount           int64 `json:"invitee_count"`
	OrdinaryRewardQuota    int64 `json:"ordinary_reward_quota"`
	TotalRewardQuota       int64 `json:"total_reward_quota"`
	RedemptionCount        int64 `json:"redemption_count"`
	RedeemedQuota          int64 `json:"redeemed_quota"`
}

type AffiliateAgentRecord struct {
	AffiliateAgent
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type AffiliateCommissionRecord struct {
	AffiliateCommission
	InviterUsername string `json:"inviter_username"`
	InviteeUsername string `json:"invitee_username"`
	RedemptionName  string `json:"redemption_name"`
}

type AffiliateInviteeRecord struct {
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	CreatedAt       int64  `json:"created_at"`
	RedemptionCount int64  `json:"redemption_count"`
	RedeemedQuota   int64  `json:"redeemed_quota"`
	RewardQuota     int64  `json:"reward_quota"`
}

type AffiliateInvitationRecord struct {
	InviterUserId   int    `json:"inviter_user_id"`
	InviterUsername string `json:"inviter_username"`
	InviteeUserId   int    `json:"invitee_user_id"`
	InviteeUsername string `json:"invitee_username"`
	InviteeName     string `json:"invitee_name"`
	CreatedAt       int64  `json:"created_at"`
	RedemptionCount int64  `json:"redemption_count"`
	RedeemedQuota   int64  `json:"redeemed_quota"`
	RewardQuota     int64  `json:"reward_quota"`
}

type AffiliateRedemptionRecord struct {
	RedemptionId    int    `json:"redemption_id"`
	InviterUserId   int    `json:"inviter_user_id"`
	InviterUsername string `json:"inviter_username"`
	InviteeUserId   int    `json:"invitee_user_id"`
	InviteeUsername string `json:"invitee_username"`
	SourceQuota     int64  `json:"source_quota"`
	RedeemedAt      int64  `json:"redeemed_at"`
	RewardType      string `json:"reward_type"`
	Destination     string `json:"destination"`
	RateBps         int    `json:"rate_bps"`
	RewardQuota     int64  `json:"reward_quota"`
}

type AffiliateConversionRecord struct {
	AffiliateConversion
	AgentUsername string `json:"agent_username"`
}

type AffiliateWithdrawalRecord struct {
	AffiliateWithdrawal
	AgentUsername string `json:"agent_username"`
}

func validateAffiliateConfig(enabled bool, rateBps int) error {
	if !enabled && rateBps == 0 {
		return nil
	}
	if rateBps < AffiliateAgentMinRateBps || rateBps > AffiliateAgentMaxRateBps {
		return ErrAffiliateRateInvalid
	}
	return nil
}

func validateAffiliateAmount(amountQuota int64) error {
	if amountQuota <= 0 || amountQuota > int64(common.MaxQuota) {
		return ErrAffiliateAmountInvalid
	}
	return nil
}

// CalculateAffiliateCommission calculates a basis-point percentage without
// float arithmetic. The shared quota converter provides the same rounding and
// overflow policy used by the rest of billing.
func CalculateAffiliateCommission(sourceQuota int, rateBps int) (int, error) {
	if sourceQuota <= 0 {
		return 0, ErrAffiliateAmountInvalid
	}
	if rateBps < AffiliateAgentMinRateBps || rateBps > AffiliateAgentMaxRateBps {
		return 0, ErrAffiliateRateInvalid
	}
	commissionDecimal := decimal.NewFromInt(int64(sourceQuota)).
		Mul(decimal.NewFromInt(int64(rateBps))).
		Div(decimal.NewFromInt(AffiliateRateBasisPoints))
	commissionQuota, clamp := common.QuotaFromDecimalChecked(commissionDecimal)
	if clamp != nil {
		return 0, clamp
	}
	if commissionQuota < 0 {
		return 0, ErrAffiliateAmountInvalid
	}
	return commissionQuota, nil
}

func createAffiliateCommissionForRedemptionTx(tx *gorm.DB, redemption *Redemption, inviteeUserId int) error {
	if redemption == nil || redemption.Id == 0 || redemption.Quota <= 0 {
		return ErrAffiliateAmountInvalid
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").First(&invitee, "id = ?", inviteeUserId).Error; err != nil {
		return err
	}
	if invitee.InviterId == 0 {
		return nil
	}

	var agent AffiliateAgent
	err := lockForUpdate(tx).
		Where("user_id = ? AND enabled = ?", invitee.InviterId, true).
		First(&agent).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err == nil {
		if _, err := claimAffiliateFirstRedemptionTx(
			tx,
			invitee.InviterId,
			inviteeUserId,
			redemption.Id,
			AffiliateRewardTypeSelectedAgent,
		); err != nil {
			return err
		}

		commissionQuota, err := CalculateAffiliateCommission(redemption.Quota, agent.CommissionRateBps)
		if err != nil {
			return err
		}
		commission := &AffiliateCommission{
			InviterUserId:   agent.UserId,
			InviteeUserId:   inviteeUserId,
			RedemptionId:    redemption.Id,
			SourceQuota:     redemption.Quota,
			RateBps:         agent.CommissionRateBps,
			CommissionQuota: commissionQuota,
			RewardType:      AffiliateRewardTypeSelectedAgent,
			Destination:     AffiliateRewardDestinationCashback,
		}
		if err := tx.Create(commission).Error; err != nil {
			return err
		}
		if commissionQuota == 0 {
			return nil
		}

		maxBeforeIncrement := int64(math.MaxInt64) - int64(commissionQuota)
		result := tx.Model(&AffiliateAgent{}).
			Where(
				"user_id = ? AND enabled = ? AND commission_rate_bps = ? "+
					"AND available_quota >= 0 AND total_commission_quota >= 0 "+
					"AND available_quota <= ? AND total_commission_quota <= ?",
				agent.UserId,
				true,
				agent.CommissionRateBps,
				maxBeforeIncrement,
				maxBeforeIncrement,
			).
			Updates(map[string]interface{}{
				"available_quota":        gorm.Expr("available_quota + ?", commissionQuota),
				"total_commission_quota": gorm.Expr("total_commission_quota + ?", commissionQuota),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAffiliateLedgerOverflow
		}
		return nil
	}

	firstRedemption, err := claimAffiliateFirstRedemptionTx(
		tx,
		invitee.InviterId,
		inviteeUserId,
		redemption.Id,
		AffiliateRewardTypeOrdinaryFirst,
	)
	if err != nil || !firstRedemption {
		return err
	}

	commissionQuota, err := CalculateAffiliateCommission(
		redemption.Quota,
		AffiliateOrdinaryFirstRateBps,
	)
	if err != nil {
		return err
	}
	commission := &AffiliateCommission{
		InviterUserId:   invitee.InviterId,
		InviteeUserId:   inviteeUserId,
		RedemptionId:    redemption.Id,
		SourceQuota:     redemption.Quota,
		RateBps:         AffiliateOrdinaryFirstRateBps,
		CommissionQuota: commissionQuota,
		RewardType:      AffiliateRewardTypeOrdinaryFirst,
		Destination:     AffiliateRewardDestinationBalance,
	}
	if err := tx.Create(commission).Error; err != nil {
		return err
	}
	if commissionQuota == 0 {
		return nil
	}

	result := tx.Model(&User{}).
		Where(
			"id = ? AND quota >= 0 AND quota <= ?",
			invitee.InviterId,
			common.MaxQuota-commissionQuota,
		).
		Update("quota", gorm.Expr("quota + ?", commissionQuota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAffiliateBalanceOverflow
	}
	return nil
}

func claimAffiliateFirstRedemptionTx(
	tx *gorm.DB,
	inviterUserId int,
	inviteeUserId int,
	redemptionId int,
	firstRewardType string,
) (bool, error) {
	claim := &AffiliateFirstRewardClaim{
		InviteeUserId:   inviteeUserId,
		InviterUserId:   inviterUserId,
		RedemptionId:    redemptionId,
		FirstRewardType: firstRewardType,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "invitee_user_id"}},
		DoNothing: true,
	}).Create(claim)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func GetAffiliateSummary(userId int) (*AffiliateSummary, error) {
	var inviteeCount int64
	if err := DB.Model(&User{}).Where("inviter_id = ?", userId).Count(&inviteeCount).Error; err != nil {
		return nil, err
	}

	type rewardAggregate struct {
		OrdinaryRewardQuota int64
		TotalRewardQuota    int64
	}
	var rewards rewardAggregate
	if err := DB.Model(&AffiliateCommission{}).
		Select(
			"COALESCE(SUM(CASE WHEN reward_type = ? THEN commission_quota ELSE 0 END), 0) AS ordinary_reward_quota, "+
				"COALESCE(SUM(commission_quota), 0) AS total_reward_quota",
			AffiliateRewardTypeOrdinaryFirst,
		).
		Where("inviter_user_id = ?", userId).
		Scan(&rewards).Error; err != nil {
		return nil, err
	}

	type redemptionAggregate struct {
		RedemptionCount int64
		RedeemedQuota   int64
	}
	var redemptions redemptionAggregate
	if err := DB.Table("redemptions").
		Joins("JOIN users AS invitee_user ON invitee_user.id = redemptions.used_user_id").
		Select(
			"COUNT(*) AS redemption_count, COALESCE(SUM(redemptions.quota), 0) AS redeemed_quota",
		).
		Where(
			"invitee_user.inviter_id = ? AND redemptions.status = ?",
			userId,
			common.RedemptionCodeStatusUsed,
		).
		Where("(redemptions.benefit_type = ? OR redemptions.benefit_type = '')", RedemptionBenefitQuota).
		Scan(&redemptions).Error; err != nil {
		return nil, err
	}

	summary := &AffiliateSummary{
		InviteeCount:        inviteeCount,
		OrdinaryRewardQuota: rewards.OrdinaryRewardQuota,
		TotalRewardQuota:    rewards.TotalRewardQuota,
		RedemptionCount:     redemptions.RedemptionCount,
		RedeemedQuota:       redemptions.RedeemedQuota,
	}

	var agent AffiliateAgent
	err := DB.First(&agent, "user_id = ?", userId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return summary, nil
	}
	if err != nil {
		return nil, err
	}

	summary.IsAgent = true
	summary.Enabled = agent.Enabled
	summary.CommissionRateBps = agent.CommissionRateBps
	summary.CashWithdrawalEnabled = agent.CashWithdrawalEnabled
	summary.AvailableQuota = agent.AvailableQuota
	summary.PendingWithdrawalQuota = agent.PendingWithdrawalQuota
	summary.ConvertedQuota = agent.ConvertedQuota
	summary.WithdrawnQuota = agent.WithdrawnQuota
	summary.TotalCommissionQuota = agent.TotalCommissionQuota
	return summary, nil
}

func GetAffiliateAgentForAdmin(userId int) (*AffiliateAgentRecord, error) {
	var user User
	if err := DB.Select("id", "username", "display_name").First(&user, "id = ?", userId).Error; err != nil {
		return nil, err
	}

	record := &AffiliateAgentRecord{
		AffiliateAgent: AffiliateAgent{UserId: userId},
		Username:       user.Username,
		DisplayName:    user.DisplayName,
	}
	err := DB.First(&record.AffiliateAgent, "user_id = ?", userId).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return record, nil
}

func UpsertAffiliateAgentConfig(userId int, enabled bool, rateBps int, cashWithdrawalEnabled bool) (*AffiliateAgent, error) {
	if userId <= 0 {
		return nil, ErrAffiliateAgentNotFound
	}
	if err := validateAffiliateConfig(enabled, rateBps); err != nil {
		return nil, err
	}

	var agent AffiliateAgent
	err := DB.Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&User{}).Where("id = ?", userId).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount != 1 {
			return gorm.ErrRecordNotFound
		}

		err := tx.First(&agent, "user_id = ?", userId).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			agent = AffiliateAgent{
				UserId:                userId,
				Enabled:               enabled,
				CommissionRateBps:     rateBps,
				CashWithdrawalEnabled: cashWithdrawalEnabled,
			}
			return tx.Create(&agent).Error
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&AffiliateAgent{}).Where("user_id = ?", userId).Updates(map[string]interface{}{
			"enabled":                 enabled,
			"commission_rate_bps":     rateBps,
			"cash_withdrawal_enabled": cashWithdrawalEnabled,
		}).Error; err != nil {
			return err
		}
		return tx.First(&agent, "user_id = ?", userId).Error
	})
	return &agent, err
}

func ListAffiliateAgents(keyword string, pageInfo *common.PageInfo) ([]*AffiliateAgentRecord, int64, error) {
	keyword = strings.TrimSpace(keyword)
	base := DB.Table("affiliate_agents AS agent").
		Joins("JOIN users ON users.id = agent.user_id")
	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("users.username LIKE ? OR users.display_name LIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []*AffiliateAgentRecord
	err := base.
		Select("agent.*, users.username, users.display_name").
		Order("agent.updated_at DESC, agent.user_id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

func listAffiliateCommissions(inviterUserId int, keyword string, pageInfo *common.PageInfo) ([]*AffiliateCommissionRecord, int64, error) {
	base := DB.Table("affiliate_commissions AS commission").
		Joins("JOIN users AS inviter_user ON inviter_user.id = commission.inviter_user_id").
		Joins("JOIN users AS invitee_user ON invitee_user.id = commission.invitee_user_id").
		Joins("JOIN redemptions ON redemptions.id = commission.redemption_id")
	if inviterUserId > 0 {
		base = base.Where("commission.inviter_user_id = ?", inviterUserId)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where(
			"inviter_user.username LIKE ? OR invitee_user.username LIKE ? OR redemptions.name LIKE ?",
			like,
			like,
			like,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []*AffiliateCommissionRecord
	err := base.
		Select(
			"commission.*, inviter_user.username AS inviter_username, " +
				"invitee_user.username AS invitee_username, redemptions.name AS redemption_name",
		).
		Order("commission.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

func ListAffiliateCommissionsForAgent(agentUserId int, pageInfo *common.PageInfo) ([]*AffiliateCommissionRecord, int64, error) {
	records, total, err := listAffiliateCommissions(agentUserId, "", pageInfo)
	if err != nil {
		return nil, 0, err
	}
	for _, record := range records {
		record.InviteeUsername = maskAffiliateAccount(record.InviteeUsername)
		record.RedemptionName = ""
	}
	return records, total, nil
}

func ListAllAffiliateCommissions(keyword string, pageInfo *common.PageInfo) ([]*AffiliateCommissionRecord, int64, error) {
	return listAffiliateCommissions(0, keyword, pageInfo)
}

func maskAffiliateAccount(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	switch len(runes) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return string(runes[0]) + "*"
	default:
		return string(runes[0]) + "***" + string(runes[len(runes)-1])
	}
}

func ListAffiliateInvitees(agentUserId int, pageInfo *common.PageInfo) ([]*AffiliateInviteeRecord, int64, error) {
	base := DB.Model(&User{}).Where("inviter_id = ?", agentUserId)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []User
	if err := base.
		Select("id", "username", "display_name", "created_at").
		Order("id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	records := make([]*AffiliateInviteeRecord, 0, len(users))
	if len(users) == 0 {
		return records, total, nil
	}

	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}
	type aggregate struct {
		InviteeUserId   int
		RedemptionCount int64
		RedeemedQuota   int64
		RewardQuota     int64
	}
	var redemptionAggregates []aggregate
	if err := DB.Table("redemptions").
		Select(
			"used_user_id AS invitee_user_id, COUNT(*) AS redemption_count, "+
				"COALESCE(SUM(quota), 0) AS redeemed_quota",
		).
		Where(
			"status = ? AND used_user_id IN ?",
			common.RedemptionCodeStatusUsed,
			userIds,
		).
		Where("(benefit_type = ? OR benefit_type = '')", RedemptionBenefitQuota).
		Group("used_user_id").
		Scan(&redemptionAggregates).Error; err != nil {
		return nil, 0, err
	}

	var rewardAggregates []aggregate
	if err := DB.Model(&AffiliateCommission{}).
		Select(
			"invitee_user_id, COALESCE(SUM(commission_quota), 0) AS reward_quota",
		).
		Where("inviter_user_id = ? AND invitee_user_id IN ?", agentUserId, userIds).
		Group("invitee_user_id").
		Scan(&rewardAggregates).Error; err != nil {
		return nil, 0, err
	}

	aggregateByUser := make(map[int]aggregate, len(redemptionAggregates))
	for _, item := range redemptionAggregates {
		aggregateByUser[item.InviteeUserId] = item
	}
	for _, reward := range rewardAggregates {
		item := aggregateByUser[reward.InviteeUserId]
		item.InviteeUserId = reward.InviteeUserId
		item.RewardQuota = reward.RewardQuota
		aggregateByUser[reward.InviteeUserId] = item
	}
	for _, user := range users {
		item := aggregateByUser[user.Id]
		records = append(records, &AffiliateInviteeRecord{
			UserId:          user.Id,
			Username:        maskAffiliateAccount(user.Username),
			DisplayName:     maskAffiliateAccount(user.DisplayName),
			CreatedAt:       user.CreatedAt,
			RedemptionCount: item.RedemptionCount,
			RedeemedQuota:   item.RedeemedQuota,
			RewardQuota:     item.RewardQuota,
		})
	}
	return records, total, nil
}

func ListAllAffiliateInvitations(keyword string, pageInfo *common.PageInfo) ([]*AffiliateInvitationRecord, int64, error) {
	keyword = strings.TrimSpace(keyword)
	base := DB.Table("users AS invitee_user").
		Joins("JOIN users AS inviter_user ON inviter_user.id = invitee_user.inviter_id").
		Where("invitee_user.inviter_id <> 0")
	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where(
			"inviter_user.username LIKE ? OR invitee_user.username LIKE ? OR invitee_user.display_name LIKE ?",
			like,
			like,
			like,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []*AffiliateInvitationRecord
	if err := base.
		Select(
			"inviter_user.id AS inviter_user_id, inviter_user.username AS inviter_username, " +
				"invitee_user.id AS invitee_user_id, invitee_user.username AS invitee_username, " +
				"invitee_user.display_name AS invitee_name, invitee_user.created_at",
		).
		Order("invitee_user.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error; err != nil {
		return nil, 0, err
	}
	if len(records) == 0 {
		return records, total, nil
	}

	inviteeIds := make([]int, 0, len(records))
	for _, record := range records {
		inviteeIds = append(inviteeIds, record.InviteeUserId)
	}

	type redemptionAggregate struct {
		InviteeUserId   int
		RedemptionCount int64
		RedeemedQuota   int64
	}
	var redemptionAggregates []redemptionAggregate
	if err := DB.Table("redemptions").
		Select(
			"used_user_id AS invitee_user_id, COUNT(*) AS redemption_count, "+
				"COALESCE(SUM(quota), 0) AS redeemed_quota",
		).
		Where(
			"status = ? AND used_user_id IN ?",
			common.RedemptionCodeStatusUsed,
			inviteeIds,
		).
		Where("(benefit_type = ? OR benefit_type = '')", RedemptionBenefitQuota).
		Group("used_user_id").
		Scan(&redemptionAggregates).Error; err != nil {
		return nil, 0, err
	}

	type rewardAggregate struct {
		InviteeUserId int
		RewardQuota   int64
	}
	var rewardAggregates []rewardAggregate
	if err := DB.Model(&AffiliateCommission{}).
		Select(
			"invitee_user_id, COALESCE(SUM(commission_quota), 0) AS reward_quota",
		).
		Where("invitee_user_id IN ?", inviteeIds).
		Group("invitee_user_id").
		Scan(&rewardAggregates).Error; err != nil {
		return nil, 0, err
	}

	redemptionByInvitee := make(map[int]redemptionAggregate, len(redemptionAggregates))
	for _, aggregate := range redemptionAggregates {
		redemptionByInvitee[aggregate.InviteeUserId] = aggregate
	}
	rewardByInvitee := make(map[int]int64, len(rewardAggregates))
	for _, aggregate := range rewardAggregates {
		rewardByInvitee[aggregate.InviteeUserId] = aggregate.RewardQuota
	}
	for _, record := range records {
		redemption := redemptionByInvitee[record.InviteeUserId]
		record.RedemptionCount = redemption.RedemptionCount
		record.RedeemedQuota = redemption.RedeemedQuota
		record.RewardQuota = rewardByInvitee[record.InviteeUserId]
	}
	return records, total, nil
}

func listAffiliateRedemptions(
	inviterUserId int,
	keyword string,
	pageInfo *common.PageInfo,
) ([]*AffiliateRedemptionRecord, int64, error) {
	base := DB.Table("redemptions").
		Joins("JOIN users AS invitee_user ON invitee_user.id = redemptions.used_user_id").
		Joins("JOIN users AS inviter_user ON inviter_user.id = invitee_user.inviter_id").
		Joins(
			"LEFT JOIN affiliate_commissions AS commission "+
				"ON commission.redemption_id = redemptions.id "+
				"AND commission.inviter_user_id = invitee_user.inviter_id",
		).
		Where("redemptions.status = ?", common.RedemptionCodeStatusUsed).
		Where("(redemptions.benefit_type = ? OR redemptions.benefit_type = '')", RedemptionBenefitQuota)
	if inviterUserId > 0 {
		base = base.Where("invitee_user.inviter_id = ?", inviterUserId)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where(
			"inviter_user.username LIKE ? OR invitee_user.username LIKE ? OR redemptions.name LIKE ?",
			like,
			like,
			like,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []*AffiliateRedemptionRecord
	err := base.
		Select(
			"redemptions.id AS redemption_id, invitee_user.inviter_id AS inviter_user_id, " +
				"inviter_user.username AS inviter_username, invitee_user.id AS invitee_user_id, " +
				"invitee_user.username AS invitee_username, redemptions.quota AS source_quota, " +
				"redemptions.redeemed_time AS redeemed_at, " +
				"COALESCE(commission.reward_type, '') AS reward_type, " +
				"COALESCE(commission.destination, '') AS destination, " +
				"COALESCE(commission.rate_bps, 0) AS rate_bps, " +
				"COALESCE(commission.commission_quota, 0) AS reward_quota",
		).
		Order("redemptions.redeemed_time DESC, redemptions.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

func ListAffiliateRedemptionsForInviter(inviterUserId int, pageInfo *common.PageInfo) ([]*AffiliateRedemptionRecord, int64, error) {
	records, total, err := listAffiliateRedemptions(inviterUserId, "", pageInfo)
	if err != nil {
		return nil, 0, err
	}
	for _, record := range records {
		record.InviteeUsername = maskAffiliateAccount(record.InviteeUsername)
	}
	return records, total, nil
}

func ListAllAffiliateRedemptions(keyword string, pageInfo *common.PageInfo) ([]*AffiliateRedemptionRecord, int64, error) {
	return listAffiliateRedemptions(0, keyword, pageInfo)
}

func ConvertAffiliateCashbackToBalance(agentUserId int, amountQuota int64) (*AffiliateConversion, error) {
	if err := validateAffiliateAmount(amountQuota); err != nil {
		return nil, err
	}

	conversion := &AffiliateConversion{AgentUserId: agentUserId, AmountQuota: amountQuota}
	err := DB.Transaction(func(tx *gorm.DB) error {
		maxBeforeIncrement := int64(math.MaxInt64) - amountQuota
		result := tx.Model(&AffiliateAgent{}).
			Where(
				"user_id = ? AND available_quota >= ? "+
					"AND converted_quota >= 0 AND converted_quota <= ?",
				agentUserId,
				amountQuota,
				maxBeforeIncrement,
			).
			Updates(map[string]interface{}{
				"available_quota": gorm.Expr("available_quota - ?", amountQuota),
				"converted_quota": gorm.Expr("converted_quota + ?", amountQuota),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var count int64
			if err := tx.Model(&AffiliateAgent{}).Where("user_id = ?", agentUserId).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrAffiliateAgentNotFound
			}
			var agent AffiliateAgent
			if err := tx.First(&agent, "user_id = ?", agentUserId).Error; err != nil {
				return err
			}
			if agent.AvailableQuota >= amountQuota && agent.ConvertedQuota > maxBeforeIncrement {
				return ErrAffiliateLedgerOverflow
			}
			return ErrAffiliateBalanceInsufficient
		}

		amount := int(amountQuota)
		userResult := tx.Model(&User{}).
			Where("id = ? AND quota <= ?", agentUserId, common.MaxQuota-amount).
			Update("quota", gorm.Expr("quota + ?", amount))
		if userResult.Error != nil {
			return userResult.Error
		}
		if userResult.RowsAffected != 1 {
			return ErrAffiliateBalanceOverflow
		}
		return tx.Create(conversion).Error
	})
	if err != nil {
		return nil, err
	}
	RecordLog(agentUserId, LogTypeSystem, fmt.Sprintf("代理返现转入站内余额 %s", logger.LogQuota(int(amountQuota))))
	return conversion, nil
}

func CreateAffiliateWithdrawal(agentUserId int, amountQuota int64, applicantNote string) (*AffiliateWithdrawal, error) {
	if err := validateAffiliateAmount(amountQuota); err != nil {
		return nil, err
	}
	applicantNote = strings.TrimSpace(applicantNote)
	if len([]rune(applicantNote)) > 500 {
		return nil, errors.New("提现备注不能超过 500 个字符")
	}

	withdrawal := &AffiliateWithdrawal{
		AgentUserId:   agentUserId,
		AmountQuota:   amountQuota,
		Status:        AffiliateWithdrawalStatusPending,
		ApplicantNote: applicantNote,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		maxBeforeIncrement := int64(math.MaxInt64) - amountQuota
		result := tx.Model(&AffiliateAgent{}).
			Where(
				"user_id = ? AND cash_withdrawal_enabled = ? AND available_quota >= ? "+
					"AND pending_withdrawal_quota >= 0 AND pending_withdrawal_quota <= ?",
				agentUserId,
				true,
				amountQuota,
				maxBeforeIncrement,
			).
			Updates(map[string]interface{}{
				"available_quota":          gorm.Expr("available_quota - ?", amountQuota),
				"pending_withdrawal_quota": gorm.Expr("pending_withdrawal_quota + ?", amountQuota),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var agent AffiliateAgent
			if err := tx.First(&agent, "user_id = ?", agentUserId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAffiliateAgentNotFound
				}
				return err
			}
			if !agent.CashWithdrawalEnabled {
				return ErrAffiliateCashWithdrawalDisabled
			}
			if agent.AvailableQuota >= amountQuota && agent.PendingWithdrawalQuota > maxBeforeIncrement {
				return ErrAffiliateLedgerOverflow
			}
			return ErrAffiliateBalanceInsufficient
		}
		return tx.Create(withdrawal).Error
	})
	if err != nil {
		return nil, err
	}
	RecordLog(agentUserId, LogTypeSystem, fmt.Sprintf("提交代理返现提现申请 %s", logger.LogQuota(int(amountQuota))))
	return withdrawal, nil
}

func listAffiliateConversions(agentUserId int, pageInfo *common.PageInfo) ([]*AffiliateConversionRecord, int64, error) {
	base := DB.Table("affiliate_conversions AS conversion").
		Joins("JOIN users AS agent_user ON agent_user.id = conversion.agent_user_id")
	if agentUserId > 0 {
		base = base.Where("conversion.agent_user_id = ?", agentUserId)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []*AffiliateConversionRecord
	err := base.
		Select("conversion.*, agent_user.username AS agent_username").
		Order("conversion.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

func ListAffiliateConversionsForAgent(agentUserId int, pageInfo *common.PageInfo) ([]*AffiliateConversionRecord, int64, error) {
	return listAffiliateConversions(agentUserId, pageInfo)
}

func ListAllAffiliateConversions(pageInfo *common.PageInfo) ([]*AffiliateConversionRecord, int64, error) {
	return listAffiliateConversions(0, pageInfo)
}

func listAffiliateWithdrawals(agentUserId int, status string, pageInfo *common.PageInfo) ([]*AffiliateWithdrawalRecord, int64, error) {
	base := DB.Table("affiliate_withdrawals AS withdrawal").
		Joins("JOIN users AS agent_user ON agent_user.id = withdrawal.agent_user_id")
	if agentUserId > 0 {
		base = base.Where("withdrawal.agent_user_id = ?", agentUserId)
	}
	if status != "" {
		base = base.Where("withdrawal.status = ?", status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []*AffiliateWithdrawalRecord
	err := base.
		Select("withdrawal.*, agent_user.username AS agent_username").
		Order("withdrawal.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

func ListAffiliateWithdrawalsForAgent(agentUserId int, pageInfo *common.PageInfo) ([]*AffiliateWithdrawalRecord, int64, error) {
	return listAffiliateWithdrawals(agentUserId, "", pageInfo)
}

func ListAllAffiliateWithdrawals(status string, pageInfo *common.PageInfo) ([]*AffiliateWithdrawalRecord, int64, error) {
	switch status {
	case "", AffiliateWithdrawalStatusPending, AffiliateWithdrawalStatusPaid, AffiliateWithdrawalStatusRejected:
	default:
		return nil, 0, errors.New("无效的提现状态")
	}
	return listAffiliateWithdrawals(0, status, pageInfo)
}

func GetAffiliateWithdrawalById(id int) (*AffiliateWithdrawal, error) {
	var withdrawal AffiliateWithdrawal
	err := DB.First(&withdrawal, "id = ?", id).Error
	return &withdrawal, err
}

func ReviewAffiliateWithdrawal(id int, reviewerUserId int, paid bool, adminNote string) (*AffiliateWithdrawal, error) {
	adminNote = strings.TrimSpace(adminNote)
	if len([]rune(adminNote)) > 500 {
		return nil, errors.New("审核备注不能超过 500 个字符")
	}

	var withdrawal AffiliateWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&withdrawal, "id = ?", id).Error; err != nil {
			return err
		}
		if withdrawal.Status != AffiliateWithdrawalStatusPending {
			return ErrAffiliateWithdrawalNotPending
		}

		now := common.GetTimestamp()
		targetStatus := AffiliateWithdrawalStatusRejected
		updates := map[string]interface{}{
			"status":           targetStatus,
			"reviewer_user_id": reviewerUserId,
			"reviewed_at":      now,
			"admin_note":       adminNote,
		}
		if paid {
			targetStatus = AffiliateWithdrawalStatusPaid
			updates["status"] = targetStatus
			updates["paid_at"] = now
		}
		result := tx.Model(&AffiliateWithdrawal{}).
			Where("id = ? AND status = ?", id, AffiliateWithdrawalStatusPending).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAffiliateWithdrawalNotPending
		}

		agentUpdates := map[string]interface{}{
			"pending_withdrawal_quota": gorm.Expr("pending_withdrawal_quota - ?", withdrawal.AmountQuota),
		}
		agentWhere := "user_id = ? AND pending_withdrawal_quota >= ?"
		agentWhereArgs := []interface{}{withdrawal.AgentUserId, withdrawal.AmountQuota}
		maxBeforeIncrement := int64(math.MaxInt64) - withdrawal.AmountQuota
		if paid {
			agentUpdates["withdrawn_quota"] = gorm.Expr("withdrawn_quota + ?", withdrawal.AmountQuota)
			agentWhere += " AND withdrawn_quota >= 0 AND withdrawn_quota <= ?"
			agentWhereArgs = append(agentWhereArgs, maxBeforeIncrement)
		} else {
			agentUpdates["available_quota"] = gorm.Expr("available_quota + ?", withdrawal.AmountQuota)
			agentWhere += " AND available_quota >= 0 AND available_quota <= ?"
			agentWhereArgs = append(agentWhereArgs, maxBeforeIncrement)
		}
		agentResult := tx.Model(&AffiliateAgent{}).
			Where(agentWhere, agentWhereArgs...).
			Updates(agentUpdates)
		if agentResult.Error != nil {
			return agentResult.Error
		}
		if agentResult.RowsAffected != 1 {
			var agent AffiliateAgent
			if err := tx.First(&agent, "user_id = ?", withdrawal.AgentUserId).Error; err != nil {
				return err
			}
			if agent.PendingWithdrawalQuota < withdrawal.AmountQuota {
				return ErrAffiliateBalanceInsufficient
			}
			return ErrAffiliateLedgerOverflow
		}
		return tx.First(&withdrawal, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}

	action := "管理员拒绝代理返现提现"
	if paid {
		action = "管理员确认代理返现已线下付款"
	}
	RecordLog(
		withdrawal.AgentUserId,
		LogTypeSystem,
		fmt.Sprintf("%s %s，申请ID %d", action, logger.LogQuota(int(withdrawal.AmountQuota)), withdrawal.Id),
	)
	return &withdrawal, nil
}
