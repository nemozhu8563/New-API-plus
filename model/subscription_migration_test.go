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
