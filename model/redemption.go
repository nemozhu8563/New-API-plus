package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

const (
	RedemptionBenefitQuota        = "quota"
	RedemptionBenefitSubscription = "subscription"
)

var (
	ErrSubscriptionRedemptionPlanDisabled = errors.New("subscription plan is disabled")
	ErrRedemptionStatusInvalid            = errors.New("兑换码状态无效")
	ErrRedemptionStatusImmutable          = errors.New("已使用的兑换码状态不可修改")
)

type redemptionSubscriptionSnapshot struct {
	PlanId                  int    `json:"plan_id"`
	Title                   string `json:"title"`
	DurationUnit            string `json:"duration_unit"`
	DurationValue           int    `json:"duration_value"`
	CustomSeconds           int64  `json:"custom_seconds"`
	MaxPurchasePerUser      int    `json:"max_purchase_per_user"`
	UpgradeGroup            string `json:"upgrade_group"`
	DowngradeGroup          string `json:"downgrade_group"`
	TotalAmount             int64  `json:"total_amount"`
	QuotaResetPeriod        string `json:"quota_reset_period"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds"`
	AllowWalletOverflow     bool   `json:"allow_wallet_overflow"`
}

type RedemptionResult struct {
	Type           string `json:"type"`
	Quota          int    `json:"quota,omitempty"`
	SubscriptionId int    `json:"subscription_id,omitempty"`
	PlanId         int    `json:"plan_id,omitempty"`
	PlanTitle      string `json:"plan_title,omitempty"`
	StartTime      int64  `json:"start_time,omitempty"`
	EndTime        int64  `json:"end_time,omitempty"`
}

type Redemption struct {
	Id                       int            `json:"id"`
	UserId                   int            `json:"user_id"`
	Key                      string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status                   int            `json:"status" gorm:"default:1"`
	Name                     string         `json:"name" gorm:"index"`
	Quota                    int            `json:"quota" gorm:"default:100"`
	BenefitType              string         `json:"benefit_type" gorm:"type:varchar(32);not null;default:'quota';index"`
	SubscriptionPlanId       int            `json:"subscription_plan_id" gorm:"index"`
	SubscriptionPlanTitle    string         `json:"subscription_plan_title" gorm:"type:varchar(128);default:''"`
	SubscriptionPlanSnapshot string         `json:"-" gorm:"type:text"`
	UsedSubscriptionId       int            `json:"used_subscription_id" gorm:"index"`
	CreatedTime              int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime             int64          `json:"redeemed_time" gorm:"bigint"`
	Count                    int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId               int            `json:"used_user_id"`
	DeletedAt                gorm.DeletedAt `gorm:"index"`
	ExpiredTime              int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

func NormalizeRedemptionBenefitType(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", RedemptionBenefitQuota:
		return RedemptionBenefitQuota, nil
	case RedemptionBenefitSubscription:
		return RedemptionBenefitSubscription, nil
	default:
		return "", errors.New("invalid redemption benefit type")
	}
}

// FreezeSubscriptionPlan copies every plan field that affects the entitlement.
// Redemption uses this snapshot even if the source plan is later edited,
// disabled, or deleted.
func (redemption *Redemption) FreezeSubscriptionPlan(planId int) error {
	if planId <= 0 {
		return errors.New("invalid subscription plan id")
	}
	var plan SubscriptionPlan
	if err := DB.Where("id = ?", planId).First(&plan).Error; err != nil {
		return err
	}
	plan.NormalizeDefaults()
	if !plan.Enabled {
		return ErrSubscriptionRedemptionPlanDisabled
	}
	allowWalletOverflow := true
	if plan.AllowWalletOverflow != nil {
		allowWalletOverflow = *plan.AllowWalletOverflow
	}
	snapshot := redemptionSubscriptionSnapshot{
		PlanId:                  plan.Id,
		Title:                   plan.Title,
		DurationUnit:            plan.DurationUnit,
		DurationValue:           plan.DurationValue,
		CustomSeconds:           plan.CustomSeconds,
		MaxPurchasePerUser:      plan.MaxPurchasePerUser,
		UpgradeGroup:            plan.UpgradeGroup,
		DowngradeGroup:          plan.DowngradeGroup,
		TotalAmount:             plan.TotalAmount,
		QuotaResetPeriod:        plan.QuotaResetPeriod,
		QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
		AllowWalletOverflow:     allowWalletOverflow,
	}
	data, err := common.Marshal(snapshot)
	if err != nil {
		return err
	}
	redemption.BenefitType = RedemptionBenefitSubscription
	redemption.SubscriptionPlanId = plan.Id
	redemption.SubscriptionPlanTitle = plan.Title
	redemption.SubscriptionPlanSnapshot = string(data)
	redemption.Quota = 0
	return nil
}

