package model

import (
	"strconv"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Protect the persisted wallet domain without depending on the misleading
// type:int tag: GORM derives the integer width from the Go field size.
func TestWalletQuotaSchemaSupportsLargeBalances(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("large wallets require a 64-bit application")
	}
	s, err := schema.Parse(&User{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	quota := s.LookUpField("Quota")
	require.NotNil(t, quota)
	for _, tc := range []struct {
		name    string
		dialect gorm.Dialector
		want    string
	}{
		{"mysql", mysql.New(mysql.Config{SkipInitializeWithVersion: true}), "bigint"},
		{"postgres", postgres.New(postgres.Config{}), "bigint"},
		{"sqlite", sqlite.Open(":memory:"), "integer"},
	} {
		t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.want, tc.dialect.DataTypeOf(quota)) })
	}
}
