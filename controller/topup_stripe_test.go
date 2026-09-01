package controller

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
	"gorm.io/gorm"
)

func setupStripeWebhookTest(t *testing.T) *gorm.DB {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedis := common.RedisEnabled
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	originalFetchStripeRefundsForCharge := fetchStripeRefundsForCharge
	originalFetchStripeInvoicePayments := fetchStripeInvoicePayments
	originalGinMode := gin.Mode()
	var sqlDB *sql.DB
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedis
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
		fetchStripeRefundsForCharge = originalFetchStripeRefundsForCharge
		fetchStripeInvoicePayments = originalFetchStripeInvoicePayments
		gin.SetMode(originalGinMode)
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.TopUp{},
		&model.StripeWebhookEvent{},
		&model.StripePaymentReference{},
		&model.StripePaymentRecovery{},
		&model.StripePaymentAdjustment{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.StripeSubscriptionSettlement{},
		&model.StripeSubscriptionLock{},
	))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	setting.StripeApiSecret = "rk_test_placeholder"
	setting.StripeWebhookSecret = "whsec_local_test"
	setting.StripePriceId = "price_local_test"
	gin.SetMode(gin.TestMode)
	return db
}

func stripeWebhookPayloadForEvent(t *testing.T, secret string, eventID string, eventType stripe.EventType, eventObject any) ([]byte, string) {
	t.Helper()
	event := map[string]any{
		"id":          eventID,
		"object":      "event",
		"type":        string(eventType),
		"livemode":    false,
		"created":     int64(1_700_000_000),
		"api_version": stripe.APIVersion,
		"data": map[string]any{
			"object": eventObject,
		},
	}
	payload, err := common.Marshal(event)
	require.NoError(t, err)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
	return payload, signed.Header
}

func stripeWebhookPayload(t *testing.T, secret string, eventID string, session map[string]any) ([]byte, string) {
	t.Helper()
	return stripeWebhookPayloadForEvent(t, secret, eventID, stripe.EventTypeCheckoutSessionCompleted, session)
}

func invokeStripeWebhook(payload []byte, signature string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", strings.NewReader(string(payload)))
	c.Request.Header.Set("Stripe-Signature", signature)
	StripeWebhook(c)
	return recorder
}

func TestStripeWebhookRejectsInvalidSignature(t *testing.T) {
	setupStripeWebhookTest(t)
	payload := []byte(`{"id":"evt_invalid","object":"event","type":"checkout.session.completed","data":{"object":{}}}`)

	recorder := invokeStripeWebhook(payload, "t=1,v1=invalid")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestStripeWebhookRejectsIncompatibleAPIVersion(t *testing.T) {
	setupStripeWebhookTest(t)
	event := map[string]any{
		"id": "evt_legacy_version", "object": "event", "type": "customer.updated",
		"livemode": false, "api_version": "2024-06-20",
		"data": map[string]any{"object": map[string]any{"id": "cus_legacy_version", "object": "customer"}},
	}
	payload, err := common.Marshal(event)
	require.NoError(t, err)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload, Secret: setting.StripeWebhookSecret,
	})

	recorder := invokeStripeWebhook(payload, signed.Header)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestStripeWebhookReturnsForbiddenWhenWebhookIsDisabled(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_webhook_disabled",
		stripe.EventTypeCustomerUpdated,
		map[string]any{"id": "cus_webhook_disabled", "object": "customer"},
	)
	setting.StripeWebhookSecret = ""

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	var eventCount int64
	require.NoError(t, db.Model(&model.StripeWebhookEvent{}).Count(&eventCount).Error)
	assert.Zero(t, eventCount)
}

func TestStripeWebhookRejectsSameEventIDWithDifferentSignedPayload(t *testing.T) {
	db := setupStripeWebhookTest(t)
	firstPayload, firstSignature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_payload_mismatch",
		stripe.EventTypeCustomerUpdated,
		map[string]any{"id": "cus_payload_first", "object": "customer"},
	)
	secondPayload, secondSignature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_payload_mismatch",
		stripe.EventTypeCustomerUpdated,
		map[string]any{"id": "cus_payload_second", "object": "customer"},
	)

	first := invokeStripeWebhook(firstPayload, firstSignature)
	second := invokeStripeWebhook(secondPayload, secondSignature)

	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusBadRequest, second.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_payload_mismatch").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
	assert.Equal(t, 1, event.Attempts)
}

func TestStripeWebhookRejectsSignedMalformedEventData(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_malformed_checkout",
		stripe.EventTypeCheckoutSessionCompleted,
		map[string]any{"id": 123, "object": "checkout.session"},
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_malformed_checkout").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusRejected, event.Status)
	assert.Equal(t, 1, event.Attempts)
}

func TestStripeWebhookReturnsServerErrorWhenOrderDoesNotExist(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayload(t, setting.StripeWebhookSecret, "evt_missing_order", map[string]any{
		"id":                  "cs_missing_order",
		"object":              "checkout.session",
		"client_reference_id": "ref_missing_order",
		"status":              "complete",
		"payment_status":      "paid",
		"mode":                "payment",
		"currency":            "usd",
		"amount_total":        1000,
		"metadata": map[string]string{
			"trade_no":   "ref_missing_order",
			"order_kind": "topup",
			"price_id":   "price_local_test",
		},
	})

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_missing_order").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusFailed, event.Status)
	assert.Equal(t, 1, event.Attempts)
}

func TestStripeWebhookRetriesTransientFailureAndSettlesTopUpOnce(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayload(t, setting.StripeWebhookSecret, "evt_retry_missing_order", map[string]any{
		"id":                  "cs_retry_missing_order",
		"object":              "checkout.session",
		"client_reference_id": "ref_retry_missing_order",
		"status":              "complete",
		"payment_status":      "paid",
		"mode":                "payment",
		"currency":            "usd",
		"amount_total":        100,
		"customer":            "cus_retry_missing_order",
		"payment_intent": map[string]any{
			"id":            "pi_retry_missing_order",
			"latest_charge": "ch_retry_missing_order",
		},
		"metadata": map[string]string{
			"trade_no":   "ref_retry_missing_order",
			"order_kind": "topup",
			"price_id":   "price_local_test",
		},
	})

	first := invokeStripeWebhook(payload, signature)
	require.Equal(t, http.StatusServiceUnavailable, first.Code)

	user := &model.User{Id: 904, Username: "stripe_retry_user", Status: common.UserStatusEnabled, Quota: 25}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId: user.Id, Amount: 1, Money: 1, TradeNo: "ref_retry_missing_order",
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderOrderId: "cs_retry_missing_order", ProviderProductId: "price_local_test",
		ProviderCustomerId: "cus_retry_missing_order", CreditedQuota: int64(common.QuotaPerUnit),
		ExpectedAmountMinor: 100, ExpectedCurrency: "USD", Status: common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())

	second := invokeStripeWebhook(payload, signature)
	third := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, http.StatusOK, third.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_retry_missing_order").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
	assert.Equal(t, 2, event.Attempts)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, user.Quota+int(common.QuotaPerUnit), storedUser.Quota)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)
	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
}

