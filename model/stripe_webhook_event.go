package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StripeWebhookEventStatusProcessing = "processing"
	StripeWebhookEventStatusSucceeded  = "succeeded"
	StripeWebhookEventStatusFailed     = "failed"
	StripeWebhookEventStatusRejected   = "rejected"
)

var ErrStripeWebhookClaimLost = errors.New("stripe webhook event claim lost")

type StripeWebhookEvent struct {
	Id            int64  `json:"id"`
	StripeEventId string `json:"stripe_event_id" gorm:"type:varchar(255);uniqueIndex"`
	EventType     string `json:"event_type" gorm:"type:varchar(128);index"`
	Livemode      bool   `json:"livemode"`
	PayloadSha256 string `json:"payload_sha256" gorm:"type:varchar(64)"`
	Status        string `json:"status" gorm:"type:varchar(32);index"`
	Attempts      int    `json:"attempts"`
	LeaseUntil    int64  `json:"lease_until" gorm:"index"`
	LastError     string `json:"last_error" gorm:"type:text"`
	CreateTime    int64  `json:"create_time"`
	UpdateTime    int64  `json:"update_time"`
}

type StripeWebhookEventClaim struct {
	Event           *StripeWebhookEvent
	Attempt         int
	ShouldProcess   bool
	AlreadyFinal    bool
	InProgress      bool
	PayloadMismatch bool
}

func ClaimStripeWebhookEvent(stripeEventId string, eventType string, livemode bool, payloadSha256 string, leaseDuration time.Duration) (*StripeWebhookEventClaim, error) {
	stripeEventId = strings.TrimSpace(stripeEventId)
	eventType = strings.TrimSpace(eventType)
	payloadSha256 = strings.TrimSpace(payloadSha256)
	if stripeEventId == "" || eventType == "" || payloadSha256 == "" {
		return nil, errors.New("invalid stripe webhook event")
	}
	if leaseDuration <= 0 {
		return nil, errors.New("invalid stripe webhook lease")
	}

	now := time.Now().Unix()
	leaseUntil := now + int64(leaseDuration/time.Second)
	if leaseUntil <= now {
		leaseUntil = now + 1
	}
	var claim *StripeWebhookEventClaim
	err := DB.Transaction(func(tx *gorm.DB) error {
		candidate := &StripeWebhookEvent{
			StripeEventId: stripeEventId,
			EventType:     eventType,
			Livemode:      livemode,
			PayloadSha256: payloadSha256,
			Status:        StripeWebhookEventStatusProcessing,
			Attempts:      1,
			LeaseUntil:    leaseUntil,
			CreateTime:    now,
			UpdateTime:    now,
		}
		insert := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "stripe_event_id"}}, DoNothing: true}).
			Create(candidate)
		if insert.Error != nil {
			return insert.Error
		}
		inserted := insert.RowsAffected == 1

		event := &StripeWebhookEvent{}
		if err := lockForUpdate(tx).Where("stripe_event_id = ?", stripeEventId).First(event).Error; err != nil {
			return err
		}
		if inserted {
			claim = &StripeWebhookEventClaim{Event: event, Attempt: event.Attempts, ShouldProcess: true}
			return nil
		}

		payloadMismatch := event.EventType != eventType ||
			event.Livemode != livemode ||
			event.PayloadSha256 != payloadSha256
		if event.Status == StripeWebhookEventStatusSucceeded || event.Status == StripeWebhookEventStatusRejected {
			claim = &StripeWebhookEventClaim{
				Event:           event,
				AlreadyFinal:    true,
				PayloadMismatch: payloadMismatch,
			}
			return nil
		}
		if payloadMismatch {
			if err := finishStripeWebhookEventWithTx(tx, event, StripeWebhookEventStatusRejected, "event id reused with different payload"); err != nil {
				return err
			}
			claim = &StripeWebhookEventClaim{
				Event:           event,
				AlreadyFinal:    true,
				PayloadMismatch: true,
			}
			return nil
		}
		if event.Status == StripeWebhookEventStatusProcessing && event.LeaseUntil > now {
			claim = &StripeWebhookEventClaim{Event: event, InProgress: true}
			return nil
		}

		event.Status = StripeWebhookEventStatusProcessing
		event.Attempts++
		event.LeaseUntil = leaseUntil
		event.LastError = ""
		event.UpdateTime = now
		if err := tx.Save(event).Error; err != nil {
			return err
		}
		claim = &StripeWebhookEventClaim{Event: event, Attempt: event.Attempts, ShouldProcess: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claim, nil
}

func FinishStripeWebhookEvent(stripeEventId string, attempt int, status string, lastError string) error {
	if strings.TrimSpace(stripeEventId) == "" || attempt <= 0 {
		return errors.New("invalid stripe webhook event claim")
	}
	if status != StripeWebhookEventStatusSucceeded &&
		status != StripeWebhookEventStatusFailed &&
		status != StripeWebhookEventStatusRejected {
		return errors.New("invalid stripe webhook event status")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		event := &StripeWebhookEvent{}
		if err := lockForUpdate(tx).Where("stripe_event_id = ?", stripeEventId).First(event).Error; err != nil {
			return err
		}
		if event.Status != StripeWebhookEventStatusProcessing || event.Attempts != attempt {
			return ErrStripeWebhookClaimLost
		}
		return finishStripeWebhookEventWithTx(tx, event, status, lastError)
	})
}

func finishStripeWebhookEventWithTx(tx *gorm.DB, event *StripeWebhookEvent, status string, lastError string) error {
	event.Status = status
	event.LastError = lastError
	event.LeaseUntil = 0
	event.UpdateTime = time.Now().Unix()
	return tx.Save(event).Error
}
