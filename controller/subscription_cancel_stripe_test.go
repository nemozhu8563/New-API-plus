package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func invokeStripeSubscriptionCancel(userID int, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/stripe/cancel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userID)
	CancelStripeSubscription(c)
	return recorder
}

func TestCancelStripeSubscriptionSchedulesPeriodEndCancellation(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalUpdate := updateStripeSubscription
	t.Cleanup(func() { updateStripeSubscription = originalUpdate })
	setting.StripeApiSecret = "rk_test_placeholder"

	user := &model.User{Username: "cancel-stripe-user", Email: "cancel-stripe@example.com"}
	require.NoError(t, db.Create(user).Error)
	subscriptionID := "sub_cancel_test"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "cancel-stripe-order", PlanId: 17, PlanTitle: "Builder",
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		ProviderCustomerId: "cus_cancel_test", ProviderSubscriptionId: &subscriptionID,
		ProviderLivemode: false, StripeStatus: "active", Status: common.TopUpStatusSuccess,
	}).Error)

	updateStripeSubscription = func(_ context.Context, id string, params *stripe.SubscriptionUpdateParams) (*stripe.Subscription, error) {
		assert.Equal(t, subscriptionID, id)
		require.NotNil(t, params.CancelAtPeriodEnd)
		assert.True(t, *params.CancelAtPeriodEnd)
		return &stripe.Subscription{
			ID: subscriptionID, Customer: &stripe.Customer{ID: "cus_cancel_test"},
			Status: stripe.SubscriptionStatusActive, Livemode: false,
			CancelAtPeriodEnd: true, CancelAt: 2_000,
			Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{
				{ID: "si_later", CurrentPeriodEnd: 2_100},
				{ID: "si_earlier", CurrentPeriodEnd: 1_900},
			}},
		}, nil
	}

	recorder := invokeStripeSubscriptionCancel(user.Id, `{"subscription_id":"sub_cancel_test"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                            `json:"success"`
		Data    model.StripeSubscriptionSummary `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, subscriptionID, response.Data.SubscriptionId)
	assert.True(t, response.Data.CancelAtPeriodEnd)
	assert.Equal(t, int64(1_900), response.Data.CurrentPeriodEnd)

	var order model.SubscriptionOrder
	require.NoError(t, db.Where("provider_subscription_id = ?", subscriptionID).First(&order).Error)
	assert.True(t, order.StripeCancelAtPeriodEnd)
	assert.Equal(t, int64(2_000), order.StripeCancelAt)
	assert.Equal(t, int64(1_900), order.StripeCurrentPeriodEnd)
	assert.Positive(t, order.StripeCancelRequestedAt)
}