func TestIsPermanentStripeWebhookError(t *testing.T) {
	assert.True(t, isPermanentStripeWebhookError(rejectStripeWebhook("invalid event")))
	assert.True(t, isPermanentStripeWebhookError(model.ErrPaymentMethodMismatch))
	assert.True(t, isPermanentStripeWebhookError(model.ErrTopUpStatusInvalid))
	assert.True(t, isPermanentStripeWebhookError(model.ErrStripeSubscriptionMismatch))
	assert.True(t, isPermanentStripeWebhookError(model.ErrStripeSubscriptionPeriodOverlap))
	assert.False(t, isPermanentStripeWebhookError(model.ErrTopUpNotFound))
	assert.False(t, isPermanentStripeWebhookError(assert.AnError))
}

func TestStripeWebhookIgnoresUnrelatedEventBeforeCheckoutParsing(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_customer_updated",
		stripe.EventTypeCustomerUpdated,
		map[string]any{
			"id":     "cus_unrelated",
			"object": "customer",
			"email":  map[string]any{"unexpected": true},
		},
	)

	first := invokeStripeWebhook(payload, signature)
	second := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusOK, second.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_customer_updated").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
	assert.Equal(t, 1, event.Attempts)
	assert.Empty(t, event.LastError)
}

func TestStripeWebhookRejectsRefundLivemodeMismatchWhenProvided(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_refund_livemode_mismatch",
		stripe.EventTypeRefundCreated,
		map[string]any{
			"id":             "re_livemode_mismatch",
			"object":         "refund",
			"amount":         2000,
			"currency":       "cny",
			"livemode":       true,
			"payment_intent": "pi_livemode_mismatch",
			"status":         "succeeded",
		},
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_refund_livemode_mismatch").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusRejected, event.Status)
	assert.Contains(t, event.LastError, "livemode")
}

func TestStripeWebhookAcceptsRefundWithoutObjectLivemode(t *testing.T) {
	db := setupStripeWebhookTest(t)
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_refund_without_livemode",
		stripe.EventTypeRefundCreated,
		map[string]any{
			"id":             "re_without_livemode",
			"object":         "refund",
			"amount":         2000,
			"currency":       "cny",
			"payment_intent": "pi_without_livemode",
			"status":         "succeeded",
		},
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_refund_without_livemode").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
}

func TestStripeWebhookFetchesCompleteChargeRefundList(t *testing.T) {
	db := setupStripeWebhookTest(t)
	requestedChargeID := ""
	fetchStripeRefundsForCharge = func(_ context.Context, chargeID string) ([]*stripe.Refund, error) {
		requestedChargeID = chargeID
		return []*stripe.Refund{{
			ID:            "re_refunds_has_more",
			Amount:        2000,
			Currency:      stripe.CurrencyCNY,
			PaymentIntent: &stripe.PaymentIntent{ID: "pi_refunds_has_more"},
			Charge:        &stripe.Charge{ID: chargeID},
			Status:        stripe.RefundStatusSucceeded,
		}}, nil
	}
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_charge_refunds_has_more",
		stripe.EventTypeChargeRefunded,
		map[string]any{
			"id":              "ch_refunds_has_more",
			"object":          "charge",
			"amount_refunded": 2000,
			"livemode":        false,
			"payment_intent":  "pi_refunds_has_more",
			"refunds": map[string]any{
				"object":   "list",
				"has_more": true,
				"data": []map[string]any{{
					"id":             "re_refunds_has_more",
					"object":         "refund",
					"amount":         2000,
					"currency":       "cny",
					"payment_intent": "pi_refunds_has_more",
					"status":         "succeeded",
				}},
			},
		},
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ch_refunds_has_more", requestedChargeID)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_charge_refunds_has_more").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
	assert.Empty(t, event.LastError)
}

func TestStripeWebhookFetchesChargeRefundsWhenEmbeddedListIsMissing(t *testing.T) {
	db := setupStripeWebhookTest(t)
	requestedChargeID := ""
	fetchStripeRefundsForCharge = func(_ context.Context, chargeID string) ([]*stripe.Refund, error) {
		requestedChargeID = chargeID
		return []*stripe.Refund{{
			ID:            "re_refunds_missing",
			Amount:        2000,
			Currency:      stripe.CurrencyCNY,
			PaymentIntent: &stripe.PaymentIntent{ID: "pi_refunds_missing"},
			Charge:        &stripe.Charge{ID: chargeID},
			Status:        stripe.RefundStatusSucceeded,
		}}, nil
	}
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_charge_refunds_missing",
		stripe.EventTypeChargeRefunded,
		map[string]any{
			"id":              "ch_refunds_missing",
			"object":          "charge",
			"amount_refunded": 2000,
			"livemode":        false,
			"payment_intent":  "pi_refunds_missing",
		},
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ch_refunds_missing", requestedChargeID)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_charge_refunds_missing").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
	assert.Empty(t, event.LastError)
}

func TestStripeWebhookRetriesWhenCompleteChargeRefundListIsUnavailable(t *testing.T) {
	db := setupStripeWebhookTest(t)
	fetchStripeRefundsForCharge = func(_ context.Context, _ string) ([]*stripe.Refund, error) {
		return nil, assert.AnError
	}
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_charge_refunds_fetch_failed",
		stripe.EventTypeChargeRefunded,
		map[string]any{
			"id":              "ch_refunds_fetch_failed",
			"object":          "charge",
			"amount_refunded": 2000,
			"livemode":        false,
			"payment_intent":  "pi_refunds_fetch_failed",
			"refunds": map[string]any{
				"object":   "list",
				"has_more": true,
				"data": []map[string]any{{
					"id":             "re_refunds_fetch_failed",
					"object":         "refund",
					"amount":         2000,
					"currency":       "cny",
					"payment_intent": "pi_refunds_fetch_failed",
					"status":         "succeeded",
				}},
			},
		},
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_charge_refunds_fetch_failed").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusFailed, event.Status)
	assert.Contains(t, event.LastError, "完整退款列表")
}

func TestStripeWebhookRejectsUnassociatedPendingTopUp(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{Id: 902, Username: "stripe_legacy_user", Status: common.UserStatusEnabled, Quota: 10}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId:          user.Id,
		Amount:          1,
		Money:           1,
		TradeNo:         "ref_legacy_pending",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
	payload, signature := stripeWebhookPayload(t, setting.StripeWebhookSecret, "evt_legacy_pending", map[string]any{
		"id":                  "cs_legacy_pending",
		"object":              "checkout.session",
		"client_reference_id": topUp.TradeNo,
		"status":              "complete",
		"payment_status":      "paid",
		"mode":                "payment",
		"currency":            "usd",
		"amount_total":        100,
		"payment_intent":      "pi_disabled_sales_pending",
		"customer":            "cus_legacy",
	})

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, 10, storedUser.Quota)
}

func TestStripeWebhookStillProcessesPendingOrderAfterSalesAreDisabled(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{Id: 903, Username: "stripe_disabled_sales_user", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId:              user.Id,
		Amount:              1,
		Money:               1,
		TradeNo:             "ref_disabled_sales_pending",
		PaymentMethod:       model.PaymentMethodStripe,
		PaymentProvider:     model.PaymentProviderStripe,
		ProviderOrderId:     "cs_disabled_sales_pending",
		ProviderProductId:   "price_local_test",
		CreditedQuota:       int64(common.QuotaPerUnit),
		ExpectedAmountMinor: 100,
		ExpectedCurrency:    "USD",
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
	setting.StripeApiSecret = ""
	setting.StripePriceId = ""
	payload, signature := stripeWebhookPayload(t, setting.StripeWebhookSecret, "evt_disabled_sales_pending", map[string]any{
		"id":                  topUp.ProviderOrderId,
		"object":              "checkout.session",
		"client_reference_id": topUp.TradeNo,
		"status":              "complete",
		"payment_status":      "paid",
		"mode":                "payment",
		"currency":            "usd",
		"amount_total":        100,
		"payment_intent":      "pi_disabled_sales_pending",
		"metadata": map[string]string{
			"trade_no":   topUp.TradeNo,
			"order_kind": "topup",
			"price_id":   topUp.ProviderProductId,
		},
	})

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)
}

