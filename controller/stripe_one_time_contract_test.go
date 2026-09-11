package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func TestStripeCheckoutRejectsSubscriptionMode(t *testing.T) {
	for _, eventType := range []stripe.EventType{
		stripe.EventTypeCheckoutSessionCompleted,
		stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded,
		stripe.EventTypeCheckoutSessionAsyncPaymentFailed,
		stripe.EventTypeCheckoutSessionExpired,
	} {
		t.Run(string(eventType), func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "reject_recurring")
			checkout := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)
			checkout.Mode = stripe.CheckoutSessionModeSubscription
			checkout.Customer = &stripe.Customer{ID: "cus_recurring"}
			checkout.Subscription = &stripe.Subscription{ID: "sub_recurring"}
			payload, err := common.Marshal(checkout)
			require.NoError(t, err)
			err = processStripeWebhookEvent(context.Background(), stripe.Event{
				ID: "evt_reject_recurring", Type: eventType, Created: 1,
				Data: &stripe.EventData{Raw: payload},
			}, "127.0.0.1")
			require.Error(t, err)
			assert.True(t, isPermanentStripeWebhookError(err))
			require.NoError(t, db.First(order, order.Id).Error)
			assert.Equal(t, common.TopUpStatusPending, order.Status)
			var count int64
			require.NoError(t, db.Model(&model.UserSubscription{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestStripeRecurringEventsCannotGrantEntitlements(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "ignore_recurring")
	for _, eventType := range []stripe.EventType{
		stripe.EventTypeInvoicePaid,
		stripe.EventTypeInvoicePaymentFailed,
		stripe.EventTypeCustomerSubscriptionUpdated,
		stripe.EventTypeCustomerSubscriptionDeleted,
	} {
		err := processStripeWebhookEvent(context.Background(), stripe.Event{
			ID: "evt_ignore_recurring", Type: eventType,
			Data: &stripe.EventData{Raw: []byte(`{"id":"unmanaged_recurring","metadata":{"trade_no":"ignore_recurring"}}`)},
		}, "127.0.0.1")
		require.NoError(t, err)
	}
	require.NoError(t, db.First(order, order.Id).Error)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	var count int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Count(&count).Error)
	assert.Zero(t, count)
}
