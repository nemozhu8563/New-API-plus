package controller

import (
	"context"
	"database/sql"
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
	model.InvalidateSubscriptionPlanCache(1)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedis := common.RedisEnabled
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	originalFetchStripeRefundsForCharge := fetchStripeRefundsForCharge
	originalGinMode := gin.Mode()
	var sqlDB *sql.DB
	t.Cleanup(func() {
		model.InvalidateSubscriptionPlanCache(1)
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedis
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
		fetchStripeRefundsForCharge = originalFetchStripeRefundsForCharge
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

func TestGenStripeLinkConfiguresWeChatPayForWebCheckout(t *testing.T) {
	originalAPISecret := setting.StripeApiSecret
	originalPriceID := setting.StripePriceId
	originalCreateStripeCheckoutSession := createStripeCheckoutSession
	t.Cleanup(func() {
		setting.StripeApiSecret = originalAPISecret
		setting.StripePriceId = originalPriceID
		createStripeCheckoutSession = originalCreateStripeCheckoutSession
	})

	setting.StripeApiSecret = "rk_test_placeholder"
	setting.StripePriceId = "price_local_test"

	var capturedParams *stripe.CheckoutSessionCreateParams
	createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		capturedParams = params
		return &stripe.CheckoutSession{
			ID:  "cs_local_test",
			URL: "https://checkout.stripe.com/c/pay/cs_local_test",
		}, nil
	}

	result, err := genStripeLink(context.Background(), "ref_local_test", "", "", 1, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, capturedParams)
	require.NotNil(t, capturedParams.PaymentMethodOptions)
	require.NotNil(t, capturedParams.PaymentMethodOptions.WeChatPay)
	require.NotNil(t, capturedParams.PaymentMethodOptions.WeChatPay.Client)

	assert.Equal(t, stripe.CheckoutSessionModePayment, stripe.CheckoutSessionMode(*capturedParams.Mode))
	assert.Empty(t, capturedParams.PaymentMethodTypes)
	assert.Equal(t,
		string(stripe.CheckoutSessionPaymentMethodOptionsWeChatPayClientWeb),
		*capturedParams.PaymentMethodOptions.WeChatPay.Client,
	)
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
		Quota: common.MaxWalletQuota - 100,
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
	assert.Equal(t, common.MaxWalletQuota-100, storedUser.Quota)
	storedTopUp := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, storedTopUp)
	assert.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
	var event model.StripeWebhookEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_quota_limit").First(&event).Error)
	assert.Equal(t, model.StripeWebhookEventStatusFailed, event.Status)
	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&logCount).Error)
	assert.Zero(t, logCount)

	// A failed delivery can settle after capacity is corrected, even when the
	// wallet is still above the single-charge int32 limit. Replays stay idempotent.
	require.NoError(t, db.Model(user).Update("quota", 2_500_000_000).Error)
	assert.Equal(t, http.StatusOK, invokeStripeWebhook(payload, signature).Code)
	assert.Equal(t, http.StatusOK, invokeStripeWebhook(payload, signature).Code)
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, 2_500_000_200, storedUser.Quota)
	assert.Equal(t, common.TopUpStatusSuccess, model.GetTopUpByTradeNo(topUp.TradeNo).Status)
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
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

func insertStripeOneTimeSubscriptionOrderForWebhookTest(t *testing.T, db *gorm.DB, tradeNo string) *model.SubscriptionOrder {
	t.Helper()
	user := &model.User{
		Username: "stripe_one_time_subscription_" + tradeNo,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:                   "One-time Stripe plan",
		PriceAmount:             399,
		Currency:                model.SubscriptionCurrencyCNY,
		DurationUnit:            model.SubscriptionDurationMonth,
		DurationValue:           1,
		TotalAmount:             1000,
		QuotaResetPeriod:        model.SubscriptionResetBillingCycle,
		QuotaResetCustomSeconds: 0,
		Enabled:                 true,
		StripePriceId:           "price_one_time_subscription",
	}
	require.NoError(t, db.Create(plan).Error)
	order := &model.SubscriptionOrder{
		UserId:                  user.Id,
		PlanId:                  plan.Id,
		Money:                   plan.PriceAmount,
		TradeNo:                 tradeNo,
		PaymentMethod:           model.PaymentMethodStripe,
		PaymentProvider:         model.PaymentProviderStripe,
		ProviderOrderId:         "cs_" + tradeNo,
		ProviderProductId:       plan.StripePriceId,
		ExpectedAmountMinor:     39900,
		ExpectedCurrency:        model.SubscriptionCurrencyCNY,
		PlanTitle:               plan.Title,
		PlanDurationUnit:        plan.DurationUnit,
		PlanDurationValue:       plan.DurationValue,
		PlanTotalAmount:         plan.TotalAmount,
		PlanResetPeriod:         plan.QuotaResetPeriod,
		PlanAllowWalletOverflow: true,
		Status:                  common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())
	return order
}