func TestStripeWebhookCompletedTopUpIsIdempotent(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{Id: 901, Username: "stripe_webhook_user", Status: common.UserStatusEnabled, Quota: 50}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId:              user.Id,
		Amount:              2,
		Money:               2,
		TradeNo:             "ref_webhook_idempotent",
		PaymentMethod:       model.PaymentMethodStripe,
		PaymentProvider:     model.PaymentProviderStripe,
		ProviderOrderId:     "cs_webhook_idempotent",
		ProviderProductId:   "price_local_test",
		CreditedQuota:       int64(2 * common.QuotaPerUnit),
		ExpectedAmountMinor: 200,
		ExpectedCurrency:    "USD",
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
	payload, signature := stripeWebhookPayload(t, setting.StripeWebhookSecret, "evt_webhook_idempotent", map[string]any{
		"id":                  "cs_webhook_idempotent",
		"object":              "checkout.session",
		"client_reference_id": topUp.TradeNo,
		"status":              "complete",
		"payment_status":      "paid",
		"mode":                "payment",
		"currency":            "usd",
		"amount_total":        200,
		"customer":            "cus_local_webhook",
		"payment_intent":      "pi_webhook_idempotent",
		"metadata": map[string]string{
			"trade_no":   topUp.TradeNo,
			"order_kind": "topup",
			"price_id":   "price_local_test",
		},
	})

	first := invokeStripeWebhook(payload, signature)
	second := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusOK, second.Code)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, 50+int(2*common.QuotaPerUnit), storedUser.Quota)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)
	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
}

func TestStripeWebhookTopUpNearQuotaLimitFailsWithoutPartialSettlement(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{
		Id: 906, Username: "stripe_quota_limit_user", Status: common.UserStatusEnabled,
		Quota: common.MaxQuota - 100,
	}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId: user.Id, Amount: 2, Money: 2, TradeNo: "ref_quota_limit",
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderOrderId: "cs_quota_limit", ProviderProductId: "price_local_test",
		ProviderCustomerId: "cus_quota_limit", CreditedQuota: 200,
		ExpectedAmountMinor: 200, ExpectedCurrency: "USD", Status: common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
	payload, signature := stripeWebhookPayload(t, setting.StripeWebhookSecret, "evt_quota_limit", map[string]any{
		"id": topUp.ProviderOrderId, "object": "checkout.session",
		"client_reference_id": topUp.TradeNo, "status": "complete", "payment_status": "paid",
		"mode": "payment", "currency": "usd", "amount_total": 200,
		"customer": topUp.ProviderCustomerId, "payment_intent": "pi_quota_limit",
		"metadata": map[string]string{
			"trade_no": topUp.TradeNo, "order_kind": "topup", "price_id": topUp.ProviderProductId,
		},
	})

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, common.MaxQuota-100, storedUser.Quota)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_quota_limit").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusFailed, event.Status)
	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&logCount).Error)
	assert.Zero(t, logCount)
}

func TestStripeAsyncPaymentSuccessCreditsTopUpAfterCheckoutExpired(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{Id: 905, Username: "stripe_delayed_success_user", Status: common.UserStatusEnabled, Quota: 75}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId: user.Id, Amount: 2, Money: 2, TradeNo: "ref_delayed_success",
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderOrderId: "cs_delayed_success", ProviderProductId: "price_local_test",
		ProviderCustomerId: "cus_delayed_success", CreditedQuota: int64(2 * common.QuotaPerUnit),
		ExpectedAmountMinor: 200, ExpectedCurrency: "USD", Status: common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())

	expired := &stripe.CheckoutSession{
		ID: topUp.ProviderOrderId, ClientReferenceID: topUp.TradeNo,
		Status: stripe.CheckoutSessionStatusExpired, Mode: stripe.CheckoutSessionModePayment,
		AmountTotal: topUp.ExpectedAmountMinor, Currency: stripe.CurrencyUSD,
		Customer: &stripe.Customer{ID: topUp.ProviderCustomerId},
		Metadata: map[string]string{
			"trade_no": topUp.TradeNo, "order_kind": "topup", "price_id": topUp.ProviderProductId,
		},
	}
	require.NoError(t, sessionExpired(context.Background(), expired))

	paid := *expired
	paid.Status = stripe.CheckoutSessionStatusComplete
	paid.PaymentStatus = stripe.CheckoutSessionPaymentStatusPaid
	paid.PaymentIntent = &stripe.PaymentIntent{ID: "pi_delayed_success", LatestCharge: &stripe.Charge{ID: "ch_delayed_success"}}
	require.NoError(t, sessionAsyncPaymentSucceeded(context.Background(), stripe.Event{
		Type: stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded,
	}, &paid, "127.0.0.1"))

	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, user.Quota+int(2*common.QuotaPerUnit), storedUser.Quota)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)
	assert.Equal(t, paid.PaymentIntent.ID, storedTopUp.ProviderPaymentIntent)
}

func TestStripeExpiredEventCannotInvalidateAlreadyPaidTopUp(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{Id: 906, Username: "stripe_paid_expired_user", Status: common.UserStatusEnabled, Quota: 25}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId: user.Id, Amount: 2, Money: 2, TradeNo: "ref_paid_then_expired",
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderOrderId: "cs_paid_then_expired", ProviderProductId: "price_local_test",
		ProviderCustomerId: "cus_paid_then_expired", CreditedQuota: int64(2 * common.QuotaPerUnit),
		ExpectedAmountMinor: 200, ExpectedCurrency: "USD", Status: common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())

	paid := &stripe.CheckoutSession{
		ID: topUp.ProviderOrderId, ClientReferenceID: topUp.TradeNo,
		Status: stripe.CheckoutSessionStatusComplete, PaymentStatus: stripe.CheckoutSessionPaymentStatusPaid,
		Mode: stripe.CheckoutSessionModePayment, AmountTotal: topUp.ExpectedAmountMinor, Currency: stripe.CurrencyUSD,
		Customer:      &stripe.Customer{ID: topUp.ProviderCustomerId},
		PaymentIntent: &stripe.PaymentIntent{ID: "pi_paid_then_expired", LatestCharge: &stripe.Charge{ID: "ch_paid_then_expired"}},
		Metadata: map[string]string{
			"trade_no": topUp.TradeNo, "order_kind": "topup", "price_id": topUp.ProviderProductId,
		},
	}
	require.NoError(t, sessionCompleted(context.Background(), stripe.Event{Type: stripe.EventTypeCheckoutSessionCompleted}, paid, "127.0.0.1"))

	expired := *paid
	expired.Status = stripe.CheckoutSessionStatusExpired
	expired.PaymentStatus = stripe.CheckoutSessionPaymentStatusUnpaid
	require.NoError(t, sessionExpired(context.Background(), &expired))

	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, user.Quota+int(2*common.QuotaPerUnit), storedUser.Quota)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)
}

