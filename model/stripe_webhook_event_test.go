package model

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClaimStripeWebhookEventConcurrentSingleOwner(t *testing.T) {
	previousDB := DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&StripeWebhookEvent{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() {
		DB = previousDB
		_ = sqlDB.Close()
	})

	const workers = 2
	start := make(chan struct{})
	claims := make([]*StripeWebhookEventClaim, workers)
	errs := make([]error, workers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer waitGroup.Done()
			<-start
			for attempt := 0; attempt < 10; attempt++ {
				claims[index], errs[index] = ClaimStripeWebhookEvent(
					"evt_concurrent_claim", "invoice.paid", false, "payload_hash", time.Minute,
				)
				if errs[index] == nil || !strings.Contains(errs[index].Error(), "locked") {
					return
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	close(start)
	waitGroup.Wait()

	owners := 0
	inProgress := 0
	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		require.NotNil(t, claims[i])
		if claims[i].ShouldProcess {
			owners++
		}
		if claims[i].InProgress {
			inProgress++
		}
	}
	assert.Equal(t, 1, owners)
	assert.Equal(t, 1, inProgress)

	var count int64
	require.NoError(t, db.Model(&StripeWebhookEvent{}).
		Where("stripe_event_id = ?", "evt_concurrent_claim").
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestFinishStripeWebhookEventRejectsStaleLeaseOwner(t *testing.T) {
	truncateTables(t)

	first, err := ClaimStripeWebhookEvent("evt_lease_reclaimed", "invoice.paid", false, "payload_hash", time.Second)
	require.NoError(t, err)
	require.True(t, first.ShouldProcess)
	require.NoError(t, DB.Model(&StripeWebhookEvent{}).
		Where("stripe_event_id = ?", first.Event.StripeEventId).
		Update("lease_until", time.Now().Unix()-1).Error)

	second, err := ClaimStripeWebhookEvent("evt_lease_reclaimed", "invoice.paid", false, "payload_hash", time.Minute)
	require.NoError(t, err)
	require.True(t, second.ShouldProcess)
	require.Greater(t, second.Attempt, first.Attempt)

	err = FinishStripeWebhookEvent(first.Event.StripeEventId, first.Attempt, StripeWebhookEventStatusFailed, "stale worker")
	require.ErrorIs(t, err, ErrStripeWebhookClaimLost)

	var stored StripeWebhookEvent
	require.NoError(t, DB.Where("stripe_event_id = ?", first.Event.StripeEventId).First(&stored).Error)
	assert.Equal(t, StripeWebhookEventStatusProcessing, stored.Status)
	assert.Equal(t, second.Attempt, stored.Attempts)
	assert.Empty(t, stored.LastError)

	require.NoError(t, FinishStripeWebhookEvent(second.Event.StripeEventId, second.Attempt, StripeWebhookEventStatusSucceeded, ""))
	require.NoError(t, DB.Where("stripe_event_id = ?", first.Event.StripeEventId).First(&stored).Error)
	assert.Equal(t, StripeWebhookEventStatusSucceeded, stored.Status)
}

func TestClaimStripeWebhookEventKeepsTerminalEventFinal(t *testing.T) {
	truncateTables(t)

	first, err := ClaimStripeWebhookEvent("evt_terminal", "customer.updated", false, "payload_hash", time.Minute)
	require.NoError(t, err)
	require.True(t, first.ShouldProcess)
	require.NoError(t, FinishStripeWebhookEvent(first.Event.StripeEventId, first.Attempt, StripeWebhookEventStatusSucceeded, ""))

	second, err := ClaimStripeWebhookEvent("evt_terminal", "customer.updated", false, "payload_hash", time.Minute)
	require.NoError(t, err)
	assert.True(t, second.AlreadyFinal)
	assert.False(t, second.ShouldProcess)
	assert.Equal(t, first.Attempt, second.Event.Attempts)
}
