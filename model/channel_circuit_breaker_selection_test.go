package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetRandomSatisfiedChannelExcludingWorksWithoutMemoryCache(t *testing.T) {
	previousDB := DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
		DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		initCol()
	})

	weight := uint(1)
	autoBan := 1
	priority18 := int64(10)
	priority28 := int64(5)
	for _, channel := range []*Channel{
		{Id: 18, Type: 1, Key: "test-18", Status: common.ChannelStatusEnabled, Name: "channel-18", Weight: &weight, Priority: &priority18, AutoBan: &autoBan, Group: "default", Models: "shared-model"},
		{Id: 28, Type: 1, Key: "test-28", Status: common.ChannelStatusEnabled, Name: "channel-28", Weight: &weight, Priority: &priority28, AutoBan: &autoBan, Group: "default", Models: "shared-model"},
	} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}

	channel, err := GetRandomSatisfiedChannelExcluding(
		"default",
		"shared-model",
		"",
		0,
		"",
		map[int]struct{}{18: {}},
	)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 28, channel.Id)
}