func TestStripeCompletedTopUpRejectsMismatchedSuccessfulReplay(t *testing.T) {
	db := setupStripeWebhookTest(t)
	user := &model.User{Id: 904, Username: "stripe_replay_user", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	topUp := &model.TopUp{
		UserId: user.Id, Amount: 2, Money: 2, TradeNo: "ref_completed_replay",
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderOrderId: "cs_completed_replay", ProviderProductId: "price_local_test",
		ProviderCustomerId: "cus_completed_replay", ProviderPaymentIntent: "pi_completed_replay",
		ProviderChargeId: "ch_completed_replay", CreditedQuota: int64(2 * common.QuotaPerUnit),
		ExpectedAmountMinor: 200, ExpectedCurrency: "USD", Status: common.TopUpStatusSuccess,
	}
	require.NoError(t, topUp.Insert())

	err := model.Recharge(topUp.TradeNo, model.StripeTopUpSettlement{
		CustomerId: topUp.ProviderCustomerId, PaymentIntentId: "pi_other", ChargeId: topUp.ProviderChargeId,
		AmountMinor: topUp.ExpectedAmountMinor, Currency: topUp.ExpectedCurrency, Livemode: topUp.ProviderLivemode,
	}, "127.0.0.1")

	require.ErrorIs(t, err, model.ErrStripeSnapshotMismatch)
}

func TestValidateStripeCheckoutOrderRejectsSnapshotMismatch(t *testing.T) {
	checkoutSession := &stripe.CheckoutSession{
		ID:                "cs_actual",
		ClientReferenceID: "ref_actual",
		Mode:              stripe.CheckoutSessionModePayment,
		Metadata: map[string]string{
			"trade_no":   "ref_actual",
			"order_kind": "topup",
			"price_id":   "price_actual",
		},
	}

	testCases := []struct {
		name              string
		tradeNo           string
		providerOrderID   string
		providerProductID string
		metadataKey       string
		metadataValue     string
	}{
		{name: "session id", tradeNo: "ref_actual", providerOrderID: "cs_other", providerProductID: "price_actual"},
		{name: "client reference", tradeNo: "ref_other", providerOrderID: "cs_actual", providerProductID: "price_actual"},
		{name: "price", tradeNo: "ref_actual", providerOrderID: "cs_actual", providerProductID: "price_other"},
		{name: "metadata order kind", tradeNo: "ref_actual", providerOrderID: "cs_actual", providerProductID: "price_actual", metadataKey: "order_kind", metadataValue: "subscription"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sessionCopy := *checkoutSession
			sessionCopy.Metadata = map[string]string{}
			for key, value := range checkoutSession.Metadata {
				sessionCopy.Metadata[key] = value
			}
			if testCase.metadataKey != "" {
				sessionCopy.Metadata[testCase.metadataKey] = testCase.metadataValue
			}

			err := validateStripeCheckoutOrder(
				testCase.tradeNo,
				"topup",
				testCase.providerOrderID,
				testCase.providerProductID,
				stripe.CheckoutSessionModePayment,
				&sessionCopy,
			)

			require.Error(t, err)
		})
	}
}

func insertStripeSubscriptionOrderForWebhookTest(t *testing.T, db *gorm.DB, tradeNo string) *model.SubscriptionOrder {
	t.Helper()
	user := &model.User{
		Username: "stripe_subscription_" + tradeNo,
		AffCode:  "aff_" + tradeNo,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title: "Frozen Stripe plan", PriceAmount: 12, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		Enabled: true, TotalAmount: 1200, StripePriceId: "price_subscription_local",
	}
	require.NoError(t, db.Create(plan).Error)
	order := &model.SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, Money: 12, TradeNo: tradeNo,
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderOrderId: "cs_" + tradeNo, ProviderProductId: plan.StripePriceId,
		ProviderCustomerId: "cus_" + tradeNo, ProviderSubscriptionId: common.GetPointer("sub_" + tradeNo),
		ExpectedAmountMinor: 1200, ExpectedCurrency: "USD",
		PlanTitle: plan.Title, PlanDurationUnit: plan.DurationUnit, PlanDurationValue: plan.DurationValue,
		PlanTotalAmount: plan.TotalAmount, PlanResetPeriod: model.SubscriptionResetNever,
		PlanAllowWalletOverflow: true, Status: common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())
	return order
}

func stripeSubscriptionCheckoutForWebhookTest(order *model.SubscriptionOrder) *stripe.CheckoutSession {
	return &stripe.CheckoutSession{
		ID: order.ProviderOrderId, ClientReferenceID: order.TradeNo,
		Mode: stripe.CheckoutSessionModeSubscription, AmountTotal: order.ExpectedAmountMinor,
		Currency: stripe.CurrencyUSD, Customer: &stripe.Customer{ID: order.ProviderCustomerId},
		Subscription: &stripe.Subscription{ID: *order.ProviderSubscriptionId},
		Metadata: map[string]string{
			"trade_no": order.TradeNo, "order_kind": "subscription", "price_id": order.ProviderProductId,
		},
	}
}

func stripePaidInvoiceForWebhookTest(order *model.SubscriptionOrder, invoiceID string) *stripe.Invoice {
	periodStart := time.Now().Unix()
	return &stripe.Invoice{
		ID: invoiceID, Status: stripe.InvoiceStatusPaid,
		Currency: stripe.CurrencyUSD, Total: order.ExpectedAmountMinor, AmountPaid: order.ExpectedAmountMinor,
		CollectionMethod: stripe.InvoiceCollectionMethodChargeAutomatically,
		BillingReason:    stripe.InvoiceBillingReasonSubscriptionCycle,
		Customer:         &stripe.Customer{ID: order.ProviderCustomerId},
		Payments: &stripe.InvoicePaymentList{Data: []*stripe.InvoicePayment{{
			ID: "inpy_" + invoiceID, AmountPaid: order.ExpectedAmountMinor,
			Currency: stripe.CurrencyUSD, Livemode: order.ProviderLivemode, Status: "paid",
			Payment: &stripe.InvoicePaymentPayment{
				Type:          stripe.InvoicePaymentPaymentTypePaymentIntent,
				PaymentIntent: &stripe.PaymentIntent{ID: "pi_" + invoiceID},
			},
		}}},
		Parent: &stripe.InvoiceParent{
			Type: stripe.InvoiceParentTypeSubscriptionDetails,
			SubscriptionDetails: &stripe.InvoiceParentSubscriptionDetails{
				Subscription: &stripe.Subscription{ID: *order.ProviderSubscriptionId},
				Metadata: map[string]string{
					"trade_no": order.TradeNo, "order_kind": "subscription", "price_id": order.ProviderProductId,
				},
			},
		},
		Lines: &stripe.InvoiceLineItemList{Data: []*stripe.InvoiceLineItem{{
			Currency: stripe.CurrencyUSD, Quantity: 1,
			Parent: &stripe.InvoiceLineItemParent{
				Type: stripe.InvoiceLineItemParentTypeSubscriptionItemDetails,
				SubscriptionItemDetails: &stripe.InvoiceLineItemParentSubscriptionItemDetails{
					Subscription: *order.ProviderSubscriptionId, SubscriptionItem: "si_" + order.TradeNo,
				},
			},
			Pricing: &stripe.InvoiceLineItemPricing{
				Type: stripe.InvoiceLineItemPricingTypePriceDetails,
				PriceDetails: &stripe.InvoiceLineItemPricingPriceDetails{
					Price: &stripe.Price{ID: order.ProviderProductId}, Product: "prod_" + order.TradeNo,
				},
				UnitAmountDecimal: float64(order.ExpectedAmountMinor),
			},
			Period: &stripe.Period{Start: periodStart, End: time.Unix(periodStart, 0).AddDate(0, 1, 0).Unix()},
		}}},
	}
}