func stripeOneTimeSubscriptionCheckoutForWebhookTest(order *model.SubscriptionOrder) *stripe.CheckoutSession {
	return &stripe.CheckoutSession{
		ID:                order.ProviderOrderId,
		ClientReferenceID: order.TradeNo,
		Status:            stripe.CheckoutSessionStatusComplete,
		PaymentStatus:     stripe.CheckoutSessionPaymentStatusPaid,
		Mode:              stripe.CheckoutSessionModePayment,
		AmountTotal:       order.ExpectedAmountMinor,
		Currency:          stripe.CurrencyCNY,
		Customer:          &stripe.Customer{ID: "cus_" + order.TradeNo},
		PaymentIntent:     &stripe.PaymentIntent{ID: "pi_" + order.TradeNo, LatestCharge: &stripe.Charge{ID: "ch_" + order.TradeNo}},
		Metadata: map[string]string{
			"trade_no": order.TradeNo, "order_kind": "subscription", "price_id": order.ProviderProductId,
		},
	}
}

func TestStripeOneTimeSubscriptionCheckoutCreatesApplicationEntitlement(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "ref_one_time_subscription")
	checkoutSession := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)

	require.NoError(t, sessionCompleted(context.Background(), stripe.Event{
		ID: "evt_one_time_subscription", Type: stripe.EventTypeCheckoutSessionCompleted,
	}, checkoutSession, "127.0.0.1"))

	storedOrder := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusSuccess, storedOrder.Status)
	assert.Equal(t, checkoutSession.Customer.ID, storedOrder.ProviderCustomerId)

	var subscription model.UserSubscription
	require.NoError(t, db.Where("user_id = ? AND plan_id = ?", order.UserId, order.PlanId).First(&subscription).Error)
	assert.Equal(t, "active", subscription.Status)
	assert.Equal(t, int64(1000), subscription.AmountTotal)
	assert.Greater(t, subscription.EndTime, subscription.StartTime)

	var topUp model.TopUp
	require.NoError(t, db.Where("trade_no = ?", order.TradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	var paymentReference model.StripePaymentReference
	require.NoError(t, db.Where("payment_intent_id = ?", "pi_"+order.TradeNo).First(&paymentReference).Error)
	assert.Equal(t, model.StripePaymentTargetSubscriptionEntitlement, paymentReference.TargetKind)
	assert.Equal(t, subscription.Id, paymentReference.TargetId)
}

func TestStripeOneTimeSubscriptionUsesPurchasedSnapshotAfterPlanChanges(t *testing.T) {
	for _, change := range []string{"edited", "deleted"} {
		t.Run(change, func(t *testing.T) {
			db := setupStripeWebhookTest(t)
			order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "snapshot_"+change)
			order.PlanDowngradeGroup = "default"
			order.PlanAllowWalletOverflow = false
			require.NoError(t, db.Save(order).Error)
			if change == "deleted" {
				require.NoError(t, db.Delete(&model.SubscriptionPlan{}, order.PlanId).Error)
			} else {
				require.NoError(t, db.Model(&model.SubscriptionPlan{}).Where("id = ?", order.PlanId).Updates(map[string]any{
					"title": "Edited plan", "total_amount": 2000,
					"duration_unit": model.SubscriptionDurationHour, "duration_value": 2, "custom_seconds": 7200,
					"quota_reset_period": model.SubscriptionResetCustom, "quota_reset_custom_seconds": 60,
					"downgrade_group": "edited_default", "allow_wallet_overflow": true,
				}).Error)
			}
			model.InvalidateSubscriptionPlanCache(order.PlanId)
			checkoutSession := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)
			event := stripe.Event{ID: "evt_snapshot_" + change, Type: stripe.EventTypeCheckoutSessionCompleted}

			require.NoError(t, sessionCompleted(context.Background(), event, checkoutSession, "127.0.0.1"))
			require.NoError(t, sessionCompleted(context.Background(), event, checkoutSession, "127.0.0.1"))

			var subscriptions []model.UserSubscription
			require.NoError(t, db.Where("user_id = ?", order.UserId).Find(&subscriptions).Error)
			require.Len(t, subscriptions, 1)
			subscription := subscriptions[0]
			assert.Equal(t, "active", subscription.Status)
			assert.Equal(t, order.PlanTitle, subscription.PlanTitle)
			assert.Equal(t, order.PlanTotalAmount, subscription.AmountTotal)
			assert.Zero(t, subscription.AmountUsed)
			assert.Equal(t, time.Unix(subscription.StartTime, 0).AddDate(0, 1, 0).Unix(), subscription.EndTime)
			assert.Equal(t, order.PlanResetPeriod, subscription.QuotaResetPeriod)
			assert.Equal(t, order.PlanResetCustomSeconds, subscription.QuotaResetCustomSeconds)
			assert.Zero(t, subscription.LastResetTime)
			assert.Zero(t, subscription.NextResetTime)
			assert.Equal(t, order.PlanUpgradeGroup, subscription.UpgradeGroup)
			assert.Equal(t, order.PlanDowngradeGroup, subscription.DowngradeGroup)
			assert.Equal(t, order.PlanAllowWalletOverflow, subscription.AllowWalletOverflow)
			storedOrder := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
			require.NotNil(t, storedOrder)
			assert.Equal(t, common.TopUpStatusSuccess, storedOrder.Status)
		})
	}
}

