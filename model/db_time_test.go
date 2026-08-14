package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetDBTimestampFallsBackWhenDatabaseIsNil(t *testing.T) {
	before := time.Now().Unix()
	timestamp := getDBTimestamp(nil)
	after := time.Now().Unix()

	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)
}