func stripePaidInvoiceObjectForWebhookTest(order *model.SubscriptionOrder, invoiceID string) map[string]any {
	periodStart := int64(1_700_000_000)
	paymentIntentID := "pi_" + order.TradeNo
	chargeID := "ch_" + order.TradeNo
	return map[string]any{
		"id":                     invoiceID,
		"object":                 "invoice",
		"livemode":               false,
		"status":                 string(stripe.InvoiceStatusPaid),
		"currency":               string(stripe.CurrencyUSD),
		"total":                  order.ExpectedAmountMinor,
		"amount_paid":            order.ExpectedAmountMinor,
		"amount_remaining":       int64(0),
		"amount_paid_off_stripe": int64(0),
		"collection_method":      string(stripe.InvoiceCollectionMethodChargeAutomatically),
		"billing_reason":         string(stripe.InvoiceBillingReasonSubscriptionCycle),
		"customer":               order.ProviderCustomerId,
		"parent": map[string]any{
			"type": string(stripe.InvoiceParentTypeSubscriptionDetails),
			"subscription_details": map[string]any{
				"subscription": *order.ProviderSubscriptionId,
				"metadata": map[string]string{
					"trade_no": order.TradeNo, "order_kind": "subscription", "price_id": order.ProviderProductId,
				},
			},
		},
		"payments": map[string]any{
			"object":   "list",
			"has_more": false,
			"data": []any{map[string]any{
				"id":          "inpy_" + order.TradeNo,
				"object":      "invoice_payment",
				"amount_paid": order.ExpectedAmountMinor,
				"currency":    string(stripe.CurrencyUSD),
				"livemode":    false,
				"status":      "paid",
				"payment": map[string]any{
					"type":           "payment_intent",
					"payment_intent": paymentIntentID,
					"charge":         chargeID,
				},
			}},
		},
		"lines": map[string]any{
			"object": "list",
			"data": []any{map[string]any{
				"id":       "il_" + order.TradeNo,
				"object":   "line_item",
				"currency": string(stripe.CurrencyUSD),
				"quantity": int64(1),
				"parent": map[string]any{
					"type": string(stripe.InvoiceLineItemParentTypeSubscriptionItemDetails),
					"subscription_item_details": map[string]any{
						"subscription": *order.ProviderSubscriptionId,
						"proration":    false,
					},
				},
				"pricing": map[string]any{
					"type": string(stripe.InvoiceLineItemPricingTypePriceDetails),
					"price_details": map[string]any{
						"price":   order.ProviderProductId,
						"product": "prod_" + order.TradeNo,
					},
					"unit_amount_decimal": fmt.Sprintf("%d", order.ExpectedAmountMinor),
				},
				"period": map[string]any{
					"start": periodStart,
					"end":   time.Unix(periodStart, 0).AddDate(0, 1, 0).Unix(),
				},
			}},
		},
	}
}

func TestStripePaidInvoiceWebhookCompletesDahliaSubscriptionSettlement(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_signed_dahlia")
	invoiceObject := stripePaidInvoiceObjectForWebhookTest(order, "in_signed_dahlia")
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_signed_dahlia_invoice",
		stripe.EventTypeInvoicePaid,
		invoiceObject,
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	var settlement model.StripeSubscriptionSettlement
	require.NoError(t, db.Where("invoice_id = ?", "in_signed_dahlia").First(&settlement).Error)
	assert.Equal(t, order.ExpectedAmountMinor, settlement.InvoiceTotalMinor)
	assert.Equal(t, *order.ProviderSubscriptionId, settlement.ProviderSubscriptionId)
	var subscription model.UserSubscription
	require.NoError(t, db.Where("provider_invoice_id = ?", "in_signed_dahlia").First(&subscription).Error)
	assert.Equal(t, *order.ProviderSubscriptionId, subscription.ProviderSubscriptionId)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_signed_dahlia_invoice").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
}

func TestStripePaidInvoiceWebhookTreatsMissingAmountPaidOffStripeAsZero(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_missing_paid_off_stripe")
	invoiceObject := stripePaidInvoiceObjectForWebhookTest(order, "in_missing_paid_off_stripe")
	delete(invoiceObject, "amount_paid_off_stripe")
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_missing_paid_off_stripe",
		stripe.EventTypeInvoicePaid,
		invoiceObject,
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("provider_invoice_id = ?", "in_missing_paid_off_stripe").Count(&subscriptions).Error)
	assert.Equal(t, int64(1), subscriptions)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_missing_paid_off_stripe").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusSucceeded, event.Status)
}

func TestStripePaidInvoiceWebhookFallsBackToBoundSubscriptionWithoutTradeNo(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_without_trade_no")
	invoiceObject := stripePaidInvoiceObjectForWebhookTest(order, "in_without_trade_no")
	parent := invoiceObject["parent"].(map[string]any)
	subscriptionDetails := parent["subscription_details"].(map[string]any)
	metadata := subscriptionDetails["metadata"].(map[string]string)
	delete(metadata, "trade_no")
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_without_trade_no",
		stripe.EventTypeInvoicePaid,
		invoiceObject,
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusOK, recorder.Code)
	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	var settlement model.StripeSubscriptionSettlement
	require.NoError(t, db.Where("invoice_id = ?", "in_without_trade_no").First(&settlement).Error)
	assert.Equal(t, order.Id, settlement.SubscriptionOrderId)
	assert.Equal(t, *order.ProviderSubscriptionId, settlement.ProviderSubscriptionId)
}

func TestStripeSubscriptionAsyncPaymentFailureRemainsRetryable(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_async_failed")

	checkoutSession := stripeSubscriptionCheckoutForWebhookTest(order)
	checkoutSession.Created = 300
	require.NoError(t, sessionAsyncPaymentFailed(context.Background(), stripe.Event{Created: 100}, checkoutSession, "127.0.0.1"))

	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusPending, stored.Status)
	assert.Equal(t, "payment_failed", stored.StripeStatus)
	assert.Equal(t, int64(100), stored.StripeStatusEventTime)

	paidInvoice := stripePaidInvoiceForWebhookTest(order, "in_async_ordering")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 200}, paidInvoice))
	stored = model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, "active", stored.StripeStatus)
	assert.Equal(t, int64(200), stored.StripeStatusEventTime)
}

func TestStripeInvoiceBeforeCheckoutCompletionBindsSubscription(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_invoice_first")
	require.NoError(t, db.Model(&model.SubscriptionOrder{}).Where("id = ?", order.Id).Updates(map[string]any{
		"provider_customer_id": "", "provider_subscription_id": nil,
	}).Error)
	order.ProviderCustomerId = "cus_invoice_first"
	order.ProviderSubscriptionId = common.GetPointer("sub_invoice_first")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_invoice_first")

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))

	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	assert.Equal(t, order.ProviderCustomerId, stored.ProviderCustomerId)
	assert.Equal(t, order.ProviderSubscriptionId, stored.ProviderSubscriptionId)
	expectedPeriodEnd := invoice.Lines.Data[0].Period.End
	assert.Equal(t, expectedPeriodEnd, stored.StripeCurrentPeriodEnd)

	// checkout.session.completed may be delivered after invoice.paid.
	require.NoError(t, bindStripeSubscriptionCheckout(stripeSubscriptionCheckoutForWebhookTest(order)))
	stored = model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, expectedPeriodEnd, stored.StripeCurrentPeriodEnd)
}

