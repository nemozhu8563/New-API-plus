package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TopUp struct {
	Id                    int     `json:"id"`
	UserId                int     `json:"user_id" gorm:"index"`
	Amount                int64   `json:"amount"`
	Money                 float64 `json:"money"`
	CreditedQuota         int64   `json:"credited_quota" gorm:"type:bigint;not null;default:0"`
	ExpectedAmountMinor   int64   `json:"expected_amount_minor" gorm:"type:bigint;not null;default:0"`
	ExpectedCurrency      string  `json:"expected_currency" gorm:"type:varchar(8);default:''"`
	TradeNo               string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod         string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider       string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	ProviderOrderId       string  `json:"provider_order_id" gorm:"type:varchar(255);default:'';index"`
	ProviderProductId     string  `json:"provider_product_id" gorm:"type:varchar(255);default:''"`
	ProviderCustomerId    string  `json:"provider_customer_id" gorm:"type:varchar(255);default:'';index"`
	ProviderPaymentIntent string  `json:"provider_payment_intent" gorm:"type:varchar(255);default:'';index"`
	ProviderChargeId      string  `json:"provider_charge_id" gorm:"type:varchar(255);default:'';index"`
	ProviderLivemode      bool    `json:"provider_livemode"`
	CreateTime            int64   `json:"create_time"`
	CompleteTime          int64   `json:"complete_time"`
	Status                string  `json:"status"`
}

const (
	PaymentMethodStripe       = "stripe"
	PaymentMethodCreem        = "creem"
	PaymentMethodWaffo        = "waffo"
	PaymentMethodWaffoPancake = "waffo_pancake"
	PaymentMethodBalance      = "balance"
)

const (
	PaymentProviderEpay         = "epay"
	PaymentProviderStripe       = "stripe"
	PaymentProviderCreem        = "creem"
	PaymentProviderWaffo        = "waffo"
	PaymentProviderWaffoPancake = "waffo_pancake"
	PaymentProviderBalance      = "balance"
)

var (
	ErrPaymentMethodMismatch  = errors.New("payment method mismatch")
	ErrTopUpNotFound          = errors.New("topup not found")
	ErrTopUpStatusInvalid     = errors.New("topup status invalid")
	ErrStripeCheckoutUnbound  = errors.New("stripe checkout is not bound")
	ErrStripeSnapshotMismatch = errors.New("stripe payment does not match the immutable order snapshot")
)

func (topUp *TopUp) Insert() error {
	var err error
	err = DB.Create(topUp).Error
	return err
}

func (topUp *TopUp) Update() error {
	var err error
	err = DB.Save(topUp).Error
	return err
}

type StripeCheckoutBinding struct {
	OrderId     string
	ProductId   string
	CustomerId  string
	AmountMinor int64
	Currency    string
	Livemode    bool
}

func (topUp *TopUp) BindStripeCheckout(binding StripeCheckoutBinding) error {
	if topUp == nil || topUp.Id == 0 || strings.TrimSpace(binding.OrderId) == "" ||
		strings.TrimSpace(binding.ProductId) == "" || binding.AmountMinor <= 0 ||
		strings.TrimSpace(binding.Currency) == "" {
		return errors.New("invalid Stripe Checkout binding")
	}
	if topUp.PaymentProvider != PaymentProviderStripe || topUp.PaymentMethod != PaymentMethodStripe {
		return ErrPaymentMethodMismatch
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var stored TopUp
		if err := lockForUpdate(tx).Where("id = ?", topUp.Id).First(&stored).Error; err != nil {
			return err
		}
		if stored.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}
		if stored.ProviderOrderId != "" && stored.ProviderOrderId != binding.OrderId {
			return fmt.Errorf("%w: Checkout Session already bound", ErrStripeSnapshotMismatch)
		}
		if stored.ProviderProductId != "" && stored.ProviderProductId != binding.ProductId {
			return fmt.Errorf("%w: Price already bound", ErrStripeSnapshotMismatch)
		}
		if stored.ExpectedAmountMinor > 0 && stored.ExpectedAmountMinor != binding.AmountMinor {
			return fmt.Errorf("%w: Checkout amount changed", ErrStripeSnapshotMismatch)
		}
		if stored.ExpectedCurrency != "" && !strings.EqualFold(stored.ExpectedCurrency, binding.Currency) {
			return fmt.Errorf("%w: Checkout currency changed", ErrStripeSnapshotMismatch)
		}

		stored.ProviderOrderId = binding.OrderId
		stored.ProviderProductId = binding.ProductId
		stored.ProviderCustomerId = binding.CustomerId
		stored.ExpectedAmountMinor = binding.AmountMinor
		stored.ExpectedCurrency = strings.ToUpper(binding.Currency)
		stored.ProviderLivemode = binding.Livemode
		if err := tx.Save(&stored).Error; err != nil {
			return err
		}
		*topUp = stored
		return nil
	})
}

func GetTopUpById(id int) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("id = ?", id).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