func TestCancelStripeSubscriptionRejectsSubscriptionOwnedByAnotherUser(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalUpdate := updateStripeSubscription
	t.Cleanup(func() { updateStripeSubscription = originalUpdate })
	setting.StripeApiSecret = "rk_test_placeholder"

	owner := &model.User{Username: "cancel-owner", AffCode: "cancel-owner-aff"}
	requester := &model.User{Username: "cancel-requester", AffCode: "cancel-requester-aff"}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(requester).Error)
	subscriptionID := "sub_cancel_owned"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: owner.Id, TradeNo: "cancel-owned-order", PaymentProvider: model.PaymentProviderStripe,
		ProviderCustomerId: "cus_cancel_owned", ProviderSubscriptionId: &subscriptionID,
		ProviderLivemode: false, Status: common.TopUpStatusSuccess,
	}).Error)
	called := false
	updateStripeSubscription = func(_ context.Context, _ string, _ *stripe.SubscriptionUpdateParams) (*stripe.Subscription, error) {
		called = true
		return nil, errors.New("must not be called")
	}

	recorder := invokeStripeSubscriptionCancel(requester.Id, `{"subscription_id":"sub_cancel_owned"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, called)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}

func TestCancelStripeSubscriptionRejectsMismatchedStripeResponse(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalUpdate := updateStripeSubscription
	t.Cleanup(func() { updateStripeSubscription = originalUpdate })
	setting.StripeApiSecret = "rk_test_placeholder"

	user := &model.User{Username: "cancel-mismatch-user"}
	require.NoError(t, db.Create(user).Error)
	subscriptionID := "sub_cancel_mismatch"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "cancel-mismatch-order", PaymentProvider: model.PaymentProviderStripe,
		ProviderCustomerId: "cus_cancel_mismatch", ProviderSubscriptionId: &subscriptionID,
		ProviderLivemode: false, Status: common.TopUpStatusSuccess,
	}).Error)
	updateStripeSubscription = func(_ context.Context, _ string, _ *stripe.SubscriptionUpdateParams) (*stripe.Subscription, error) {
		return &stripe.Subscription{
			ID: subscriptionID, Customer: &stripe.Customer{ID: "cus_other"},
			Status: stripe.SubscriptionStatusActive, CancelAtPeriodEnd: true,
			Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{CurrentPeriodEnd: 1_900}}},
		}, nil
	}

	recorder := invokeStripeSubscriptionCancel(user.Id, `{"subscription_id":"sub_cancel_mismatch"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	var order model.SubscriptionOrder
	require.NoError(t, db.Where("provider_subscription_id = ?", subscriptionID).First(&order).Error)
	assert.False(t, order.StripeCancelAtPeriodEnd)
}

func TestGetSubscriptionSelfShowsOnlyCurrentStripeSubscriptionsAndKeepsHistoricalInvoices(t *testing.T) {
	db := setupStripeWebhookTest(t)
	setting.StripeApiSecret = "rk_test_placeholder"
	user := &model.User{Username: "self-stripe-billing", AffCode: "self-stripe-billing-aff", BillingDebt: 25}
	require.NoError(t, db.Create(user).Error)
	subscriptionID := "sub_self_billing"
	order := &model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "self-stripe-billing-order", PlanId: 23, PlanTitle: "Developer",
		PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
		ProviderCustomerId: "cus_self_billing", ProviderSubscriptionId: &subscriptionID,
		ProviderLivemode: false, StripeStatus: "active", StripeCurrentPeriodEnd: 2_000,
		Status: common.TopUpStatusSuccess,
	}
	require.NoError(t, db.Create(order).Error)
	require.NoError(t, db.Create(&model.StripeSubscriptionSettlement{
		InvoiceId: "in_self_billing", SubscriptionOrderId: order.Id, UserSubscriptionId: 99,
		ProviderCustomerId: "cus_self_billing", ProviderSubscriptionId: subscriptionID,
		ProviderProductId: "price_self_billing", Quantity: 1, UnitAmountMinor: 2_000,
		InvoiceTotalMinor: 2_000, AmountPaidMinor: 2_000, Currency: "CNY",
		PeriodStart: 1_000, PeriodEnd: 2_000, CreatedAt: 1_001,
	}).Error)

	historicalOrders := []struct {
		tradeNo        string
		subscriptionID string
		stripeStatus   string
		invoiceID      string
		periodStart    int64
		periodEnd      int64
		userSubID      int
	}{
		{
			tradeNo: "self-stripe-unknown-order", subscriptionID: "sub_self_billing_unknown",
			invoiceID: "in_self_billing_unknown", periodStart: 500, periodEnd: 1_000, userSubID: 98,
		},
		{
			tradeNo: "self-stripe-canceled-order", subscriptionID: "sub_self_billing_canceled",
			stripeStatus: "canceled", invoiceID: "in_self_billing_canceled",
			periodStart: 100, periodEnd: 500, userSubID: 97,
		},
	}
	for index, historical := range historicalOrders {
		historicalOrder := &model.SubscriptionOrder{
			UserId: user.Id, TradeNo: historical.tradeNo, PlanId: 24 + index, PlanTitle: "Standard",
			PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
			ProviderCustomerId: "cus_self_billing", ProviderSubscriptionId: &historical.subscriptionID,
			ProviderLivemode: false, StripeStatus: historical.stripeStatus,
			StripeCurrentPeriodEnd: historical.periodEnd, Status: common.TopUpStatusSuccess,
		}
		require.NoError(t, db.Create(historicalOrder).Error)
		require.NoError(t, db.Create(&model.StripeSubscriptionSettlement{
			InvoiceId: historical.invoiceID, SubscriptionOrderId: historicalOrder.Id,
			UserSubscriptionId: historical.userSubID, ProviderCustomerId: "cus_self_billing",
			ProviderSubscriptionId: historical.subscriptionID, ProviderProductId: "price_self_billing_history",
			Quantity: 1, UnitAmountMinor: 2_000, InvoiceTotalMinor: 2_000,
			AmountPaidMinor: 2_000, Currency: "CNY", PeriodStart: historical.periodStart,
			PeriodEnd: historical.periodEnd, CreatedAt: historical.periodStart + 1,
		}).Error)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/self", nil)
	c.Set("id", user.Id)
	GetSubscriptionSelf(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			BillingDebt         int64                             `json:"billing_debt"`
			StripeSubscriptions []model.StripeSubscriptionSummary `json:"stripe_subscriptions"`
			StripeInvoices      []model.StripeInvoiceSummary      `json:"stripe_invoices"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, int64(25), response.Data.BillingDebt)
	require.Len(t, response.Data.StripeSubscriptions, 1)
	assert.Equal(t, subscriptionID, response.Data.StripeSubscriptions[0].SubscriptionId)
	require.Len(t, response.Data.StripeInvoices, 3)
	assert.Equal(t, "in_self_billing", response.Data.StripeInvoices[0].InvoiceId)
	assert.Equal(t, "in_self_billing_unknown", response.Data.StripeInvoices[1].InvoiceId)
	assert.Equal(t, "in_self_billing_canceled", response.Data.StripeInvoices[2].InvoiceId)
}