func TestStripeInvoiceBeforeCheckoutCompletionRejectsMismatchedMetadata(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(*stripe.Invoice)
	}{
		{name: "order kind", mutate: func(invoice *stripe.Invoice) {
			invoice.Parent.SubscriptionDetails.Metadata["order_kind"] = "topup"
		}},
		{name: "price", mutate: func(invoice *stripe.Invoice) {
			invoice.Parent.SubscriptionDetails.Metadata["price_id"] = "price_other"
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_invoice_first_metadata_"+strings.ReplaceAll(testCase.name, " ", "_"))
			require.NoError(t, db.Model(&model.SubscriptionOrder{}).Where("id = ?", order.Id).Updates(map[string]any{
				"provider_customer_id": "", "provider_subscription_id": nil,
			}).Error)
			order.ProviderCustomerId = "cus_invoice_first_metadata"
			order.ProviderSubscriptionId = common.GetPointer("sub_invoice_first_metadata")
			invoice := stripePaidInvoiceForWebhookTest(order, "in_invoice_first_metadata_"+strings.ReplaceAll(testCase.name, " ", "_"))
			testCase.mutate(invoice)

			err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

			require.Error(t, err)
			stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
			require.NotNil(t, stored)
			assert.Nil(t, stored.ProviderSubscriptionId)
			assert.Empty(t, stored.ProviderCustomerId)
		})
	}
}

func TestStripePaidInvoiceAcceptsCustomerCreditThatChangesAmountPaid(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_credit_invoice")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_credit_invoice")
	invoice.AmountPaid = 0
	invoice.Payments = nil
	event := stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}

	require.NoError(t, processStripeInvoice(context.Background(), event, invoice))

	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("provider_invoice_id = ?", invoice.ID).Count(&subscriptions).Error)
	assert.Equal(t, int64(1), subscriptions)
	var settlement model.StripeSubscriptionSettlement
	require.NoError(t, db.Where("invoice_id = ?", invoice.ID).First(&settlement).Error)
	assert.Zero(t, settlement.AmountPaidMinor)
}

func TestStripePaidInvoiceRetriesWhenCollectedPaymentReferencesAreUnavailable(t *testing.T) {
	setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, model.DB, "sub_ref_missing_payment_references")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_missing_payment_references")
	invoice.Payments = nil
	fetchStripeInvoicePayments = func(_ context.Context, invoiceID string) ([]*stripe.InvoicePayment, error) {
		assert.Equal(t, invoice.ID, invoiceID)
		return nil, assert.AnError
	}

	err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

	require.ErrorContains(t, err, "获取 Stripe Invoice")
	assert.False(t, isPermanentStripeWebhookError(err))
}

func TestStripePaidInvoiceFetchesCompletePaymentReferences(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_fetch_payment_references")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_fetch_payment_references")
	invoice.Payments = &stripe.InvoicePaymentList{
		ListMeta: stripe.ListMeta{HasMore: true},
		Data:     invoice.Payments.Data,
	}
	requestedInvoiceID := ""
	fetchStripeInvoicePayments = func(_ context.Context, invoiceID string) ([]*stripe.InvoicePayment, error) {
		requestedInvoiceID = invoiceID
		return []*stripe.InvoicePayment{
			{
				ID: "inpy_fetch_first", AmountPaid: 500, Currency: stripe.CurrencyUSD,
				Livemode: false, Status: "paid",
				Payment: &stripe.InvoicePaymentPayment{
					Type:          stripe.InvoicePaymentPaymentTypePaymentIntent,
					PaymentIntent: &stripe.PaymentIntent{ID: "pi_fetch_first"},
				},
			},
			{
				ID: "inpy_fetch_second", AmountPaid: 700, Currency: stripe.CurrencyUSD,
				Livemode: false, Status: "paid",
				Payment: &stripe.InvoicePaymentPayment{
					Type:          stripe.InvoicePaymentPaymentTypePaymentIntent,
					PaymentIntent: &stripe.PaymentIntent{ID: "pi_fetch_second"},
				},
			},
		}, nil
	}

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))
	assert.Equal(t, invoice.ID, requestedInvoiceID)

	var settlement model.StripeSubscriptionSettlement
	require.NoError(t, db.Where("invoice_id = ?", invoice.ID).First(&settlement).Error)
	var references []model.StripePaymentReference
	require.NoError(t, db.Where("target_kind = ? AND target_id = ?", model.StripePaymentTargetSubscription, settlement.Id).
		Order("amount_minor asc").Find(&references).Error)
	require.Len(t, references, 2)
	assert.Equal(t, int64(500), references[0].AmountMinor)
	assert.Equal(t, int64(700), references[1].AmountMinor)
}

func TestStripePaidInvoiceRetriesWhenPaymentReferenceFetchFails(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_payment_references_retry")
	invoiceObject := stripePaidInvoiceObjectForWebhookTest(order, "in_payment_references_retry")
	payments := invoiceObject["payments"].(map[string]any)
	payments["has_more"] = true
	fetchStripeInvoicePayments = func(_ context.Context, _ string) ([]*stripe.InvoicePayment, error) {
		return nil, assert.AnError
	}
	payload, signature := stripeWebhookPayloadForEvent(
		t,
		setting.StripeWebhookSecret,
		"evt_payment_references_retry",
		stripe.EventTypeInvoicePaid,
		invoiceObject,
	)

	recorder := invokeStripeWebhook(payload, signature)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_payment_references_retry").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusFailed, event.Status)
	assert.Contains(t, event.LastError, "付款引用")
}

func TestStripePaidInvoiceRejectsInvalidSettlementEnvelope(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(*stripe.Invoice)
	}{
		{name: "discounted total", mutate: func(invoice *stripe.Invoice) { invoice.Total-- }},
		{name: "remaining balance", mutate: func(invoice *stripe.Invoice) { invoice.AmountRemaining = 1 }},
		{name: "paid out of band", mutate: func(invoice *stripe.Invoice) { invoice.AmountPaidOffStripe = 1 }},
		{name: "manual collection", mutate: func(invoice *stripe.Invoice) {
			invoice.CollectionMethod = stripe.InvoiceCollectionMethodSendInvoice
		}},
		{name: "subscription update", mutate: func(invoice *stripe.Invoice) {
			invoice.BillingReason = stripe.InvoiceBillingReasonSubscriptionUpdate
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_invoice_envelope_"+strings.ReplaceAll(testCase.name, " ", "_"))
			invoice := stripePaidInvoiceForWebhookTest(order, "in_invoice_envelope_"+strings.ReplaceAll(testCase.name, " ", "_"))
			testCase.mutate(invoice)

			err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

			require.Error(t, err)
			var subscriptions int64
			require.NoError(t, db.Model(&model.UserSubscription{}).Where("provider_invoice_id = ?", invoice.ID).Count(&subscriptions).Error)
			assert.Zero(t, subscriptions)
		})
	}
}