func (redemption *Redemption) subscriptionPlanFromSnapshot() (*SubscriptionPlan, error) {
	if redemption.SubscriptionPlanId <= 0 || redemption.SubscriptionPlanSnapshot == "" {
		return nil, errors.New("subscription redemption snapshot is missing")
	}
	var snapshot redemptionSubscriptionSnapshot
	if err := common.UnmarshalJsonStr(redemption.SubscriptionPlanSnapshot, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.PlanId != redemption.SubscriptionPlanId || snapshot.PlanId <= 0 || strings.TrimSpace(snapshot.Title) == "" {
		return nil, errors.New("subscription redemption snapshot is invalid")
	}
	return &SubscriptionPlan{
		Id:                      snapshot.PlanId,
		Title:                   snapshot.Title,
		DurationUnit:            snapshot.DurationUnit,
		DurationValue:           snapshot.DurationValue,
		CustomSeconds:           snapshot.CustomSeconds,
		AllowWalletOverflow:     common.GetPointer(snapshot.AllowWalletOverflow),
		MaxPurchasePerUser:      snapshot.MaxPurchasePerUser,
		UpgradeGroup:            snapshot.UpgradeGroup,
		DowngradeGroup:          snapshot.DowngradeGroup,
		TotalAmount:             snapshot.TotalAmount,
		QuotaResetPeriod:        snapshot.QuotaResetPeriod,
		QuotaResetCustomSeconds: snapshot.QuotaResetCustomSeconds,
	}, nil
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	err = tx.Model(&Redemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, status string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Redemption{})

	if keyword != "" {
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
		} else {
			query = query.Where("name LIKE ?", keyword+"%")
		}
	}

	if status != "" {
		now := common.GetTimestamp()
		switch status {
		case "expired":
			query = query.Where(
				"status = ? AND expired_time != 0 AND expired_time < ?",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusEnabled):
			query = query.Where(
				"status = ? AND (expired_time = 0 OR expired_time >= ?)",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusDisabled):
			query = query.Where("status = ?", common.RedemptionCodeStatusDisabled)
		case strconv.Itoa(common.RedemptionCodeStatusUsed):
			query = query.Where("status = ?", common.RedemptionCodeStatusUsed)
		}
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (quota int, err error) {
	result, err := redeemCode(key, userId, RedemptionBenefitQuota)
	if err != nil {
		return 0, err
	}
	return result.Quota, nil
}

func RedeemCode(key string, userId int) (*RedemptionResult, error) {
	return redeemCode(key, userId, "")
}

func redeemCode(key string, userId int, requiredBenefitType string) (*RedemptionResult, error) {
	if key == "" {
		return nil, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return nil, errors.New("无效的 user id")
	}
	redemption := &Redemption{}
	var result *RedemptionResult
	groupChanged := false

	keyCol := "`key`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		keyCol = `"key"`
	}
	common.RandomSleep()
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}
		benefitType, err := NormalizeRedemptionBenefitType(redemption.BenefitType)
		if err != nil {
			return err
		}
		if requiredBenefitType != "" && benefitType != requiredBenefitType {
			return errors.New("兑换码类型不适用于该接口")
		}

		switch benefitType {
		case RedemptionBenefitQuota:
			if redemption.Quota <= 0 || redemption.Quota > common.MaxQuota {
				return errors.New("兑换码额度无效")
			}
			// Compare-and-swap on status: only the transaction that flips
			// enabled -> used may credit quota, so a concurrent redeem of the
			// same code loses here even without a row lock (e.g. on SQLite).
			updateResult := tx.Model(&Redemption{}).
				Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
				Updates(map[string]interface{}{
					"redeemed_time": common.GetTimestamp(),
					"status":        common.RedemptionCodeStatusUsed,
					"used_user_id":  userId,
				})
			if updateResult.Error != nil {
				return updateResult.Error
			}
			if updateResult.RowsAffected == 0 {
				return errors.New("该兑换码已被使用")
			}
			userResult := tx.Model(&User{}).
				Where("id = ? AND quota <= ?", userId, common.MaxQuota-redemption.Quota).
				Update("quota", gorm.Expr("quota + ?", redemption.Quota))
			if userResult.Error != nil {
				return userResult.Error
			}
			if userResult.RowsAffected != 1 {
				return errors.New("用户不存在或余额将超过系统上限")
			}
			if err := createAffiliateCommissionForRedemptionTx(tx, redemption, userId); err != nil {
				return err
			}
			result = &RedemptionResult{
				Type:  RedemptionBenefitQuota,
				Quota: redemption.Quota,
			}
		case RedemptionBenefitSubscription:
			plan, err := redemption.subscriptionPlanFromSnapshot()
			if err != nil {
				return err
			}
			// Serialize subscription creation for one user so the snapshotted
			// purchase limit cannot be bypassed by redeeming different codes
			// concurrently.
			var lockedUser User
			if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&lockedUser).Error; err != nil {
				return errors.New("用户不存在")
			}
			subscription, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "redemption")
			if err != nil {
				return err
			}
			updateResult := tx.Model(&Redemption{}).
				Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
				Updates(map[string]interface{}{
					"redeemed_time":        common.GetTimestamp(),
					"status":               common.RedemptionCodeStatusUsed,
					"used_user_id":         userId,
					"used_subscription_id": subscription.Id,
				})
			if updateResult.Error != nil {
				return updateResult.Error
			}
			if updateResult.RowsAffected == 0 {
				return errors.New("该兑换码已被使用")
			}
			groupChanged = subscription.PrevUserGroup != ""
			result = &RedemptionResult{
				Type:           RedemptionBenefitSubscription,
				SubscriptionId: subscription.Id,
				PlanId:         subscription.PlanId,
				PlanTitle:      subscription.PlanTitle,
				StartTime:      subscription.StartTime,
				EndTime:        subscription.EndTime,
			}
		}
		return nil
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return nil, ErrRedeemFailed
	}
	if result == nil {
		return nil, ErrRedeemFailed
	}
	if result.Type == RedemptionBenefitSubscription {
		if groupChanged {
			refreshSubscriptionUserGroupCache(userId, "subscription redemption")
		}
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码激活订阅 %s，兑换码ID %d，订阅ID %d", result.PlanTitle, redemption.Id, result.SubscriptionId))
		return result, nil
	}
	syncCreditUserQuotaCache(userId, redemption.Quota, "redemption")
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return result, nil
}