func GetTopUpByTradeNo(tradeNo string) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("trade_no = ?", tradeNo).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

func UpdatePendingTopUpStatus(tradeNo string, expectedPaymentProvider string, targetStatus string) error {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if expectedPaymentProvider != "" && topUp.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status != common.TopUpStatusPending {
			return nil
		}

		topUp.Status = targetStatus
		return tx.Save(topUp).Error
	})
}

type StripeTopUpSettlement struct {
	CustomerId      string
	PaymentIntentId string
	ChargeId        string
	AmountMinor     int64
	Currency        string
	Livemode        bool
}

func Recharge(referenceId string, settlement StripeTopUpSettlement, callerIp string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}
	if settlement.PaymentIntentId == "" || settlement.AmountMinor <= 0 || strings.TrimSpace(settlement.Currency) == "" {
		return fmt.Errorf("%w: Stripe 充值结算数据不完整", ErrStripeSnapshotMismatch)
	}

	var quota int
	var credited bool
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderStripe {
			return ErrPaymentMethodMismatch
		}

		if topUp.ProviderOrderId == "" || topUp.ProviderProductId == "" ||
			topUp.ExpectedAmountMinor <= 0 || topUp.ExpectedCurrency == "" {
			return fmt.Errorf("%w: Stripe 充值订单缺少不可变支付快照", ErrStripeCheckoutUnbound)
		}
		if settlement.AmountMinor != topUp.ExpectedAmountMinor ||
			!strings.EqualFold(settlement.Currency, topUp.ExpectedCurrency) ||
			settlement.Livemode != topUp.ProviderLivemode {
			return fmt.Errorf("%w: Stripe 充值结算金额、币种或模式不匹配", ErrStripeSnapshotMismatch)
		}
		if topUp.ProviderCustomerId != "" && settlement.CustomerId != topUp.ProviderCustomerId {
			return fmt.Errorf("%w: Stripe Customer 与订单快照不匹配", ErrStripeSnapshotMismatch)
		}
		if topUp.ProviderPaymentIntent != "" && settlement.PaymentIntentId != "" && settlement.PaymentIntentId != topUp.ProviderPaymentIntent {
			return fmt.Errorf("%w: Stripe PaymentIntent 与订单不匹配", ErrStripeSnapshotMismatch)
		}
		if topUp.ProviderChargeId != "" && settlement.ChargeId != "" && settlement.ChargeId != topUp.ProviderChargeId {
			return fmt.Errorf("%w: Stripe Charge 与订单不匹配", ErrStripeSnapshotMismatch)
		}
		if topUp.Status == common.TopUpStatusSuccess {
			if settlement.PaymentIntentId != topUp.ProviderPaymentIntent ||
				settlement.ChargeId != topUp.ProviderChargeId {
				return fmt.Errorf("%w: completed Stripe settlement changed", ErrStripeSnapshotMismatch)
			}
			return nil
		}
		if topUp.Status != common.TopUpStatusPending && topUp.Status != common.TopUpStatusExpired {
			return ErrTopUpStatusInvalid
		}

		if topUp.CreditedQuota <= 0 || topUp.CreditedQuota > int64(common.MaxQuota) {
			return errors.New("无效的充值额度")
		}
		quota = int(topUp.CreditedQuota)

		var user User
		if err := lockForUpdate(tx).Where("id = ?", topUp.UserId).First(&user).Error; err != nil {
			return errors.New("用户不存在")
		}
		debtPayment := topUp.CreditedQuota
		if debtPayment > user.BillingDebt {
			debtPayment = user.BillingDebt
		}
		walletCredit := topUp.CreditedQuota - debtPayment
		if walletCredit < 0 || int64(user.Quota)+walletCredit > int64(common.MaxQuota) {
			return errors.New("用户余额将超过系统上限")
		}
		if debtPayment > 0 {
			if _, err := applyStripeBillingDebtPaymentTx(tx, topUp.UserId, debtPayment); err != nil {
				return err
			}
		}
		user.StripeCustomer = settlement.CustomerId
		user.Quota += int(walletCredit)
		user.BillingDebt -= debtPayment
		if user.BillingDebt < 0 {
			return errors.New("用户支付欠款不能为负数")
		}
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"stripe_customer": user.StripeCustomer,
			"quota":           user.Quota,
			"billing_debt":    user.BillingDebt,
		}).Error; err != nil {
			return err
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		topUp.ProviderCustomerId = settlement.CustomerId
		topUp.ProviderPaymentIntent = settlement.PaymentIntentId
		topUp.ProviderChargeId = settlement.ChargeId
		topUp.ProviderLivemode = settlement.Livemode
		if err := registerStripeTopUpPaymentTx(tx, topUp, settlement); err != nil {
			return err
		}
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}
		credited = true

		return nil
	})

	if err != nil {
		common.SysError("topup failed: " + err.Error())
		return err
	}

	if credited {
		if err := InvalidateUserCache(topUp.UserId); err != nil {
			common.SysError("failed to invalidate user cache after Stripe topup: " + err.Error())
		}
		RecordTopupLog(topUp.UserId, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%d", logger.FormatQuota(quota), topUp.Amount), callerIp, topUp.PaymentMethod, PaymentMethodStripe)
	}

	return nil
}