func TestStripeInvoiceRejectsMismatchedSubscriptionMetadata(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(*stripe.Invoice)
	}{
		{name: "trade number", mutate: func(invoice *stripe.Invoice) {
			invoice.Parent.SubscriptionDetails.Metadata["trade_no"] = "sub_ref_other"
		}},
		{name: "order kind", mutate: func(invoice *stripe.Invoice) {
			invoice.Parent.SubscriptionDetails.Metadata["order_kind"] = "topup"
		}},
		{name: "price", mutate: func(invoice *stripe.Invoice) {
			invoice.Parent.SubscriptionDetails.Metadata["price_id"] = "price_other"
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_invoice_metadata_"+strings.ReplaceAll(testCase.name, " ", "_"))
			invoice := stripePaidInvoiceForWebhookTest(order, "in_invoice_metadata_"+strings.ReplaceAll(testCase.name, " ", "_"))
			testCase.mutate(invoice)

			err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

			require.Error(t, err)
			var subscriptions int64
			require.NoError(t, db.Model(&model.UserSubscription{}).Where("provider_invoice_id = ?", invoice.ID).Count(&subscriptions).Error)
			assert.Zero(t, subscriptions)
		})
	}
}

func TestStripePaidInvoiceRejectsReusedInvoiceWithDifferentPayload(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_reused_invoice")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_reused_invoice")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))
	invoice.AmountPaid--
	invoice.Payments.Data[0].AmountPaid--

	err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

	require.ErrorIs(t, err, model.ErrStripeInvoiceAlreadyBound)
}

func TestStripePaidInvoiceRejectsReusedInvoiceWithDifferentUnitAmount(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_reused_invoice_unit_amount")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_reused_invoice_unit_amount")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))
	invoice.Lines.Data[0].Pricing.UnitAmountDecimal++

	err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

	require.ErrorIs(t, err, model.ErrStripeInvoiceAlreadyBound)
}

func TestStripePaidInvoiceRejectsReusedInvoiceWithDifferentTotal(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_reused_invoice_total")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_reused_invoice_total")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))
	var settlement model.StripeSubscriptionSettlement
	require.NoError(t, db.Where("invoice_id = ?", invoice.ID).First(&settlement).Error)
	assert.Equal(t, invoice.Total, settlement.InvoiceTotalMinor)

	require.NoError(t, db.Model(&model.SubscriptionOrder{}).
		Where("id = ?", order.Id).
		Update("expected_amount_minor", order.ExpectedAmountMinor+1).Error)
	invoice.Total++

	err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

	require.ErrorIs(t, err, model.ErrStripeInvoiceAlreadyBound)
}

func TestStripeInvoicePaymentFailureCanRecoverWithPaidInvoice(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_retry_recovery")
	failedInvoice := stripePaidInvoiceForWebhookTest(order, "in_retry_failed")
	failedInvoice.Status = stripe.InvoiceStatusOpen

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaymentFailed, Created: 100}, failedInvoice))
	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusPending, stored.Status)
	assert.Equal(t, "payment_failed", stored.StripeStatus)

	paidInvoice := stripePaidInvoiceForWebhookTest(order, "in_retry_paid")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 200}, paidInvoice))
	stored = model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	assert.Equal(t, "active", stored.StripeStatus)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("provider_invoice_id = ?", paidInvoice.ID).Count(&subscriptions).Error)
	assert.Equal(t, int64(1), subscriptions)
}

func TestStripeSubscriptionLifecycleIgnoresStaleEvent(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_lifecycle_ordering")
	subscription := &stripe.Subscription{
		ID: *order.ProviderSubscriptionId, Customer: &stripe.Customer{ID: order.ProviderCustomerId},
		Status: stripe.SubscriptionStatusActive,
		Items:  &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{CurrentPeriodEnd: 9_000}}},
	}

	require.NoError(t, processStripeSubscriptionLifecycle(stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionUpdated, Created: 200,
	}, subscription))
	subscription.Status = stripe.SubscriptionStatusPastDue
	subscription.Items.Data[0].CurrentPeriodEnd = 8_000
	require.NoError(t, processStripeSubscriptionLifecycle(stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionUpdated, Created: 100,
	}, subscription))

	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, string(stripe.SubscriptionStatusActive), stored.StripeStatus)
	assert.Equal(t, int64(200), stored.StripeStatusEventTime)
	assert.Equal(t, int64(9_000), stored.StripeCurrentPeriodEnd)
}

func TestStripeSubscriptionDeletionWinsEqualTimestampRegardlessOfDeliveryOrder(t *testing.T) {
	testCases := []struct {
		name        string
		firstType   stripe.EventType
		firstState  stripe.SubscriptionStatus
		secondType  stripe.EventType
		secondState stripe.SubscriptionStatus
	}{
		{
			name:        "updated then deleted",
			firstType:   stripe.EventTypeCustomerSubscriptionUpdated,
			firstState:  stripe.SubscriptionStatusActive,
			secondType:  stripe.EventTypeCustomerSubscriptionDeleted,
			secondState: stripe.SubscriptionStatusCanceled,
		},
		{
			name:        "deleted then updated",
			firstType:   stripe.EventTypeCustomerSubscriptionDeleted,
			firstState:  stripe.SubscriptionStatusCanceled,
			secondType:  stripe.EventTypeCustomerSubscriptionUpdated,
			secondState: stripe.SubscriptionStatusActive,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_equal_lifecycle_"+strings.ReplaceAll(testCase.name, " ", "_"))
			subscription := &stripe.Subscription{
				ID:       *order.ProviderSubscriptionId,
				Customer: &stripe.Customer{ID: order.ProviderCustomerId},
			}

			subscription.Status = testCase.firstState
			require.NoError(t, processStripeSubscriptionLifecycle(stripe.Event{
				Type: testCase.firstType, Created: 200,
			}, subscription))
			subscription.Status = testCase.secondState
			require.NoError(t, processStripeSubscriptionLifecycle(stripe.Event{
				Type: testCase.secondType, Created: 200,
			}, subscription))

			stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
			require.NotNil(t, stored)
			assert.Equal(t, string(stripe.SubscriptionStatusCanceled), stored.StripeStatus)
			assert.Equal(t, int64(200), stored.StripeStatusEventTime)
		})
	}
}

func TestStripeSubscriptionDeletionPreservesPaidServicePeriod(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_deleted_period")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_deleted_period")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))

	subscription := &stripe.Subscription{
		ID: *order.ProviderSubscriptionId, Customer: &stripe.Customer{ID: order.ProviderCustomerId},
		Status: stripe.SubscriptionStatusCanceled,
	}
	require.NoError(t, processStripeSubscriptionLifecycle(stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionDeleted, Created: 300,
	}, subscription))

	var userSubscription model.UserSubscription
	require.NoError(t, db.Where("provider_invoice_id = ?", invoice.ID).First(&userSubscription).Error)
	assert.Equal(t, "active", userSubscription.Status)
	assert.Equal(t, invoice.Lines.Data[0].Period.End, userSubscription.EndTime)
	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, string(stripe.SubscriptionStatusCanceled), stored.StripeStatus)
}

