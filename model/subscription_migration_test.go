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
