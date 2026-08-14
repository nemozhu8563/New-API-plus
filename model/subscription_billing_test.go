package model

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCalcSubscriptionBalanceQuotaCeilsFractionalQuota(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 10
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	quota, err := calcSubscriptionBalanceQuota(1.01)

	require.NoError(t, err)
	assert.Equal(t, 11, quota)
}

func TestCalcSubscriptionBalanceQuotaRejectsOverflow(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 10
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	quota, err := calcSubscriptionBalanceQuota(math.MaxFloat64)

	assert.Zero(t, quota)
	var clamp *common.QuotaClamp
	require.ErrorAs(t, err, &clamp)
	assert.Equal(t, common.QuotaClampOverflow, clamp.Kind)
	assert.Equal(t, "QuotaFromDecimal", clamp.Op)
	assert.Equal(t, common.MaxQuota, clamp.Clamped)
}

func TestCompleteStripeSubscriptionInvoiceConcurrentOverlappingPeriodsSettlesOnce(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Log{},
		&TopUp{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&StripeSubscriptionSettlement{},
		&StripeSubscriptionLock{},
	))
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		_ = sqlDB.Close()
	})

	user := &User{
		Username: "stripe-overlap-concurrency-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)
	subscriptionID := "sub_overlap_concurrency"
	order := &SubscriptionOrder{
		UserId: user.Id, PlanId: 1, Money: 12, TradeNo: "trade_overlap_concurrency",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		ProviderOrderId: "cs_overlap_concurrency", ProviderProductId: "price_overlap_concurrency",
		ProviderCustomerId: "cus_overlap_concurrency", ProviderSubscriptionId: &subscriptionID,
		ExpectedAmountMinor: 1200, ExpectedCurrency: "USD",
		PlanTitle: "Overlap concurrency plan", PlanDurationUnit: SubscriptionDurationMonth,
		PlanDurationValue: 1, PlanTotalAmount: 1200, PlanResetPeriod: SubscriptionResetNever,
		PlanAllowWalletOverflow: true, Status: common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(order).Error)

	inputs := []StripeInvoiceSettlementInput{
		{
			InvoiceId: "in_overlap_concurrency_first", TradeNo: order.TradeNo,
			CustomerId: order.ProviderCustomerId, SubscriptionId: subscriptionID,
			ProductId: order.ProviderProductId, Quantity: 1, UnitAmountMinor: order.ExpectedAmountMinor,
			InvoiceTotalMinor: order.ExpectedAmountMinor, AmountPaidMinor: order.ExpectedAmountMinor, Currency: order.ExpectedCurrency,
			PeriodStart: 100, PeriodEnd: 200, EventCreated: 100,
		},
		{
			InvoiceId: "in_overlap_concurrency_second", TradeNo: order.TradeNo,
			CustomerId: order.ProviderCustomerId, SubscriptionId: subscriptionID,
			ProductId: order.ProviderProductId, Quantity: 1, UnitAmountMinor: order.ExpectedAmountMinor,
			InvoiceTotalMinor: order.ExpectedAmountMinor, AmountPaidMinor: order.ExpectedAmountMinor, Currency: order.ExpectedCurrency,
			PeriodStart: 150, PeriodEnd: 250, EventCreated: 200,
		},
	}

	start := make(chan struct{})
	errs := make([]error, len(inputs))
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(inputs))
	for index := range inputs {
		go func(index int) {
			defer waitGroup.Done()
			<-start
			for attempt := 0; attempt < 20; attempt++ {
				errs[index] = CompleteStripeSubscriptionInvoice(inputs[index])
				if errs[index] == nil || !strings.Contains(strings.ToLower(errs[index].Error()), "locked") {
					return
				}
				time.Sleep(time.Millisecond)
			}
		}(index)
	}
	close(start)
	waitGroup.Wait()

	successCount := 0
	overlapCount := 0
	for _, settlementErr := range errs {
		switch {
		case settlementErr == nil:
			successCount++
		case errors.Is(settlementErr, ErrStripeSubscriptionPeriodOverlap):
			overlapCount++
		default:
			require.NoError(t, settlementErr)
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, overlapCount)

	var settlementCount int64
	require.NoError(t, db.Model(&StripeSubscriptionSettlement{}).
		Where("provider_subscription_id = ?", subscriptionID).
		Count(&settlementCount).Error)
	assert.Equal(t, int64(1), settlementCount)
	var subscriptionCount int64
	require.NoError(t, db.Model(&UserSubscription{}).
		Where("provider_subscription_id = ?", subscriptionID).
		Count(&subscriptionCount).Error)
	assert.Equal(t, int64(1), subscriptionCount)
}

