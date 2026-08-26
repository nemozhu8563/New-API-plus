package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

const SubscriptionCurrencyCNY = "CNY"

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

var (
	ErrSubscriptionOrderNotFound       = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid  = errors.New("subscription order status invalid")
	ErrStripeSubscriptionMismatch      = errors.New("stripe subscription does not match the immutable order snapshot")
	ErrStripeInvoiceAlreadyBound       = errors.New("stripe invoice already belongs to another order")
	ErrStripeSubscriptionPeriodOverlap = errors.New("stripe subscription invoice overlaps an existing service period")
)

const (
	subscriptionPlanCacheNamespace     = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace = "new-api:subscription_plan_info:v1"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'CNY'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	// Allow falling back to wallet balance after subscription quota is exhausted (empty = true)
	AllowWalletOverflow *bool `json:"allow_wallet_overflow"`

	StripePriceId         string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId        string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`
	WaffoPancakeProductId string `json:"waffo_pancake_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// Downgrade user group on expiry (empty = revert to the group held before purchase)
	DowngradeGroup string `json:"downgrade_group" gorm:"type:varchar(64);default:''"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

func (p *SubscriptionPlan) NormalizeDefaults() {
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if p.Currency == "" {
		p.Currency = SubscriptionCurrencyCNY
	}
	if p.AllowWalletOverflow == nil {
		p.AllowWalletOverflow = common.GetPointer(true)
	}
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id     int     `json:"id"`
	UserId int     `json:"user_id" gorm:"index"`
	PlanId int     `json:"plan_id" gorm:"index"`
	Money  float64 `json:"money"`

	TradeNo                 string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod           string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider         string  `json:"payment_provider" gorm:"type:varchar(50);default:'';uniqueIndex:idx_subscription_order_provider_subscription,priority:1"`
	ProviderOrderId         string  `json:"provider_order_id" gorm:"type:varchar(255);default:'';index"`
	ProviderProductId       string  `json:"provider_product_id" gorm:"type:varchar(255);default:''"`
	ProviderCustomerId      string  `json:"provider_customer_id" gorm:"type:varchar(255);default:'';index"`
	ProviderSubscriptionId  *string `json:"provider_subscription_id" gorm:"type:varchar(255);uniqueIndex:idx_subscription_order_provider_subscription,priority:2"`
	ProviderLivemode        bool    `json:"provider_livemode"`
	ExpectedAmountMinor     int64   `json:"expected_amount_minor" gorm:"type:bigint;not null;default:0"`
	ExpectedCurrency        string  `json:"expected_currency" gorm:"type:varchar(8);default:''"`
	PlanTitle               string  `json:"plan_title" gorm:"type:varchar(128);default:''"`
	PlanDurationUnit        string  `json:"plan_duration_unit" gorm:"type:varchar(16);default:''"`
	PlanDurationValue       int     `json:"plan_duration_value" gorm:"type:int;default:0"`
	PlanCustomSeconds       int64   `json:"plan_custom_seconds" gorm:"type:bigint;default:0"`
	PlanTotalAmount         int64   `json:"plan_total_amount" gorm:"type:bigint;default:0"`
	PlanResetPeriod         string  `json:"plan_reset_period" gorm:"type:varchar(16);default:'never'"`
	PlanResetCustomSeconds  int64   `json:"plan_reset_custom_seconds" gorm:"type:bigint;default:0"`
	PlanUpgradeGroup        string  `json:"plan_upgrade_group" gorm:"type:varchar(64);default:''"`
	PlanDowngradeGroup      string  `json:"plan_downgrade_group" gorm:"type:varchar(64);default:''"`
	PlanAllowWalletOverflow bool    `json:"plan_allow_wallet_overflow"`
	StripeStatus            string  `json:"stripe_status" gorm:"type:varchar(32);default:''"`
	StripeStatusEventTime   int64   `json:"stripe_status_event_time" gorm:"type:bigint;default:0"`
	StripeCancelAtPeriodEnd bool    `json:"stripe_cancel_at_period_end"`
	StripeCancelAt          int64   `json:"stripe_cancel_at" gorm:"type:bigint;not null;default:0"`
	StripeCancelRequestedAt int64   `json:"stripe_cancel_requested_at" gorm:"type:bigint;not null;default:0"`
	StripeCurrentPeriodEnd  int64   `json:"stripe_current_period_end" gorm:"type:bigint;not null;default:0"`
	Status                  string  `json:"status"`
	CreateTime              int64   `json:"create_time"`
	CompleteTime            int64   `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	return DB.Create(o).Error
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
}

type StripeSubscriptionCheckoutBinding struct {
	CheckoutSessionId string
	CustomerId        string
	SubscriptionId    string
	PriceId           string
	AmountMinor       int64
	Currency          string
	Livemode          bool
}

