package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSensitiveOptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousSensitiveWords := setting.SensitiveWordsToString()

	common.OptionMapRWMutex.RLock()
	previousOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		previousOptionMap[key] = value
	}
	common.OptionMapRWMutex.RUnlock()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Option{}))

	t.Cleanup(func() {
		setting.SensitiveWordsFromString(previousSensitiveWords)
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestSensitiveWordsOptionPersistsOverrideAndExplicitClear(t *testing.T) {
	db := setupSensitiveOptionTestDB(t)
	builtInWords := setting.SensitiveWordsToString()
	require.NotEmpty(t, builtInWords)

	InitOptionMap()
	assert.Equal(t, builtInWords, setting.SensitiveWordsToString())

	const customWords = "custom-block\nanother-block"
	require.NoError(t, UpdateOption("SensitiveWords", customWords))
	assert.Equal(t, customWords, setting.SensitiveWordsToString())

	setting.SensitiveWordsFromString(builtInWords)
	InitOptionMap()
	assert.Equal(t, customWords, setting.SensitiveWordsToString(), "the persisted custom list must override built-in defaults after restart")

	require.NoError(t, UpdateOption("SensitiveWords", ""))
	assert.Empty(t, setting.GetSensitiveWords())

	setting.SensitiveWordsFromString(builtInWords)
	InitOptionMap()
	assert.Empty(t, setting.GetSensitiveWords(), "an explicit empty option must remain disabled after restart")

	var option Option
	require.NoError(t, db.First(&option, "key = ?", "SensitiveWords").Error)
	assert.Empty(t, option.Value)
}