func TestCompleteStripeSubscriptionInvoiceConcurrentUnboundOrdersCannotShareSubscription(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Log{},
		&TopUp{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&StripeSubscriptionSettlement{},
		&StripeSubscriptionLock{},
	))
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		_ = sqlDB.Close()
	})

	users := []*User{
		{Username: "stripe-unbound-first", Status: common.UserStatusEnabled, Group: "default", AffCode: "stripe-unbound-first"},
		{Username: "stripe-unbound-second", Status: common.UserStatusEnabled, Group: "default", AffCode: "stripe-unbound-second"},
	}
	orders := make([]*SubscriptionOrder, len(users))
	for index, user := range users {
		require.NoError(t, db.Create(user).Error)
		orders[index] = &SubscriptionOrder{
			UserId: user.Id, PlanId: index + 1, Money: 12,
			TradeNo:       fmt.Sprintf("trade_unbound_concurrency_%d", index),
			PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
			ProviderOrderId:     fmt.Sprintf("cs_unbound_concurrency_%d", index),
			ProviderProductId:   "price_unbound_concurrency",
			ExpectedAmountMinor: 1200, ExpectedCurrency: "USD",
			PlanTitle: "Unbound concurrency plan", PlanDurationUnit: SubscriptionDurationMonth,
			PlanDurationValue: 1, PlanTotalAmount: 1200, PlanResetPeriod: SubscriptionResetNever,
			PlanAllowWalletOverflow: true, Status: common.TopUpStatusPending,
		}
		require.NoError(t, db.Create(orders[index]).Error)
	}

	subscriptionID := "sub_unbound_concurrency"
	inputs := []StripeInvoiceSettlementInput{
		{
			InvoiceId: "in_unbound_concurrency_first", TradeNo: orders[0].TradeNo,
			CustomerId: "cus_unbound_concurrency", SubscriptionId: subscriptionID,
			ProductId: orders[0].ProviderProductId, Quantity: 1, UnitAmountMinor: 1200,
			InvoiceTotalMinor: 1200, AmountPaidMinor: 1200, Currency: "USD", PeriodStart: 100, PeriodEnd: 200, EventCreated: 100,
		},
		{
			InvoiceId: "in_unbound_concurrency_second", TradeNo: orders[1].TradeNo,
			CustomerId: "cus_unbound_concurrency", SubscriptionId: subscriptionID,
			ProductId: orders[1].ProviderProductId, Quantity: 1, UnitAmountMinor: 1200,
			InvoiceTotalMinor: 1200, AmountPaidMinor: 1200, Currency: "USD", PeriodStart: 150, PeriodEnd: 250, EventCreated: 200,
		},
	}

	start := make(chan struct{})
	errs := make([]error, len(inputs))
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(inputs))
	for index := range inputs {
		go func(index int) {
			defer waitGroup.Done()
			<-start
			for attempt := 0; attempt < 20; attempt++ {
				errs[index] = CompleteStripeSubscriptionInvoice(inputs[index])
				if errs[index] == nil || !strings.Contains(strings.ToLower(errs[index].Error()), "locked") {
					return
				}
				time.Sleep(time.Millisecond)
			}
		}(index)
	}
	close(start)
	waitGroup.Wait()

	successCount := 0
	rejectedCount := 0
	for _, settlementErr := range errs {
		switch {
		case settlementErr == nil:
			successCount++
		case errors.Is(settlementErr, ErrStripeSubscriptionMismatch),
			errors.Is(settlementErr, ErrStripeSubscriptionPeriodOverlap):
			rejectedCount++
		default:
			require.NoError(t, settlementErr)
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, rejectedCount)

	var settlementCount int64
	require.NoError(t, db.Model(&StripeSubscriptionSettlement{}).
		Where("provider_subscription_id = ?", subscriptionID).
		Count(&settlementCount).Error)
	assert.Equal(t, int64(1), settlementCount)
	var boundOrderCount int64
	require.NoError(t, db.Model(&SubscriptionOrder{}).
		Where("payment_provider = ? AND provider_subscription_id = ?", PaymentProviderStripe, subscriptionID).
		Count(&boundOrderCount).Error)
	assert.Equal(t, int64(1), boundOrderCount)
}
