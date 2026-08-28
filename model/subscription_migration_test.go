package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacySubscriptionOrderProviderId struct {
	Id                     int    `gorm:"primaryKey"`
	TradeNo                string `gorm:"unique;type:varchar(255);index"`
	PaymentProvider        string `gorm:"type:varchar(50);default:''"`
	ProviderSubscriptionId string `gorm:"type:varchar(255);not null;default:'';index"`
}

func TestMigrateSubscriptionOrderProviderSubscriptionIdSQLite(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Table("subscription_orders").AutoMigrate(&legacySubscriptionOrderProviderId{}))
	require.NoError(t, db.Table("subscription_orders").Create(&[]legacySubscriptionOrderProviderId{
		{Id: 1, TradeNo: "legacy-unbound-1", PaymentProvider: PaymentProviderStripe},
		{Id: 2, TradeNo: "legacy-unbound-2", PaymentProvider: PaymentProviderStripe, ProviderSubscriptionId: "   "},
	}).Error)

	require.NoError(t, migrateSubscriptionOrderProviderSubscriptionId())
	require.NoError(t, db.AutoMigrate(&SubscriptionOrder{}))

	var legacyUnboundCount int64
	require.NoError(t, db.Model(&SubscriptionOrder{}).
		Where("id IN ? AND provider_subscription_id IS NULL", []int{1, 2}).
		Count(&legacyUnboundCount).Error)
	assert.Equal(t, int64(2), legacyUnboundCount)
	assert.True(t, db.Migrator().HasIndex(&SubscriptionOrder{}, "idx_subscription_order_provider_subscription"))

	require.NoError(t, db.Create(&SubscriptionOrder{
		Id: 3, TradeNo: "migration-null-1", PaymentProvider: PaymentProviderStripe,
	}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{
		Id: 4, TradeNo: "migration-null-2", PaymentProvider: PaymentProviderStripe,
	}).Error)

	stripeSubscription := "sub_shared_migration_test"
	require.NoError(t, db.Create(&SubscriptionOrder{
		Id: 5, TradeNo: "migration-stripe", PaymentProvider: PaymentProviderStripe,
		ProviderSubscriptionId: &stripeSubscription,
	}).Error)
	assert.Error(t, db.Create(&SubscriptionOrder{
		Id: 6, TradeNo: "migration-stripe-duplicate", PaymentProvider: PaymentProviderStripe,
		ProviderSubscriptionId: &stripeSubscription,
	}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{
		Id: 7, TradeNo: "migration-other-provider", PaymentProvider: PaymentProviderCreem,
		ProviderSubscriptionId: &stripeSubscription,
	}).Error)

	require.NoError(t, migrateSubscriptionOrderProviderSubscriptionId())
	require.NoError(t, db.AutoMigrate(&SubscriptionOrder{}))
}

func TestMigrateSubscriptionOrderProviderSubscriptionIdRejectsHistoricalDuplicates(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Table("subscription_orders").AutoMigrate(&legacySubscriptionOrderProviderId{}))
	require.NoError(t, db.Table("subscription_orders").Create(&[]legacySubscriptionOrderProviderId{
		{Id: 11, TradeNo: "legacy-duplicate-1", PaymentProvider: PaymentProviderStripe, ProviderSubscriptionId: "sub_duplicate"},
		{Id: 12, TradeNo: "legacy-duplicate-2", PaymentProvider: PaymentProviderStripe, ProviderSubscriptionId: "sub_duplicate"},
	}).Error)

	err = migrateSubscriptionOrderProviderSubscriptionId()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate subscription order provider binding")
	assert.Contains(t, err.Error(), "sub_duplicate")
	assert.Contains(t, err.Error(), "11")
	assert.Contains(t, err.Error(), "12")
}

func TestStripeSubscriptionSettlementMigrationBackfillsInvoiceTotal(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Exec(`CREATE TABLE stripe_subscription_settlements (
		id integer PRIMARY KEY,
		invoice_id varchar(255),
		unit_amount_minor bigint NOT NULL,
		amount_paid_minor bigint NOT NULL
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO stripe_subscription_settlements
		(id, invoice_id, unit_amount_minor, amount_paid_minor) VALUES (1, 'in_legacy', 1200, 1200)`).Error)

	require.NoError(t, db.Migrator().AddColumn(&StripeSubscriptionSettlement{}, "InvoiceTotalMinor"))

	var invoiceTotalMinor int64
	require.NoError(t, db.Table("stripe_subscription_settlements").
		Select("invoice_total_minor").Where("id = ?", 1).Scan(&invoiceTotalMinor).Error)
	assert.Zero(t, invoiceTotalMinor)
}

func TestSubscriptionPlanSQLiteMigrationAddsVisibilityAndNormalizesMonthlyBilling(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Exec(`CREATE TABLE subscription_plans (
		id integer PRIMARY KEY,
		title varchar(128) NOT NULL,
		price_amount decimal(10,6) NOT NULL,
		enabled numeric,
		duration_unit varchar(16),
		duration_value integer,
		custom_seconds bigint,
		quota_reset_period varchar(16),
		quota_reset_custom_seconds bigint
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO subscription_plans
		(id, title, price_amount, enabled, duration_unit, duration_value, custom_seconds, quota_reset_period, quota_reset_custom_seconds)
		VALUES
		(1, 'Enabled legacy plan', 399, 1, 'day', 28, 0, 'custom', 604800),
		(2, 'Disabled legacy plan', 99, 0, NULL, NULL, NULL, NULL, NULL)`).Error)

	require.NoError(t, ensureSubscriptionPlanTableSQLite())
	assert.True(t, db.Migrator().HasColumn(&SubscriptionPlan{}, "PublicVisible"))
	assert.True(t, db.Migrator().HasColumn(&SubscriptionPlan{}, "Recommended"))
	require.NoError(t, migrateSubscriptionPlansToMonthlyBilling())

	var plans []SubscriptionPlan
	require.NoError(t, db.Order("id asc").Find(&plans).Error)
	require.Len(t, plans, 2)
	for _, plan := range plans {
		assert.Equal(t, SubscriptionDurationMonth, plan.DurationUnit)
		assert.Equal(t, 1, plan.DurationValue)
		assert.Zero(t, plan.CustomSeconds)
		assert.Equal(t, SubscriptionResetBillingCycle, plan.QuotaResetPeriod)
		assert.Zero(t, plan.QuotaResetCustomSeconds)
		assert.False(t, plan.Recommended)
	}
	require.NotNil(t, plans[0].PublicVisible)
	assert.True(t, *plans[0].PublicVisible)
	require.NotNil(t, plans[1].PublicVisible)
	assert.False(t, *plans[1].PublicVisible)

	require.NoError(t, db.Model(&SubscriptionPlan{}).
		Where("id = ?", plans[0].Id).
		Update("public_visible", false).Error)
	require.NoError(t, migrateSubscriptionPlansToMonthlyBilling())
	var hiddenPlan SubscriptionPlan
	require.NoError(t, db.First(&hiddenPlan, plans[0].Id).Error)
	require.NotNil(t, hiddenPlan.PublicVisible)
	assert.False(t, *hiddenPlan.PublicVisible)
}