func (redemption *Redemption) Insert() error {
	benefitType, err := NormalizeRedemptionBenefitType(redemption.BenefitType)
	if err != nil {
		return err
	}
	redemption.BenefitType = benefitType
	if benefitType == RedemptionBenefitQuota {
		return DB.Create(redemption).Error
	}
	if redemption.Quota != 0 {
		return errors.New("subscription redemption quota must be zero")
	}
	status := redemption.Status
	if status == 0 {
		status = common.RedemptionCodeStatusEnabled
	}
	// Map-based Create preserves quota=0 even though the legacy quota column
	// has a database default used by older callers.
	return DB.Model(&Redemption{}).Create(map[string]interface{}{
		"user_id":                    redemption.UserId,
		"key":                        redemption.Key,
		"status":                     status,
		"name":                       redemption.Name,
		"quota":                      0,
		"benefit_type":               redemption.BenefitType,
		"subscription_plan_id":       redemption.SubscriptionPlanId,
		"subscription_plan_title":    redemption.SubscriptionPlanTitle,
		"subscription_plan_snapshot": redemption.SubscriptionPlanSnapshot,
		"used_subscription_id":       redemption.UsedSubscriptionId,
		"created_time":               redemption.CreatedTime,
		"redeemed_time":              redemption.RedeemedTime,
		"used_user_id":               redemption.UsedUserId,
		"expired_time":               redemption.ExpiredTime,
	}).Error
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update updates editable redemption metadata. Redemption status and usage audit
// fields are changed only by UpdateStatus or the redeem transaction.
func (redemption *Redemption) Update() error {
	return DB.Model(redemption).Select("name", "quota", "expired_time").Updates(redemption).Error
}

func (redemption *Redemption) UpdateStatus() error {
	if redemption.Status != common.RedemptionCodeStatusEnabled && redemption.Status != common.RedemptionCodeStatusDisabled {
		return ErrRedemptionStatusInvalid
	}

	result := DB.Model(&Redemption{}).
		Where("id = ? AND status IN ?", redemption.Id, []int{
			common.RedemptionCodeStatusEnabled,
			common.RedemptionCodeStatusDisabled,
		}).
		Update("status", redemption.Status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var stored Redemption
	if err := DB.Select("status").First(&stored, "id = ?", redemption.Id).Error; err != nil {
		return err
	}
	if stored.Status == common.RedemptionCodeStatusUsed {
		return ErrRedemptionStatusImmutable
	}
	if stored.Status == redemption.Status {
		return nil
	}
	return ErrRedemptionStatusInvalid
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
