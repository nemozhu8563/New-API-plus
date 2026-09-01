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
	previousSensitiveWordsHighRisk := setting.SensitiveWordsHighRiskToString()
	previousSensitiveWordsAudit := setting.SensitiveWordsAuditToString()

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
		setting.SensitiveWordsHighRiskFromString(previousSensitiveWordsHighRisk)
		setting.SensitiveWordsAuditFromString(previousSensitiveWordsAudit)
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

func TestSensitiveWordCategoryOptionsPersistAndReloadIndependently(t *testing.T) {
	setupSensitiveOptionTestDB(t)

	InitOptionMap()
	require.NoError(t, UpdateOption("SensitiveWordsHighRisk", "risk-one\nrisk-two"))
	require.NoError(t, UpdateOption("SensitiveWordsAudit", "audit-one"))
	require.NoError(t, UpdateOption("SensitiveWords", "nsfw-one"))

	assert.Equal(t, "risk-one\nrisk-two", setting.SensitiveWordsHighRiskToString())
	assert.Equal(t, "audit-one", setting.SensitiveWordsAuditToString())
	assert.Equal(t, "nsfw-one", setting.SensitiveWordsToString())

	setting.SensitiveWordsHighRiskFromString("temporary-risk")
	setting.SensitiveWordsAuditFromString("temporary-audit")
	setting.SensitiveWordsFromString("temporary-nsfw")
	InitOptionMap()

	assert.Equal(t, "risk-one\nrisk-two", setting.SensitiveWordsHighRiskToString())
	assert.Equal(t, "audit-one", setting.SensitiveWordsAuditToString())
	assert.Equal(t, "nsfw-one", setting.SensitiveWordsToString())

	require.NoError(t, UpdateOption("SensitiveWordsAudit", ""))
	setting.SensitiveWordsAuditFromString("temporary-audit")
	InitOptionMap()
	assert.Empty(t, setting.GetSensitiveWordsAudit())
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
