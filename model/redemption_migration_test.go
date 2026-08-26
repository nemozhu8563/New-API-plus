package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyRedemption struct {
	Id           int `gorm:"primaryKey"`
	UserId       int
	Key          string `gorm:"type:char(32);uniqueIndex"`
	Status       int    `gorm:"default:1"`
	Name         string
	Quota        int `gorm:"default:100"`
	CreatedTime  int64
	RedeemedTime int64
	UsedUserId   int
	DeletedAt    gorm.DeletedAt
	ExpiredTime  int64
}

func (legacyRedemption) TableName() string {
	return "redemptions"
}

func TestRedemptionMigrationDefaultsLegacyCodesToQuota(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyRedemption{}))
	require.NoError(t, db.Create(&legacyRedemption{
		Key:    "40000000000000000000000000000001",
		Status: 1,
		Name:   "legacy quota code",
		Quota:  500,
	}).Error)

	require.NoError(t, db.AutoMigrate(&Redemption{}))
	assert.True(t, db.Migrator().HasColumn(&Redemption{}, "BenefitType"))
	assert.True(t, db.Migrator().HasColumn(&Redemption{}, "SubscriptionPlanSnapshot"))

	var legacy Redemption
	require.NoError(t, db.First(&legacy, "name = ?", "legacy quota code").Error)
	assert.Equal(t, RedemptionBenefitQuota, legacy.BenefitType)
	assert.Equal(t, 500, legacy.Quota)
	assert.Zero(t, legacy.SubscriptionPlanId)
	assert.Empty(t, legacy.SubscriptionPlanSnapshot)
	assert.Zero(t, legacy.UsedSubscriptionId)

	// A second migration must remain safe for restart-driven AutoMigrate.
	require.NoError(t, db.AutoMigrate(&Redemption{}))
}