func TestStripeOneTimeSubscriptionAsyncPaymentSucceededCompletesExpiredOrder(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "ref_one_time_subscription_async")
	expired := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)
	expired.Status = stripe.CheckoutSessionStatusExpired
	expired.PaymentStatus = stripe.CheckoutSessionPaymentStatusUnpaid
	expired.PaymentIntent = nil

	require.NoError(t, sessionExpired(context.Background(), expired))
	storedOrder := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusExpired, storedOrder.Status)

	paid := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)
	require.NoError(t, sessionAsyncPaymentSucceeded(context.Background(), stripe.Event{
		ID: "evt_one_time_subscription_async", Type: stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded,
	}, paid, "127.0.0.1"))

	storedOrder = model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusSuccess, storedOrder.Status)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ?", order.UserId).Count(&subscriptions).Error)
	assert.Equal(t, int64(1), subscriptions)
}

func TestStripeOneTimeSubscriptionSnapshotPreservesPurchaseLimit(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "snapshot_purchase_limit")
	require.NoError(t, db.Model(&model.SubscriptionPlan{}).Where("id = ?", order.PlanId).Update("max_purchase_per_user", 1).Error)
	model.InvalidateSubscriptionPlanCache(order.PlanId)
	require.NoError(t, db.Create(&model.UserSubscription{
		UserId: order.UserId, PlanId: order.PlanId, Status: "expired",
	}).Error)

	err := sessionCompleted(context.Background(), stripe.Event{
		ID: "evt_snapshot_purchase_limit", Type: stripe.EventTypeCheckoutSessionCompleted,
	}, stripeOneTimeSubscriptionCheckoutForWebhookTest(order), "127.0.0.1")

	require.EqualError(t, err, "已达到该套餐购买上限")
	storedOrder := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusPending, storedOrder.Status)
	var count int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ?", order.UserId).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Model(&model.StripePaymentReference{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestStripeOneTimeSubscriptionAsyncPaymentFailedExpiresOrder(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "ref_one_time_subscription_failed")
	checkoutSession := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)
	checkoutSession.PaymentStatus = stripe.CheckoutSessionPaymentStatusUnpaid
	checkoutSession.PaymentIntent = nil

	require.NoError(t, sessionAsyncPaymentFailed(context.Background(), stripe.Event{
		ID: "evt_one_time_subscription_failed", Created: 1,
	}, checkoutSession, "127.0.0.1"))

	storedOrder := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusExpired, storedOrder.Status)
}

func TestStripeOneTimeSubscriptionRefundRevokesApplicationEntitlement(t *testing.T) {
	db := setupStripeWebhookTest(t)
	order := insertStripeOneTimeSubscriptionOrderForWebhookTest(t, db, "ref_one_time_subscription_refund")
	checkoutSession := stripeOneTimeSubscriptionCheckoutForWebhookTest(order)
	require.NoError(t, sessionCompleted(context.Background(), stripe.Event{
		ID: "evt_one_time_subscription_refund_payment", Type: stripe.EventTypeCheckoutSessionCompleted,
	}, checkoutSession, "127.0.0.1"))

	require.NoError(t, processStripeRefund(context.Background(), stripe.Event{
		ID: "evt_one_time_subscription_refund", Type: stripe.EventTypeRefundCreated,
		Livemode: false, Created: 2,
	}, &stripe.Refund{
		ID:     "re_one_time_subscription",
		Amount: order.ExpectedAmountMinor, Currency: stripe.CurrencyCNY,
		PaymentIntent: &stripe.PaymentIntent{ID: "pi_" + order.TradeNo},
		Charge:        &stripe.Charge{ID: "ch_" + order.TradeNo},
		Status:        stripe.RefundStatusSucceeded,
	}, "", ""))

	var subscription model.UserSubscription
	require.NoError(t, db.Where("user_id = ? AND plan_id = ?", order.UserId, order.PlanId).First(&subscription).Error)
	assert.Equal(t, "cancelled", subscription.Status)
	var recovery model.StripePaymentRecovery
	require.NoError(t, db.Where("target_kind = ? AND target_id = ?", model.StripePaymentTargetSubscriptionEntitlement, subscription.Id).First(&recovery).Error)
	assert.True(t, recovery.EntitlementRevoked)
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