// topUpQueryWindowSeconds 限制充值记录查询的时间窗口（秒）。
const topUpQueryWindowSeconds int64 = 30 * 24 * 60 * 60

// topUpQueryCutoff 返回允许查询的最早 create_time（秒级 Unix 时间戳）。
func topUpQueryCutoff() int64 {
	return common.GetTimestamp() - topUpQueryWindowSeconds
}

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	cutoff := topUpQueryCutoff()

	// Get total count within transaction
	err = tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, cutoff).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated topups within same transaction
	err = tx.Where("user_id = ? AND create_time >= ?", userId, cutoff).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// GetAllTopUps 获取全平台的充值记录（管理员使用，不限制时间窗口）
func GetAllTopUps(pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err = tx.Model(&TopUp{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// searchTopUpCountHardLimit 搜索充值记录时 COUNT 的安全上限，
// 防止对超大表执行无界 COUNT 触发 DoS。
const searchTopUpCountHardLimit = 10000

// SearchUserTopUps 按订单号搜索某用户的充值记录
func SearchUserTopUps(userId int, keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, topUpQueryCutoff())
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号搜索全平台充值记录（管理员使用，不限制时间窗口）
func SearchAllTopUps(keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{})
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// ManualCompleteTopUp 管理员手动完成订单并给用户充值
func ManualCompleteTopUp(tradeNo string, callerIp string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var payMoney float64
	var paymentMethod string

	err := DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		// 行级锁，避免并发补单
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return errors.New("充值订单不存在")
		}

		// 幂等处理：已成功直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("订单状态不是待支付，无法补单")
		}

		// 计算应充值额度：
		// - Stripe 订单：Money 代表经分组倍率换算后的美元数量，直接 * QuotaPerUnit
		// - 其他订单（如易支付）：Amount 为美元数量，* QuotaPerUnit
		if topUp.PaymentProvider == PaymentProviderStripe {
			dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
			quotaToAdd = int(decimal.NewFromFloat(topUp.Money).Mul(dQuotaPerUnit).IntPart())
		} else {
			dAmount := decimal.NewFromInt(topUp.Amount)
			dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
			quotaToAdd = int(dAmount.Mul(dQuotaPerUnit).IntPart())
		}
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		// 标记完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		// 增加用户额度（立即写库，保持一致性）
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		userId = topUp.UserId
		payMoney = topUp.Money
		paymentMethod = topUp.PaymentMethod
		return nil
	})

	if err != nil {
		return err
	}

	// 事务外记录日志，避免阻塞
	RecordTopupLog(userId, fmt.Sprintf("管理员补单成功，充值金额: %v，支付金额：%f", logger.FormatQuota(quotaToAdd), payMoney), callerIp, paymentMethod, "admin")
	return nil
}
func RechargeCreem(referenceId string, customerEmail string, customerName string, callerIp string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quota int64
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderCreem {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		// Creem 直接使用 Amount 作为充值额度（整数）
		quota = topUp.Amount

		// 构建更新字段，优先使用邮箱，如果邮箱为空则使用用户名
		updateFields := map[string]interface{}{
			"quota": gorm.Expr("quota + ?", quota),
		}

		// 如果有客户邮箱，尝试更新用户邮箱（仅当用户邮箱为空时）
		if customerEmail != "" {
			// 先检查用户当前邮箱是否为空
			var user User
			err = tx.Where("id = ?", topUp.UserId).First(&user).Error
			if err != nil {
				return err
			}

			// 如果用户邮箱为空，则更新为支付时使用的邮箱
			if user.Email == "" {
				updateFields["email"] = customerEmail
			}
		}

		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(updateFields).Error
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		common.SysError("creem topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	RecordTopupLog(topUp.UserId, fmt.Sprintf("使用Creem充值成功，充值额度: %v，支付金额：%.2f", quota, topUp.Money), callerIp, topUp.PaymentMethod, PaymentMethodCreem)

	return nil
}

func RechargeWaffo(tradeNo string, callerIp string) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderWaffo {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil // 幂等：已成功直接返回
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		dAmount := decimal.NewFromInt(topUp.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		quotaToAdd = int(dAmount.Mul(dQuotaPerUnit).IntPart())
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		common.SysError("waffo topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		RecordTopupLog(topUp.UserId, fmt.Sprintf("Waffo充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money), callerIp, topUp.PaymentMethod, PaymentMethodWaffo)
	}

	return nil
}

func RechargeWaffoPancake(tradeNo string) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderWaffoPancake {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		quotaToAdd = int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		common.SysError("waffo pancake topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("Waffo Pancake充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money))
	}

	return nil
}
