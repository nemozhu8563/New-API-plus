package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func invokeStripeBillingPortal(userId int) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/stripe/portal", nil)
	c.Set("id", userId)
	CreateStripeBillingPortalSession(c)
	return recorder
}

func TestCreateStripeBillingPortalSessionUsesLatestCustomerForCurrentMode(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalCreate := createStripeBillingPortalSession
	originalAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		createStripeBillingPortalSession = originalCreate
		system_setting.ServerAddress = originalAddress
	})
	system_setting.ServerAddress = "https://test.tryvalo.com/"
	setting.StripeApiSecret = "rk_test_placeholder"

	user := &model.User{Username: "portal-user", Email: "portal@example.com", StripeCustomer: "cus_legacy"}
	require.NoError(t, db.Create(user).Error)
	liveSubscriptionId := "sub_live"
	testSubscriptionId := "sub_test"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "sub_live_order", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, ProviderCustomerId: "cus_live",
		ProviderSubscriptionId: &liveSubscriptionId, ProviderLivemode: true, CompleteTime: 200,
	}).Error)
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "sub_test_order", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, ProviderCustomerId: "cus_test",
		ProviderSubscriptionId: &testSubscriptionId, ProviderLivemode: false, CompleteTime: 100,
	}).Error)

	createStripeBillingPortalSession = func(_ context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
		require.NotNil(t, params.Customer)
		require.NotNil(t, params.ReturnURL)
		assert.Equal(t, "cus_test", *params.Customer)
		assert.Equal(t, "https://test.tryvalo.com/wallet", *params.ReturnURL)
		return &stripe.BillingPortalSession{
			ID: "bps_test", URL: "https://billing.stripe.com/p/session/test", Livemode: false,
		}, nil
	}

	recorder := invokeStripeBillingPortal(user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			PortalURL string `json:"portal_url"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, "https://billing.stripe.com/p/session/test", response.Data.PortalURL)
}

func TestCreateStripeBillingPortalSessionRejectsAccountWithoutStripeCustomer(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalCreate := createStripeBillingPortalSession
	t.Cleanup(func() { createStripeBillingPortalSession = originalCreate })

	user := &model.User{Username: "no-portal-user", Email: "no-portal@example.com"}
	require.NoError(t, db.Create(user).Error)
	called := false
	createStripeBillingPortalSession = func(_ context.Context, _ *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
		called = true
		return nil, errors.New("must not be called")
	}

	recorder := invokeStripeBillingPortal(user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, called)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}

func TestCreateStripeBillingPortalSessionDoesNotUseLegacyCustomerFromAnotherMode(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalCreate := createStripeBillingPortalSession
	t.Cleanup(func() { createStripeBillingPortalSession = originalCreate })

	user := &model.User{Username: "legacy-portal-user", Email: "legacy-portal@example.com", StripeCustomer: "cus_legacy_unknown_mode"}
	require.NoError(t, db.Create(user).Error)
	liveSubscriptionId := "sub_live_only"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "sub_live_only_order", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, ProviderCustomerId: "cus_live_only",
		ProviderSubscriptionId: &liveSubscriptionId, ProviderLivemode: true, CompleteTime: 100,
	}).Error)
	called := false
	createStripeBillingPortalSession = func(_ context.Context, _ *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
		called = true
		return nil, errors.New("must not be called")
	}

	recorder := invokeStripeBillingPortal(user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, called)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}

func TestCreateStripeBillingPortalSessionRejectsInvalidStripeResponse(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalCreate := createStripeBillingPortalSession
	t.Cleanup(func() { createStripeBillingPortalSession = originalCreate })

	user := &model.User{Username: "invalid-portal-user", Email: "invalid-portal@example.com"}
	require.NoError(t, db.Create(user).Error)
	testSubscriptionId := "sub_invalid"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "sub_invalid_order", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, ProviderCustomerId: "cus_invalid",
		ProviderSubscriptionId: &testSubscriptionId, ProviderLivemode: false, CompleteTime: 100,
	}).Error)
	createStripeBillingPortalSession = func(_ context.Context, _ *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
		return &stripe.BillingPortalSession{ID: "bps_invalid", Livemode: true}, nil
	}

	recorder := invokeStripeBillingPortal(user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}

func TestCreateStripeBillingPortalSessionRejectsInvalidReturnURL(t *testing.T) {
	db := setupStripeWebhookTest(t)
	originalCreate := createStripeBillingPortalSession
	originalAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		createStripeBillingPortalSession = originalCreate
		system_setting.ServerAddress = originalAddress
	})
	system_setting.ServerAddress = ""

	user := &model.User{Username: "invalid-return-user", Email: "invalid-return@example.com"}
	require.NoError(t, db.Create(user).Error)
	testSubscriptionId := "sub_invalid_return"
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: user.Id, TradeNo: "sub_invalid_return_order", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, ProviderCustomerId: "cus_invalid_return",
		ProviderSubscriptionId: &testSubscriptionId, ProviderLivemode: false, CompleteTime: 100,
	}).Error)
	called := false
	createStripeBillingPortalSession = func(_ context.Context, _ *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
		called = true
		return nil, errors.New("must not be called")
	}

	recorder := invokeStripeBillingPortal(user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, called)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}