func TestStripePaidInvoiceDoesNotRollBackNewerCanceledStatus(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_stale_paid_invoice")
	subscription := &stripe.Subscription{
		ID: *order.ProviderSubscriptionId, Customer: &stripe.Customer{ID: order.ProviderCustomerId},
		Status: stripe.SubscriptionStatusCanceled,
		Items:  &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{CurrentPeriodEnd: 9_000}}},
	}
	require.NoError(t, processStripeSubscriptionLifecycle(stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionDeleted, Created: 300,
	}, subscription))

	invoice := stripePaidInvoiceForWebhookTest(order, "in_stale_paid_invoice")
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 200}, invoice))

	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, string(stripe.SubscriptionStatusCanceled), stored.StripeStatus)
	assert.Equal(t, int64(300), stored.StripeStatusEventTime)
	assert.Equal(t, int64(9_000), stored.StripeCurrentPeriodEnd)
	var userSubscription model.UserSubscription
	require.NoError(t, db.Where("provider_invoice_id = ?", invoice.ID).First(&userSubscription).Error)
	assert.Equal(t, "active", userSubscription.Status)
}

func TestStripePaidInvoiceUsesFrozenPlanSnapshotAfterPlanMutation(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_frozen_plan")
	require.NoError(t, db.Model(&model.SubscriptionPlan{}).Where("id = ?", order.PlanId).Updates(map[string]any{
		"title": "Mutated plan", "total_amount": 999999, "duration_value": 12,
	}).Error)
	invoice := stripePaidInvoiceForWebhookTest(order, "in_frozen_plan")

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))

	var userSubscription model.UserSubscription
	require.NoError(t, db.Where("provider_invoice_id = ?", invoice.ID).First(&userSubscription).Error)
	assert.Equal(t, order.PlanTitle, userSubscription.PlanTitle)
	assert.Equal(t, order.PlanTotalAmount, userSubscription.AmountTotal)
	assert.Equal(t, invoice.Lines.Data[0].Period.End, userSubscription.EndTime)
}

func TestStripePaidInvoiceStoresOnlySettlementAuditFields(t *testing.T) {
	setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, model.DB, "sub_ref_minimal_payload")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_minimal_payload")
	invoice.Customer.Email = "private@example.com"
	invoice.Description = "private invoice description"

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))

	stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, stored)
	assert.Contains(t, stored.ProviderPayload, `"invoice_id":"in_minimal_payload"`)
	assert.Contains(t, stored.ProviderPayload, `"amount_paid_minor":1200`)
	assert.NotContains(t, stored.ProviderPayload, "private@example.com")
	assert.NotContains(t, stored.ProviderPayload, "private invoice description")
}

func TestStripePaidInvoiceUsesStripeMonthEndPeriodAsAuthoritative(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_month_end_period")
	invoice := stripePaidInvoiceForWebhookTest(order, "in_month_end_period")
	invoice.Lines.Data[0].Period = &stripe.Period{
		Start: time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC).Unix(),
		End:   time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC).Unix(),
	}

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice))

	var userSubscription model.UserSubscription
	require.NoError(t, db.Where("provider_invoice_id = ?", invoice.ID).First(&userSubscription).Error)
	assert.Equal(t, invoice.Lines.Data[0].Period.Start, userSubscription.StartTime)
	assert.Equal(t, invoice.Lines.Data[0].Period.End, userSubscription.EndTime)
}

func TestStripePaidInvoiceRejectsOverlappingServicePeriodForSameSubscription(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_overlap_period")
	firstInvoice := stripePaidInvoiceForWebhookTest(order, "in_overlap_period_first")
	firstInvoice.Lines.Data[0].Period = &stripe.Period{Start: 100, End: 200}
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, firstInvoice))

	overlappingInvoice := stripePaidInvoiceForWebhookTest(order, "in_overlap_period_second")
	overlappingInvoice.Lines.Data[0].Period = &stripe.Period{Start: 150, End: 250}

	err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 200}, overlappingInvoice)

	require.ErrorIs(t, err, model.ErrStripeSubscriptionPeriodOverlap)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).
		Where("provider_subscription_id = ?", *order.ProviderSubscriptionId).
		Count(&subscriptions).Error)
	assert.Equal(t, int64(1), subscriptions)
}

func TestStripeSubscriptionCannotBindToAnotherLocalOrder(t *testing.T) {
	db := setupStripeWebhookTest(t)
	firstOrder := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_unique_binding_first")
	secondOrder := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_unique_binding_second")

	err := secondOrder.BindStripeSubscription(
		secondOrder.ProviderCustomerId,
		*firstOrder.ProviderSubscriptionId,
		secondOrder.ProviderLivemode,
	)

	require.ErrorIs(t, err, model.ErrStripeSubscriptionMismatch)
	stored := model.GetSubscriptionOrderByTradeNo(secondOrder.TradeNo)
	require.NotNil(t, stored)
	require.NotNil(t, stored.ProviderSubscriptionId)
	assert.Equal(t, *secondOrder.ProviderSubscriptionId, *stored.ProviderSubscriptionId)
}

func TestStripePaidInvoiceAllowsAdjacentServicePeriodsForSameSubscription(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_adjacent_period")
	firstInvoice := stripePaidInvoiceForWebhookTest(order, "in_adjacent_period_first")
	firstInvoice.Lines.Data[0].Period = &stripe.Period{Start: 100, End: 200}
	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, firstInvoice))

	adjacentInvoice := stripePaidInvoiceForWebhookTest(order, "in_adjacent_period_second")
	adjacentInvoice.Lines.Data[0].Period = &stripe.Period{Start: 200, End: 300}

	require.NoError(t, processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 200}, adjacentInvoice))
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).
		Where("provider_subscription_id = ?", *order.ProviderSubscriptionId).
		Count(&subscriptions).Error)
	assert.Equal(t, int64(2), subscriptions)
}

func TestStripePaidInvoiceRejectsImmutableSnapshotMismatches(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(*stripe.Invoice)
	}{
		{name: "customer", mutate: func(invoice *stripe.Invoice) { invoice.Customer.ID = "cus_other" }},
		{name: "subscription", mutate: func(invoice *stripe.Invoice) { invoice.Parent.SubscriptionDetails.Subscription.ID = "sub_other" }},
		{name: "price", mutate: func(invoice *stripe.Invoice) { invoice.Lines.Data[0].Pricing.PriceDetails.Price.ID = "price_other" }},
		{name: "quantity", mutate: func(invoice *stripe.Invoice) { invoice.Lines.Data[0].Quantity = 2 }},
		{name: "unit amount", mutate: func(invoice *stripe.Invoice) { invoice.Lines.Data[0].Pricing.UnitAmountDecimal++ }},
		{name: "currency", mutate: func(invoice *stripe.Invoice) { invoice.Currency = stripe.CurrencyEUR }},
		{name: "livemode", mutate: func(invoice *stripe.Invoice) { invoice.Livemode = true }},
		{name: "period", mutate: func(invoice *stripe.Invoice) { invoice.Lines.Data[0].Period.End = invoice.Lines.Data[0].Period.Start }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeSubscriptionOrderForWebhookTest(t, db, "sub_ref_mismatch_"+strings.ReplaceAll(testCase.name, " ", "_"))
			invoice := stripePaidInvoiceForWebhookTest(order, "in_mismatch_"+strings.ReplaceAll(testCase.name, " ", "_"))
			testCase.mutate(invoice)

			err := processStripeInvoice(context.Background(), stripe.Event{Type: stripe.EventTypeInvoicePaid, Created: 100}, invoice)

			require.Error(t, err)
			stored := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
			require.NotNil(t, stored)
			assert.Equal(t, common.TopUpStatusPending, stored.Status)
			var subscriptions int64
			require.NoError(t, db.Model(&model.UserSubscription{}).Where("provider_invoice_id = ?", invoice.ID).Count(&subscriptions).Error)
			assert.Zero(t, subscriptions)
		})
	}
}