func (o *SubscriptionOrder) BindStripeCheckout(binding StripeSubscriptionCheckoutBinding) error {
	if o == nil || o.Id == 0 || strings.TrimSpace(binding.CheckoutSessionId) == "" ||
		strings.TrimSpace(binding.PriceId) == "" || binding.AmountMinor <= 0 ||
		strings.TrimSpace(binding.Currency) == "" {
		return errors.New("invalid Stripe subscription Checkout binding")
	}
	if (binding.CustomerId == "") != (binding.SubscriptionId == "") {
		return errors.New("incomplete Stripe subscription Checkout binding")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var stored SubscriptionOrder
		if err := lockForUpdate(tx).Where("id = ?", o.Id).First(&stored).Error; err != nil {
			return err
		}
		if stored.PaymentProvider != PaymentProviderStripe || stored.PaymentMethod != PaymentMethodStripe {
			return ErrPaymentMethodMismatch
		}
		if stored.ProviderOrderId != "" && stored.ProviderOrderId != binding.CheckoutSessionId {
			return fmt.Errorf("%w: Checkout Session mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ProviderProductId == "" || stored.ProviderProductId != binding.PriceId {
			return fmt.Errorf("%w: Price mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ExpectedAmountMinor > 0 && stored.ExpectedAmountMinor != binding.AmountMinor {
			return fmt.Errorf("%w: amount mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ExpectedCurrency != "" && !strings.EqualFold(stored.ExpectedCurrency, binding.Currency) {
			return fmt.Errorf("%w: currency mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ProviderCustomerId != "" && stored.ProviderCustomerId != binding.CustomerId {
			return fmt.Errorf("%w: Customer mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ProviderSubscriptionId != nil && *stored.ProviderSubscriptionId != binding.SubscriptionId {
			return fmt.Errorf("%w: Subscription mismatch", ErrStripeSubscriptionMismatch)
		}
		stored.ProviderOrderId = binding.CheckoutSessionId
		if binding.CustomerId != "" {
			stored.ProviderCustomerId = binding.CustomerId
			stored.ProviderSubscriptionId = common.GetPointer(binding.SubscriptionId)
		}
		stored.ExpectedAmountMinor = binding.AmountMinor
		stored.ExpectedCurrency = strings.ToUpper(binding.Currency)
		stored.ProviderLivemode = binding.Livemode
		if err := tx.Save(&stored).Error; err != nil {
			return err
		}
		*o = stored
		return nil
	})
}

func (o *SubscriptionOrder) BindStripeSubscription(customerId string, subscriptionId string, livemode bool) error {
	if o == nil || o.Id == 0 || strings.TrimSpace(customerId) == "" || strings.TrimSpace(subscriptionId) == "" {
		return errors.New("invalid Stripe subscription binding")
	}
	var bindWriteErr error
	err := DB.Transaction(func(tx *gorm.DB) error {
		var stored SubscriptionOrder
		if err := lockForUpdate(tx).Where("id = ?", o.Id).First(&stored).Error; err != nil {
			return err
		}
		if stored.PaymentProvider != PaymentProviderStripe || stored.PaymentMethod != PaymentMethodStripe {
			return ErrPaymentMethodMismatch
		}
		if stored.ProviderOrderId == "" || stored.ProviderProductId == "" ||
			stored.ExpectedAmountMinor <= 0 || stored.ExpectedCurrency == "" {
			return ErrStripeCheckoutUnbound
		}
		if stored.ProviderCustomerId != "" && stored.ProviderCustomerId != customerId {
			return fmt.Errorf("%w: Customer mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ProviderSubscriptionId != nil && *stored.ProviderSubscriptionId != subscriptionId {
			return fmt.Errorf("%w: Subscription mismatch", ErrStripeSubscriptionMismatch)
		}
		if stored.ProviderLivemode != livemode {
			return fmt.Errorf("%w: livemode mismatch", ErrStripeSubscriptionMismatch)
		}
		stored.ProviderCustomerId = customerId
		stored.ProviderSubscriptionId = common.GetPointer(subscriptionId)
		if err := tx.Save(&stored).Error; err != nil {
			bindWriteErr = err
			return err
		}
		*o = stored
		return nil
	})
	if err == nil || bindWriteErr == nil {
		return err
	}

	// A PostgreSQL unique violation aborts the transaction, so ownership must
	// be checked only after rollback. MySQL and SQLite use the same path to
	// keep conflict classification portable across all supported databases.
	var ownerCount int64
	if countErr := DB.Model(&SubscriptionOrder{}).
		Where("payment_provider = ? AND provider_subscription_id = ? AND id <> ?",
			PaymentProviderStripe, subscriptionId, o.Id).
		Count(&ownerCount).Error; countErr == nil && ownerCount > 0 {
		return fmt.Errorf("%w: Subscription already bound to another order", ErrStripeSubscriptionMismatch)
	}
	return err
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

func GetStripeSubscriptionOrderByProviderSubscriptionId(subscriptionId string) *SubscriptionOrder {
	if strings.TrimSpace(subscriptionId) == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("payment_provider = ? AND provider_subscription_id = ?", PaymentProviderStripe, subscriptionId).
		First(&order).Error; err != nil {
		return nil
	}
	return &order
}

func UpdateStripeSubscriptionLifecycle(subscriptionId string, customerId string, stripeStatus string, livemode bool, eventCreated int64, terminal bool, cancelAtPeriodEnd bool, cancelAt int64, currentPeriodEnd int64) error {
	if strings.TrimSpace(subscriptionId) == "" || strings.TrimSpace(customerId) == "" || strings.TrimSpace(stripeStatus) == "" || eventCreated <= 0 {
		return errors.New("invalid Stripe subscription lifecycle update")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := lockForUpdate(tx).
			Where("payment_provider = ? AND provider_subscription_id = ?", PaymentProviderStripe, subscriptionId).
			First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if order.ProviderCustomerId != customerId || order.ProviderLivemode != livemode {
			return fmt.Errorf("%w: Customer or livemode mismatch", ErrStripeSubscriptionMismatch)
		}
		if eventCreated < order.StripeStatusEventTime {
			return nil
		}
		// Stripe event.created has second precision and webhook delivery is not
		// ordered. For equal timestamps, never let an updated event resurrect a
		// subscription after the terminal deleted event has been recorded.
		if eventCreated == order.StripeStatusEventTime &&
			(order.StripeStatus == "canceled" || !terminal) {
			return nil
		}
		order.StripeStatus = stripeStatus
		order.StripeStatusEventTime = eventCreated
		order.StripeCancelAtPeriodEnd = cancelAtPeriodEnd
		order.StripeCancelAt = cancelAt
		order.StripeCurrentPeriodEnd = currentPeriodEnd
		return tx.Save(&order).Error
	})
}

type StripeSubscriptionSummary struct {
	SubscriptionId    string `json:"subscription_id"`
	CustomerId        string `json:"customer_id"`
	PlanId            int    `json:"plan_id"`
	PlanTitle         string `json:"plan_title"`
	Status            string `json:"status"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	CancelAt          int64  `json:"cancel_at"`
	CurrentPeriodEnd  int64  `json:"current_period_end"`
	Livemode          bool   `json:"livemode"`
}

type StripeInvoiceSummary struct {
	InvoiceId       string `json:"invoice_id"`
	SubscriptionId  string `json:"subscription_id"`
	PlanTitle       string `json:"plan_title"`
	AmountPaidMinor int64  `json:"amount_paid_minor"`
	Currency        string `json:"currency"`
	PeriodStart     int64  `json:"period_start"`
	PeriodEnd       int64  `json:"period_end"`
	CreatedAt       int64  `json:"created_at"`
	Livemode        bool   `json:"livemode"`
}

func GetStripeSubscriptionOrderForUser(userId int, subscriptionId string, livemode bool) (*SubscriptionOrder, error) {
	if userId <= 0 || strings.TrimSpace(subscriptionId) == "" {
		return nil, errors.New("invalid Stripe subscription lookup")
	}
	var order SubscriptionOrder
	if err := DB.Where(
		"user_id = ? AND payment_provider = ? AND provider_subscription_id = ? AND provider_livemode = ?",
		userId, PaymentProviderStripe, subscriptionId, livemode,
	).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func MarkStripeSubscriptionCancellationRequested(userId int, subscriptionId string, customerId string, livemode bool, stripeStatus string, cancelAt int64, currentPeriodEnd int64) error {
	if userId <= 0 || strings.TrimSpace(subscriptionId) == "" || strings.TrimSpace(customerId) == "" ||
		strings.TrimSpace(stripeStatus) == "" || currentPeriodEnd <= 0 {
		return errors.New("invalid Stripe cancellation response")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := lockForUpdate(tx).Where(
			"user_id = ? AND payment_provider = ? AND provider_subscription_id = ? AND provider_livemode = ?",
			userId, PaymentProviderStripe, subscriptionId, livemode,
		).First(&order).Error; err != nil {
			return err
		}
		if order.ProviderCustomerId != customerId {
			return fmt.Errorf("%w: Stripe Customer mismatch", ErrStripeSubscriptionMismatch)
		}
		order.StripeStatus = stripeStatus
		order.StripeCancelAtPeriodEnd = true
		order.StripeCancelAt = cancelAt
		order.StripeCancelRequestedAt = common.GetTimestamp()
		order.StripeCurrentPeriodEnd = currentPeriodEnd
		return tx.Save(&order).Error
	})
}

func GetStripeSubscriptionBilling(userId int, livemode bool) ([]StripeSubscriptionSummary, []StripeInvoiceSummary, error) {
	if userId <= 0 {
		return nil, nil, errors.New("invalid userId")
	}
	var orders []SubscriptionOrder
	if err := DB.Where(
		"user_id = ? AND payment_provider = ? AND provider_subscription_id IS NOT NULL AND provider_livemode = ?",
		userId, PaymentProviderStripe, livemode,
	).Order("stripe_current_period_end desc, id desc").Find(&orders).Error; err != nil {
		return nil, nil, err
	}
	subscriptions := make([]StripeSubscriptionSummary, 0, len(orders))
	orderTitles := make(map[int]string, len(orders))
	for _, order := range orders {
		if order.ProviderSubscriptionId == nil || strings.TrimSpace(*order.ProviderSubscriptionId) == "" {
			continue
		}
		orderTitles[order.Id] = order.PlanTitle
		subscriptions = append(subscriptions, StripeSubscriptionSummary{
			SubscriptionId:    *order.ProviderSubscriptionId,
			CustomerId:        order.ProviderCustomerId,
			PlanId:            order.PlanId,
			PlanTitle:         order.PlanTitle,
			Status:            order.StripeStatus,
			CancelAtPeriodEnd: order.StripeCancelAtPeriodEnd,
			CancelAt:          order.StripeCancelAt,
			CurrentPeriodEnd:  order.StripeCurrentPeriodEnd,
			Livemode:          order.ProviderLivemode,
		})
	}
	if len(orderTitles) == 0 {
		return subscriptions, []StripeInvoiceSummary{}, nil
	}
	orderIds := make([]int, 0, len(orderTitles))
	for orderId := range orderTitles {
		orderIds = append(orderIds, orderId)
	}
	var settlements []StripeSubscriptionSettlement
	if err := DB.Where("subscription_order_id IN ? AND livemode = ?", orderIds, livemode).
		Order("period_end desc, id desc").Limit(100).Find(&settlements).Error; err != nil {
		return nil, nil, err
	}
	invoices := make([]StripeInvoiceSummary, 0, len(settlements))
	for _, settlement := range settlements {
		invoices = append(invoices, StripeInvoiceSummary{
			InvoiceId:       settlement.InvoiceId,
			SubscriptionId:  settlement.ProviderSubscriptionId,
			PlanTitle:       orderTitles[settlement.SubscriptionOrderId],
			AmountPaidMinor: settlement.AmountPaidMinor,
			Currency:        settlement.Currency,
			PeriodStart:     settlement.PeriodStart,
			PeriodEnd:       settlement.PeriodEnd,
			CreatedAt:       settlement.CreatedAt,
			Livemode:        settlement.Livemode,
		})
	}
	return subscriptions, invoices, nil
}

func MarkStripeSubscriptionPaymentFailed(tradeNo string, subscriptionId string, customerId string, livemode bool, eventCreated int64) error {
	if strings.TrimSpace(tradeNo) == "" || strings.TrimSpace(subscriptionId) == "" || strings.TrimSpace(customerId) == "" || eventCreated <= 0 {
		return errors.New("invalid Stripe subscription payment failure")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := lockForUpdate(tx).
			Where("payment_provider = ? AND trade_no = ?", PaymentProviderStripe, tradeNo).
			First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if order.ProviderOrderId == "" || order.ProviderProductId == "" ||
			order.ExpectedAmountMinor <= 0 || order.ExpectedCurrency == "" {
			return ErrStripeCheckoutUnbound
		}
		if order.ProviderCustomerId != "" && order.ProviderCustomerId != customerId {
			return fmt.Errorf("%w: Customer mismatch", ErrStripeSubscriptionMismatch)
		}
		if order.ProviderSubscriptionId != nil && *order.ProviderSubscriptionId != subscriptionId {
			return fmt.Errorf("%w: Subscription mismatch", ErrStripeSubscriptionMismatch)
		}
		if order.ProviderLivemode != livemode {
			return fmt.Errorf("%w: livemode mismatch", ErrStripeSubscriptionMismatch)
		}
		order.ProviderCustomerId = customerId
		order.ProviderSubscriptionId = common.GetPointer(subscriptionId)
		if eventCreated > order.StripeStatusEventTime {
			order.StripeStatus = "payment_failed"
			order.StripeStatusEventTime = eventCreated
		}
		return tx.Save(&order).Error
	})
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	// Downgrade target group on expiry (snapshot from plan; empty = revert to PrevUserGroup)
	DowngradeGroup string `json:"downgrade_group" gorm:"type:varchar(64);default:''"`

	// Whether wallet fallback is allowed after this subscription's quota is exhausted (snapshot from plan)
	AllowWalletOverflow     bool   `json:"allow_wallet_overflow"`
	PlanTitle               string `json:"plan_title" gorm:"type:varchar(128);default:''"`
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`
	Provider                string `json:"provider" gorm:"type:varchar(50);default:''"`
	ProviderSubscriptionId  string `json:"provider_subscription_id" gorm:"type:varchar(255);default:'';index"`
	ProviderInvoiceId       string `json:"provider_invoice_id" gorm:"type:varchar(255);default:'';index"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

type StripeSubscriptionSettlement struct {
	Id                     int    `json:"id"`
	InvoiceId              string `json:"invoice_id" gorm:"type:varchar(255);uniqueIndex"`
	SubscriptionOrderId    int    `json:"subscription_order_id" gorm:"index;not null"`
	UserSubscriptionId     int    `json:"user_subscription_id" gorm:"uniqueIndex;not null"`
	ProviderCustomerId     string `json:"provider_customer_id" gorm:"type:varchar(255);not null;index"`
	ProviderSubscriptionId string `json:"provider_subscription_id" gorm:"type:varchar(255);not null;index;uniqueIndex:idx_stripe_subscription_period,priority:1"`
	ProviderProductId      string `json:"provider_product_id" gorm:"type:varchar(255);not null"`
	Quantity               int64  `json:"quantity" gorm:"type:bigint;not null"`
	UnitAmountMinor        int64  `json:"unit_amount_minor" gorm:"type:bigint;not null"`
	InvoiceTotalMinor      int64  `json:"invoice_total_minor" gorm:"type:bigint;not null;default:0"`
	AmountPaidMinor        int64  `json:"amount_paid_minor" gorm:"type:bigint;not null"`
	Currency               string `json:"currency" gorm:"type:varchar(8);not null"`
	Livemode               bool   `json:"livemode" gorm:"uniqueIndex:idx_stripe_subscription_period,priority:2"`
	PeriodStart            int64  `json:"period_start" gorm:"type:bigint;not null;uniqueIndex:idx_stripe_subscription_period,priority:3"`
	PeriodEnd              int64  `json:"period_end" gorm:"type:bigint;not null;uniqueIndex:idx_stripe_subscription_period,priority:4"`
	CreatedAt              int64  `json:"created_at" gorm:"type:bigint"`
}

// StripeSubscriptionLock provides a durable row-level serialization point for
// all invoices belonging to one Stripe subscription. SubscriptionOrder cannot
// serve that purpose before the first invoice binds its provider ID because
// nullable unique columns allow multiple unbound local orders.
type StripeSubscriptionLock struct {
	Id                     int    `json:"id"`
	ProviderSubscriptionId string `json:"provider_subscription_id" gorm:"type:varchar(255);not null;uniqueIndex:idx_stripe_subscription_lock,priority:1"`
	Livemode               bool   `json:"livemode" gorm:"not null;uniqueIndex:idx_stripe_subscription_lock,priority:2"`
	CreatedAt              int64  `json:"created_at" gorm:"type:bigint"`
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

type SubscriptionSummary struct {
	Subscription *UserSubscription `json:"subscription"`
}

type SubscriptionResetResult struct {
	PlanId           int    `json:"plan_id"`
	MatchedCount     int    `json:"matched_count"`
	ResetCount       int    `json:"reset_count"`
	UserCount        int    `json:"user_count"`
	AdvanceResetTime bool   `json:"advance_reset_time"`
	PlanTitle        string `json:"-"`
	AffectedUserIds  []int  `json:"-"`
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// Align to next Monday 00:00
		weekday := int(base.Weekday()) // Sunday=0
		// Convert to Monday=1..Sunday=7
		if weekday == 0 {
			weekday = 7
		}
		daysUntil := 8 - weekday
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, daysUntil)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func calcNextResetTimeForSnapshot(base time.Time, period string, customSeconds int64, endUnix int64) int64 {
	plan := &SubscriptionPlan{
		QuotaResetPeriod:        period,
		QuotaResetCustomSeconds: customSeconds,
	}
	return calcNextResetTime(base, plan, endUnix)
}

func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	if key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			cached.NormalizeDefaults()
			return &cached, nil
		}
	}
	var plan SubscriptionPlan
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	plan.NormalizeDefaults()
	_ = getSubscriptionPlanCache().SetWithTTL(key, plan, subscriptionPlanCacheTTL())
	return &plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var group string
	if err := lockForUpdate(tx).Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	downgradeGroup := strings.TrimSpace(sub.DowngradeGroup)
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	// Nothing to do if neither an explicit downgrade target nor an upgrade snapshot exists.
	if downgradeGroup == "" && upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	// If another active subscription exists, it remains the user's current
	// entitlement and owns any eventual group transition.
	var activeSub UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ?",
		sub.UserId, "active", now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	// Determine the downgrade target: an explicit downgrade group takes precedence,
	// otherwise revert to the group held before purchase (legacy behavior).
	target := downgradeGroup
	if target == "" {
		// Legacy behavior: only revert when the subscription actually elevated the user.
		if currentGroup != upgradeGroup {
			return "", nil
		}
		target = strings.TrimSpace(sub.PrevUserGroup)
	}
	if target == "" || target == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", target).Error; err != nil {
		return "", err
	}
	return target, nil
}

func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := getDBTimestamp(tx)
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" {
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}
	allowWalletOverflow := true
	if plan.AllowWalletOverflow != nil {
		allowWalletOverflow = *plan.AllowWalletOverflow
	}
	sub := &UserSubscription{
		UserId:                  userId,
		PlanId:                  plan.Id,
		AmountTotal:             plan.TotalAmount,
		AmountUsed:              0,
		StartTime:               now.Unix(),
		EndTime:                 endUnix,
		Status:                  "active",
		Source:                  source,
		LastResetTime:           lastReset,
		NextResetTime:           nextReset,
		UpgradeGroup:            upgradeGroup,
		PrevUserGroup:           prevGroup,
		DowngradeGroup:          strings.TrimSpace(plan.DowngradeGroup),
		AllowWalletOverflow:     allowWalletOverflow,
		PlanTitle:               plan.Title,
		QuotaResetPeriod:        NormalizeResetPeriod(plan.QuotaResetPeriod),
		QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
		CreatedAt:               common.GetTimestamp(),
		UpdatedAt:               common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func refreshSubscriptionUserGroupCache(userId int, operation string) {
	if err := RefreshUserGroupCache(userId); err != nil {
		common.SysError(fmt.Sprintf("failed to refresh user group cache after %s for user %d: %v", operation, userId, err))
	}
}

// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
// expectedPaymentProvider guards against cross-gateway callback attacks (empty skips the check).
// actualPaymentMethod updates the order's PaymentMethod to reflect the real payment type used (empty skips update).
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		// 锁定用户行：并发完成同一用户的不同订单（包括多实例部署下）时，
		// 使 CreateUserSubscriptionFromPlanTx 的 MaxPurchasePerUser 检查按用户串行。
		var userRow User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", order.UserId).First(&userRow).Error; err != nil {
			return err
		}
		subscription, err := CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order")
		if err != nil {
			return err
		}
		if subscription.PrevUserGroup != "" {
			upgradeGroup = strings.TrimSpace(subscription.UpgradeGroup)
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		refreshSubscriptionUserGroupCache(logUserId, "subscription payment completion")
	}
	if logUserId > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	return nil
}

type StripeInvoiceSettlementInput struct {
	InvoiceId         string
	TradeNo           string
	CustomerId        string
	SubscriptionId    string
	ProductId         string
	Quantity          int64
	UnitAmountMinor   int64
	InvoiceTotalMinor int64
	AmountPaidMinor   int64
	Currency          string
	Livemode          bool
	PeriodStart       int64
	PeriodEnd         int64
	EventCreated      int64
	ProviderPayload   string
	Payments          []StripePaymentSnapshot
}

func stripeInvoiceSettlementMatchesInput(existing *StripeSubscriptionSettlement, input StripeInvoiceSettlementInput) bool {
	return existing != nil &&
		existing.ProviderCustomerId == input.CustomerId &&
		existing.ProviderSubscriptionId == input.SubscriptionId &&
		existing.ProviderProductId == input.ProductId &&
		existing.Quantity == input.Quantity &&
		existing.UnitAmountMinor == input.UnitAmountMinor &&
		existing.InvoiceTotalMinor == input.InvoiceTotalMinor &&
		existing.AmountPaidMinor == input.AmountPaidMinor &&
		strings.EqualFold(existing.Currency, input.Currency) &&
		existing.Livemode == input.Livemode &&
		existing.PeriodStart == input.PeriodStart &&
		existing.PeriodEnd == input.PeriodEnd
}

func CompleteStripeSubscriptionInvoice(input StripeInvoiceSettlementInput) error {
	if strings.TrimSpace(input.InvoiceId) == "" || strings.TrimSpace(input.CustomerId) == "" ||
		strings.TrimSpace(input.SubscriptionId) == "" || strings.TrimSpace(input.ProductId) == "" ||
		strings.TrimSpace(input.Currency) == "" || input.Quantity != 1 || input.UnitAmountMinor <= 0 ||
		input.InvoiceTotalMinor <= 0 ||
		input.AmountPaidMinor < 0 ||
		input.PeriodStart <= 0 || input.PeriodEnd <= input.PeriodStart || input.EventCreated <= 0 {
		return fmt.Errorf("%w: invalid invoice settlement", ErrStripeSubscriptionMismatch)
	}

	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		lockRow := StripeSubscriptionLock{
			ProviderSubscriptionId: input.SubscriptionId,
			Livemode:               input.Livemode,
			CreatedAt:              common.GetTimestamp(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lockRow).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where(
			"provider_subscription_id = ? AND livemode = ?",
			input.SubscriptionId,
			input.Livemode,
		).First(&lockRow).Error; err != nil {
			return err
		}

		var existing StripeSubscriptionSettlement
		query := tx.Where("invoice_id = ?", input.InvoiceId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if !stripeInvoiceSettlementMatchesInput(&existing, input) {
				return ErrStripeInvoiceAlreadyBound
			}
			if len(input.Payments) > 0 {
				var existingSub UserSubscription
				if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&existingSub).Error; err != nil {
					return err
				}
				if err := registerStripeSubscriptionPaymentsTx(tx, &existing, &existingSub, input.Payments); err != nil {
					return err
				}
			}
			return nil
		}

		var order SubscriptionOrder
		tradeNo := strings.TrimSpace(input.TradeNo)
		boundOrderQuery := lockForUpdate(tx).
			Where("payment_provider = ? AND provider_subscription_id = ?", PaymentProviderStripe, input.SubscriptionId).
			Limit(1).
			Find(&order)
		if boundOrderQuery.Error != nil {
			return boundOrderQuery.Error
		}
		if boundOrderQuery.RowsAffected == 0 {
			if tradeNo == "" {
				return ErrSubscriptionOrderNotFound
			}
			if err := lockForUpdate(tx).
				Where("payment_provider = ? AND trade_no = ?", PaymentProviderStripe, tradeNo).
				First(&order).Error; err != nil {
				return ErrSubscriptionOrderNotFound
			}
		}
		if tradeNo != "" && order.TradeNo != tradeNo {
			return fmt.Errorf("%w: invoice trade number does not match the bound subscription order", ErrStripeSubscriptionMismatch)
		}

		// The subscription order row serializes invoice settlement. Recheck the
		// invoice after acquiring it so concurrent deliveries of the same invoice
		// remain idempotent instead of being misclassified as overlapping periods.
		existing = StripeSubscriptionSettlement{}
		query = tx.Where("invoice_id = ?", input.InvoiceId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if !stripeInvoiceSettlementMatchesInput(&existing, input) {
				return ErrStripeInvoiceAlreadyBound
			}
			if len(input.Payments) > 0 {
				var existingSub UserSubscription
				if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&existingSub).Error; err != nil {
					return err
				}
				if err := registerStripeSubscriptionPaymentsTx(tx, &existing, &existingSub, input.Payments); err != nil {
					return err
				}
			}
			return nil
		}
		if order.ProviderOrderId == "" || order.ProviderProductId == "" ||
			order.ExpectedAmountMinor <= 0 || order.ExpectedCurrency == "" {
			return ErrStripeCheckoutUnbound
		}
		if (order.ProviderCustomerId != "" && order.ProviderCustomerId != input.CustomerId) ||
			(order.ProviderSubscriptionId != nil && *order.ProviderSubscriptionId != input.SubscriptionId) ||
			order.ProviderProductId != input.ProductId || order.ProviderLivemode != input.Livemode ||
			order.ExpectedAmountMinor != input.UnitAmountMinor ||
			order.ExpectedAmountMinor != input.InvoiceTotalMinor ||
			!strings.EqualFold(order.ExpectedCurrency, input.Currency) {
			return fmt.Errorf("%w: invoice does not match order", ErrStripeSubscriptionMismatch)
		}
		if order.Status != common.TopUpStatusPending && order.Status != common.TopUpStatusSuccess {
			return ErrSubscriptionOrderStatusInvalid
		}
		// The locked order row is the serialization point for all invoices of this
		// Stripe subscription. A second local order must never bind the same
		// provider subscription, or concurrent period checks could lock different
		// rows and both commit.
		var boundOrderCount int64
		if err := tx.Model(&SubscriptionOrder{}).
			Where("payment_provider = ? AND provider_subscription_id = ? AND id <> ?",
				PaymentProviderStripe, input.SubscriptionId, order.Id).
			Count(&boundOrderCount).Error; err != nil {
			return err
		}
		if boundOrderCount > 0 {
			return fmt.Errorf("%w: Subscription already bound to another order", ErrStripeSubscriptionMismatch)
		}

		var overlappingSettlement StripeSubscriptionSettlement
		overlapQuery := lockForUpdate(tx).
			Where("provider_subscription_id = ? AND livemode = ? AND period_start < ? AND period_end > ?",
				input.SubscriptionId, input.Livemode, input.PeriodEnd, input.PeriodStart).
			Limit(1).
			Find(&overlappingSettlement)
		if overlapQuery.Error != nil {
			return overlapQuery.Error
		}
		if overlapQuery.RowsAffected > 0 {
			return ErrStripeSubscriptionPeriodOverlap
		}

		plan := &SubscriptionPlan{
			Id:                      order.PlanId,
			Title:                   order.PlanTitle,
			DurationUnit:            order.PlanDurationUnit,
			DurationValue:           order.PlanDurationValue,
			CustomSeconds:           order.PlanCustomSeconds,
			TotalAmount:             order.PlanTotalAmount,
			QuotaResetPeriod:        order.PlanResetPeriod,
			QuotaResetCustomSeconds: order.PlanResetCustomSeconds,
			UpgradeGroup:            order.PlanUpgradeGroup,
			DowngradeGroup:          order.PlanDowngradeGroup,
			AllowWalletOverflow:     common.GetPointer(order.PlanAllowWalletOverflow),
		}
		// Stripe's invoice line is authoritative for the paid service period.
		// Recomputing calendar periods locally breaks valid month-end billing anchors.
		startTime := input.PeriodStart
		endTime := input.PeriodEnd
		if endTime <= startTime {
			return fmt.Errorf("%w: invalid invoice service period", ErrStripeSubscriptionMismatch)
		}
		lastReset := int64(0)
		nextReset := calcNextResetTimeForSnapshot(time.Unix(startTime, 0), plan.QuotaResetPeriod, plan.QuotaResetCustomSeconds, endTime)
		if nextReset > 0 {
			lastReset = startTime
		}
		prevGroup := ""
		if strings.TrimSpace(plan.UpgradeGroup) != "" {
			currentGroup, err := getUserGroupByIdTx(tx, order.UserId)
			if err != nil {
				return err
			}
			if currentGroup != strings.TrimSpace(plan.UpgradeGroup) {
				prevGroup = currentGroup
				if err := tx.Model(&User{}).Where("id = ?", order.UserId).
					Update("group", strings.TrimSpace(plan.UpgradeGroup)).Error; err != nil {
					return err
				}
				upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
			}
		}
		sub := &UserSubscription{
			UserId: order.UserId, PlanId: order.PlanId, AmountTotal: plan.TotalAmount,
			StartTime: startTime, EndTime: endTime, Status: "active", Source: "stripe_invoice",
			LastResetTime: lastReset, NextResetTime: nextReset,
			UpgradeGroup: strings.TrimSpace(plan.UpgradeGroup), PrevUserGroup: prevGroup,
			DowngradeGroup: strings.TrimSpace(plan.DowngradeGroup), AllowWalletOverflow: order.PlanAllowWalletOverflow,
			PlanTitle: order.PlanTitle, QuotaResetPeriod: NormalizeResetPeriod(order.PlanResetPeriod),
			QuotaResetCustomSeconds: order.PlanResetCustomSeconds, Provider: PaymentProviderStripe,
			ProviderSubscriptionId: input.SubscriptionId, ProviderInvoiceId: input.InvoiceId,
		}
		if err := tx.Create(sub).Error; err != nil {
			return err
		}
		settlement := &StripeSubscriptionSettlement{
			InvoiceId: input.InvoiceId, SubscriptionOrderId: order.Id, UserSubscriptionId: sub.Id,
			ProviderCustomerId: input.CustomerId, ProviderSubscriptionId: input.SubscriptionId,
			ProviderProductId: input.ProductId, Quantity: input.Quantity, UnitAmountMinor: input.UnitAmountMinor,
			InvoiceTotalMinor: input.InvoiceTotalMinor, AmountPaidMinor: input.AmountPaidMinor,
			Currency: strings.ToUpper(input.Currency), Livemode: input.Livemode,
			PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd, CreatedAt: common.GetTimestamp(),
		}
		if err := tx.Create(settlement).Error; err != nil {
			return err
		}
		if len(input.Payments) > 0 {
			if err := registerStripeSubscriptionPaymentsTx(tx, settlement, sub, input.Payments); err != nil {
				return err
			}
		}
		if order.Status == common.TopUpStatusPending {
			if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
				return err
			}
			order.Status = common.TopUpStatusSuccess
			order.CompleteTime = common.GetTimestamp()
		}
		order.ProviderCustomerId = input.CustomerId
		order.ProviderSubscriptionId = common.GetPointer(input.SubscriptionId)
		if input.EventCreated > order.StripeStatusEventTime {
			order.StripeStatus = "active"
			order.StripeStatusEventTime = input.EventCreated
		}
		if input.ProviderPayload != "" {
			order.ProviderPayload = input.ProviderPayload
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = order.PlanTitle
		logMoney = order.Money
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		refreshSubscriptionUserGroupCache(logUserId, "Stripe subscription invoice")
	}
	if logUserId > 0 {
		RecordLog(logUserId, LogTypeTopup, fmt.Sprintf("Stripe 订阅账单支付成功，套餐: %s，支付金额: %.2f", logPlanTitle, logMoney))
	}
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	var topup TopUp
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId:          order.UserId,
				Amount:          0,
				Money:           order.Money,
				TradeNo:         order.TradeNo,
				PaymentMethod:   order.PaymentMethod,
				PaymentProvider: order.PaymentProvider,
				CreateTime:      order.CreateTime,
				CompleteTime:    now,
				Status:          common.TopUpStatusSuccess,
			}
			return tx.Create(&topup).Error
		}
		return err
	}
	topup.Money = order.Money
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return ErrPaymentMethodMismatch
	}
	if topup.PaymentProvider == "" {
		topup.PaymentProvider = order.PaymentProvider
	} else if topup.PaymentProvider != order.PaymentProvider {
		return ErrPaymentMethodMismatch
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = common.TopUpStatusSuccess
	return tx.Save(&topup).Error
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

// Admin bind (no payment). Creates a UserSubscription from a plan.
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	groupChanged := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 与 CompleteSubscriptionOrder 一致：先锁用户行，再做购买次数检查。
		var userRow User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&userRow).Error; err != nil {
			return err
		}
		subscription, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		if err == nil {
			groupChanged = subscription.PrevUserGroup != ""
		}
		return err
	})
	if err != nil {
		return "", err
	}
	if groupChanged {
		refreshSubscriptionUserGroupCache(userId, "admin subscription creation")
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
	}
	return "", nil
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// UserActiveSubscriptionsAllowWalletOverflow returns whether wallet balance may be used
// after the user's subscription quota is exhausted. A single active subscription that
// disallows wallet overflow (allow_wallet_overflow = false) blocks the fallback.
func UserActiveSubscriptionsAllowWalletOverflow(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var strictCount int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ? AND allow_wallet_overflow = ?",
			userId, "active", now, false).
		Count(&strictCount).Error; err != nil {
		return false, err
	}
	return strictCount == 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
		})
	}
	return result
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := lockForUpdate(tx).
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":     "cancelled",
			"end_time":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		refreshSubscriptionUserGroupCache(userId, "admin subscription update")
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := lockForUpdate(tx).
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		refreshSubscriptionUserGroupCache(userId, "admin subscription deletion")
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

func resetUserSubscriptionTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64, advanceResetTime bool) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	sub.AmountUsed = 0
	if advanceResetTime {
		period := sub.QuotaResetPeriod
		customSeconds := sub.QuotaResetCustomSeconds
		if strings.TrimSpace(sub.PlanTitle) == "" {
			period = plan.QuotaResetPeriod
			customSeconds = plan.QuotaResetCustomSeconds
		}
		nextReset := calcNextResetTimeForSnapshot(time.Unix(now, 0), period, customSeconds, sub.EndTime)
		sub.NextResetTime = nextReset
		if nextReset > 0 {
			sub.LastResetTime = now
		} else {
			sub.LastResetTime = 0
		}
	}
	return tx.Save(sub).Error
}

func buildSubscriptionResetResult(plan *SubscriptionPlan, subs []UserSubscription, advanceResetTime bool) *SubscriptionResetResult {
	userIds := make([]int, 0, len(subs))
	seenUsers := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if _, ok := seenUsers[sub.UserId]; ok {
			continue
		}
		seenUsers[sub.UserId] = struct{}{}
		userIds = append(userIds, sub.UserId)
	}
	return &SubscriptionResetResult{
		PlanId:           plan.Id,
		MatchedCount:     len(subs),
		ResetCount:       len(subs),
		UserCount:        len(userIds),
		AdvanceResetTime: advanceResetTime,
		PlanTitle:        plan.Title,
		AffectedUserIds:  userIds,
	}
}

func adminResetUserSubscriptionsByPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, now int64, advanceResetTime bool) (*SubscriptionResetResult, error) {
	if tx == nil || plan == nil {
		return nil, errors.New("invalid reset args")
	}
	var subs []UserSubscription
	if err := lockForUpdate(tx).
		Where("user_id = ? AND plan_id = ? AND status = ? AND end_time > ?", userId, plan.Id, "active", now).
		Order("end_time asc, id asc").
		Find(&subs).Error; err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return nil, errors.New("该用户没有有效的此套餐订阅")
	}
	for i := range subs {
		if err := resetUserSubscriptionTx(tx, &subs[i], plan, now, advanceResetTime); err != nil {
			return nil, err
		}
	}
	return buildSubscriptionResetResult(plan, subs, advanceResetTime), nil
}

func adminResetPlanSubscriptionsTx(tx *gorm.DB, plan *SubscriptionPlan, now int64, advanceResetTime bool) (*SubscriptionResetResult, error) {
	if tx == nil || plan == nil {
		return nil, errors.New("invalid reset args")
	}
	var subs []UserSubscription
	if err := lockForUpdate(tx).
		Where("plan_id = ? AND status = ? AND end_time > ?", plan.Id, "active", now).
		Order("user_id asc, end_time asc, id asc").
		Find(&subs).Error; err != nil {
		return nil, err
	}
	for i := range subs {
		if err := resetUserSubscriptionTx(tx, &subs[i], plan, now, advanceResetTime); err != nil {
			return nil, err
		}
	}
	return buildSubscriptionResetResult(plan, subs, advanceResetTime), nil
}

func AdminResetUserSubscriptionsByPlan(userId int, planId int, advanceResetTime bool) (*SubscriptionResetResult, error) {
	if userId <= 0 || planId <= 0 {
		return nil, errors.New("invalid userId or planId")
	}
	var result *SubscriptionResetResult
	now := GetDBTimestamp()
	err := DB.Transaction(func(tx *gorm.DB) error {
		plan, err := getSubscriptionPlanByIdTx(tx, planId)
		if err != nil {
			return err
		}
		result, err = adminResetUserSubscriptionsByPlanTx(tx, userId, plan, now, advanceResetTime)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func AdminResetPlanSubscriptions(planId int, advanceResetTime bool) (*SubscriptionResetResult, error) {
	if planId <= 0 {
		return nil, errors.New("invalid planId")
	}
	var result *SubscriptionResetResult
	now := GetDBTimestamp()
	err := DB.Transaction(func(tx *gorm.DB) error {
		plan, err := getSubscriptionPlanByIdTx(tx, planId)
		if err != nil {
			return err
		}
		result, err = adminResetPlanSubscriptionsTx(tx, plan, now, advanceResetTime)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":     "expired",
					"updated_at": common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's any active subscription, it remains the user's current
			// entitlement and owns the eventual group transition.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ?",
				userId, "active", now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// Find the most recently expired subscription that defines a group transition
			// (an explicit downgrade target or an upgrade snapshot to revert).
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND (downgrade_group <> '' OR upgrade_group <> '')",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			// An explicit downgrade group takes precedence; otherwise revert to the
			// group held before purchase (legacy behavior, only when the subscription
			// actually elevated the user).
			target := strings.TrimSpace(lastExpired.DowngradeGroup)
			if target == "" {
				upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
				prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
				if upgradeGroup == "" || prevGroup == "" {
					return nil
				}
				if currentGroup != upgradeGroup {
					return nil
				}
				target = prevGroup
			}
			if target == "" || target == currentGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", target).Error; err != nil {
				return err
			}
			cacheGroup = target
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			refreshSubscriptionUserGroupCache(userId, "subscription expiration")
		}
	}
	return expiredCount, nil
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	period := sub.QuotaResetPeriod
	customSeconds := sub.QuotaResetCustomSeconds
	if strings.TrimSpace(sub.PlanTitle) == "" {
		period = plan.QuotaResetPeriod
		customSeconds = plan.QuotaResetCustomSeconds
	}
	if NormalizeResetPeriod(period) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTimeForSnapshot(base, period, customSeconds, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTimeForSnapshot(base, period, customSeconds, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

// PreConsumeUserSubscription pre-consumes from any active subscription total quota.
func PreConsumeUserSubscription(requestId string, userId int, modelName string, quotaType int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()

	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = existing.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}

		var subs []UserSubscription
		if err := lockForUpdate(tx).
			Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}
		for _, candidate := range subs {
			sub := candidate
			plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
			if err != nil {
				return err
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
				return err
			}
			usedBefore := sub.AmountUsed
			if sub.AmountTotal > 0 {
				remain := sub.AmountTotal - usedBefore
				if remain < amount {
					continue
				}
			}
			record := &SubscriptionPreConsumeRecord{
				RequestId:          requestId,
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				PreConsumed:        amount,
				Status:             "consumed",
			}
			if err := tx.Create(record).Error; err != nil {
				var dup SubscriptionPreConsumeRecord
				if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
					if dup.Status == "refunded" {
						return errors.New("subscription pre-consume already refunded")
					}
					returnValue.UserSubscriptionId = sub.Id
					returnValue.PreConsumed = dup.PreConsumed
					returnValue.AmountTotal = sub.AmountTotal
					returnValue.AmountUsedBefore = sub.AmountUsed
					returnValue.AmountUsedAfter = sub.AmountUsed
					return nil
				}
				return err
			}
			sub.AmountUsed += amount
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = amount
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed <= 0 {
			record.Status = "refunded"
			return tx.Save(&record).Error
		}
		if err := PostConsumeUserSubscriptionDelta(record.UserSubscriptionId, -record.PreConsumed); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := lockForUpdate(tx).
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// Update subscription used amount by delta (positive consume more, negative refund).
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := lockForUpdate(tx).
			Where("id = ?", userSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		newUsed := sub.AmountUsed + delta
		if newUsed < 0 {
			newUsed = 0
		}
		if sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
			return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
		}
		sub.AmountUsed = newUsed
		return tx.Save(&sub).Error
	})
}
