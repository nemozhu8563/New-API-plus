package controller

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
	"gorm.io/gorm"
)

func TestGenStripeLinkUsesContextIdempotencyAndMetadata(t *testing.T) {
	originalSecret := setting.StripeApiSecret
	originalPriceID := setting.StripePriceId
	setting.StripeApiSecret = "rk_test_placeholder"
	setting.StripePriceId = "price_local_topup"
	originalTrustedDomains := constant.TrustedRedirectDomains
	constant.TrustedRedirectDomains = []string{"example.test"}
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripePriceId = originalPriceID
		constant.TrustedRedirectDomains = originalTrustedDomains
	})

	originalCreate := createStripeCheckoutSession
	var captured *stripe.CheckoutSessionCreateParams
	createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		captured = params
		return &stripe.CheckoutSession{ID: "cs_local_topup", URL: "https://checkout.stripe.test/topup"}, nil
	}
	t.Cleanup(func() { createStripeCheckoutSession = originalCreate })
	ctx := context.WithValue(context.Background(), struct{}{}, "request")

	result, err := genStripeLink(ctx, "ref_local_topup", "", "user@example.test", 3, "https://example.test/success", "https://example.test/cancel")

	require.NoError(t, err)
	assert.Equal(t, "cs_local_topup", result.ID)
	require.NotNil(t, captured)
	assert.Same(t, ctx, captured.Context)
	require.NotNil(t, captured.IdempotencyKey)
	assert.Equal(t, "checkout-ref_local_topup", *captured.IdempotencyKey)
	require.NotNil(t, captured.IntegrationIdentifier)
	assert.Regexp(t, `^tryvalo_topup_[a-z]{8}$`, *captured.IntegrationIdentifier)
	assert.Equal(t, "topup", captured.Metadata["order_kind"])
	assert.Equal(t, "price_local_topup", captured.Metadata["price_id"])
	require.NotNil(t, captured.PaymentIntentData)
	assert.Equal(t, captured.Metadata, captured.PaymentIntentData.Metadata)
	require.Len(t, captured.Expand, 1)
	assert.Equal(t, "payment_intent.latest_charge", *captured.Expand[0])
	assert.Equal(t, string(stripe.CheckoutSessionModePayment), *captured.Mode)
}

func TestGenStripeLinkRejectsUntrustedRedirectURLsBeforeStripeRequest(t *testing.T) {
	originalSecret := setting.StripeApiSecret
	setting.StripeApiSecret = "rk_test_placeholder"
	t.Cleanup(func() { setting.StripeApiSecret = originalSecret })

	originalCreate := createStripeCheckoutSession
	createCalled := false
	createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		createCalled = true
		return nil, errors.New("should not create Checkout")
	}
	t.Cleanup(func() { createStripeCheckoutSession = originalCreate })

	_, err := genStripeLink(
		context.Background(),
		"ref_untrusted_redirect",
		"",
		"user@example.test",
		1,
		"javascript:alert(1)",
		"https://example.test/cancel",
	)

	require.Error(t, err)
	assert.False(t, createCalled)
}

func TestGenStripeSubscriptionLinkUsesContextIdempotencyAndMetadata(t *testing.T) {
	originalCreate := createStripeCheckoutSession
	var captured *stripe.CheckoutSessionCreateParams
	createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		captured = params
		return &stripe.CheckoutSession{ID: "cs_local_subscription", URL: "https://checkout.stripe.test/subscription"}, nil
	}
	t.Cleanup(func() { createStripeCheckoutSession = originalCreate })
	ctx := context.WithValue(context.Background(), struct{}{}, "request")

	result, err := genStripeSubscriptionLink(ctx, "sub_ref_local", "cus_local", "", "price_local_subscription")

	require.NoError(t, err)
	assert.Equal(t, "cs_local_subscription", result.ID)
	require.NotNil(t, captured)
	assert.Same(t, ctx, captured.Context)
	require.NotNil(t, captured.IdempotencyKey)
	assert.Equal(t, "checkout-sub_ref_local", *captured.IdempotencyKey)
	require.NotNil(t, captured.IntegrationIdentifier)
	assert.Regexp(t, `^tryvalo_subscription_[a-z]{8}$`, *captured.IntegrationIdentifier)
	assert.Equal(t, "subscription", captured.Metadata["order_kind"])
	assert.Equal(t, "price_local_subscription", captured.Metadata["price_id"])
	require.NotNil(t, captured.SubscriptionData)
	assert.Equal(t, captured.Metadata, captured.SubscriptionData.Metadata)
	assert.Equal(t, string(stripe.CheckoutSessionModeSubscription), *captured.Mode)
}

func TestGenStripeSubscriptionLinkLetsSubscriptionModeCreateCustomer(t *testing.T) {
	originalCreate := createStripeCheckoutSession
	var captured *stripe.CheckoutSessionCreateParams
	createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		captured = params
		return &stripe.CheckoutSession{ID: "cs_local_new_customer", URL: "https://checkout.stripe.test/new-customer"}, nil
	}
	t.Cleanup(func() { createStripeCheckoutSession = originalCreate })

	_, err := genStripeSubscriptionLink(context.Background(), "sub_ref_new_customer", "", "user@example.test", "price_local_subscription")

	require.NoError(t, err)
	require.NotNil(t, captured)
	require.NotNil(t, captured.CustomerEmail)
	assert.Equal(t, "user@example.test", *captured.CustomerEmail)
	assert.Nil(t, captured.Customer)
	assert.Nil(t, captured.CustomerCreation)
}

func TestValidateStripeSubscriptionPriceRequiresExactFixedRecurringContract(t *testing.T) {
	plan := &model.SubscriptionPlan{
		StripePriceId: "price_local_subscription", PriceAmount: 12, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
	}
	validPrice := &stripe.Price{
		ID: plan.StripePriceId, Active: true, Type: stripe.PriceTypeRecurring,
		BillingScheme: stripe.PriceBillingSchemePerUnit, Currency: stripe.CurrencyUSD, UnitAmount: 1200,
		Recurring: &stripe.PriceRecurring{
			Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1,
			UsageType: stripe.PriceRecurringUsageTypeLicensed,
		},
	}

	require.NoError(t, validateStripeSubscriptionPrice(plan, validPrice, 1200, "USD", false))

	testCases := []struct {
		name   string
		mutate func(*stripe.Price)
	}{
		{name: "inactive", mutate: func(price *stripe.Price) { price.Active = false }},
		{name: "one time", mutate: func(price *stripe.Price) { price.Type = stripe.PriceTypeOneTime }},
		{name: "metered", mutate: func(price *stripe.Price) { price.Recurring.UsageType = stripe.PriceRecurringUsageTypeMetered }},
		{name: "tiered", mutate: func(price *stripe.Price) { price.BillingScheme = stripe.PriceBillingSchemeTiered }},
		{name: "amount", mutate: func(price *stripe.Price) { price.UnitAmount++ }},
		{name: "currency", mutate: func(price *stripe.Price) { price.Currency = stripe.CurrencyEUR }},
		{name: "interval", mutate: func(price *stripe.Price) { price.Recurring.Interval = stripe.PriceRecurringIntervalYear }},
		{name: "interval count", mutate: func(price *stripe.Price) { price.Recurring.IntervalCount = 2 }},
		{name: "livemode", mutate: func(price *stripe.Price) { price.Livemode = true }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			priceCopy := *validPrice
			recurringCopy := *validPrice.Recurring
			priceCopy.Recurring = &recurringCopy
			testCase.mutate(&priceCopy)
			require.Error(t, validateStripeSubscriptionPrice(plan, &priceCopy, 1200, "USD", false))
		})
	}
}

func TestValidateStripeSubscriptionPriceAcceptsEquivalentWeeklyDuration(t *testing.T) {
	plan := &model.SubscriptionPlan{
		StripePriceId: "price_two_weeks", PriceAmount: 12, Currency: "USD",
		DurationUnit: model.SubscriptionDurationDay, DurationValue: 14,
	}
	stripePrice := &stripe.Price{
		ID: plan.StripePriceId, Active: true, Type: stripe.PriceTypeRecurring,
		BillingScheme: stripe.PriceBillingSchemePerUnit, Currency: stripe.CurrencyUSD, UnitAmount: 1200,
		Recurring: &stripe.PriceRecurring{
			Interval: stripe.PriceRecurringIntervalWeek, IntervalCount: 2,
			UsageType: stripe.PriceRecurringUsageTypeLicensed,
		},
	}

	require.NoError(t, validateStripeSubscriptionPrice(plan, stripePrice, 1200, "USD", false))
}

func setupStripeCheckoutHandlerTest(t *testing.T) *gorm.DB {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedis := common.RedisEnabled
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	originalMinTopUp := setting.StripeMinTopUp
	originalUnitPrice := setting.StripeUnitPrice
	originalCreate := createStripeCheckoutSession
	originalRetrievePrice := retrieveStripePrice
	originalGinMode := gin.Mode()
	var sqlDB *sql.DB
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedis
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
		setting.StripeMinTopUp = originalMinTopUp
		setting.StripeUnitPrice = originalUnitPrice
		createStripeCheckoutSession = originalCreate
		retrieveStripePrice = originalRetrievePrice
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
	setting.StripeWebhookSecret = "whsec_placeholder"
	setting.StripePriceId = "price_topup_placeholder"
	setting.StripeMinTopUp = 1
	setting.StripeUnitPrice = 1
	retrieveStripePrice = func(_ context.Context, priceId string) (*stripe.Price, error) {
		return &stripe.Price{
			ID: priceId, Active: true, Type: stripe.PriceTypeRecurring,
			BillingScheme: stripe.PriceBillingSchemePerUnit, Currency: stripe.CurrencyUSD, UnitAmount: 1200,
			Recurring: &stripe.PriceRecurring{
				Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1,
				UsageType: stripe.PriceRecurringUsageTypeLicensed,
			},
		}, nil
	}
	gin.SetMode(gin.TestMode)
	return db
}

func invokeStripeCheckoutHandler(t *testing.T, handler gin.HandlerFunc, userID int, body string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/stripe/pay", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userID)
	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := map[string]any{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestStripeTopUpEndpointsRequirePaymentCompliance(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1006, Username: "stripe_topup_compliance", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""

	testCases := []struct {
		name    string
		handler gin.HandlerFunc
		body    string
	}{
		{name: "amount quote", handler: RequestStripeAmount, body: `{"amount":1}`},
		{name: "checkout", handler: RequestStripePay, body: `{"amount":1,"payment_method":"stripe"}`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := invokeStripeCheckoutHandler(t, testCase.handler, user.Id, testCase.body)
			assert.Equal(t, false, response["success"])
			assert.Equal(t, "payment.compliance_required", response["message"])
		})
	}

	var topUps int64
	require.NoError(t, db.Model(&model.TopUp{}).Count(&topUps).Error)
	assert.Zero(t, topUps)
}

func TestStripeTopUpEndpointsRequireCompleteConfiguration(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1007, Username: "stripe_topup_configuration", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	setting.StripeWebhookSecret = ""

	testCases := []struct {
		name    string
		handler gin.HandlerFunc
		body    string
	}{
		{name: "amount quote", handler: RequestStripeAmount, body: `{"amount":1}`},
		{name: "checkout", handler: RequestStripePay, body: `{"amount":1,"payment_method":"stripe"}`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := invokeStripeCheckoutHandler(t, testCase.handler, user.Id, testCase.body)
			assert.Equal(t, false, response["success"])
			assert.Equal(t, "Stripe 充值未配置", response["message"])
		})
	}

	var topUps int64
	require.NoError(t, db.Model(&model.TopUp{}).Count(&topUps).Error)
	assert.Zero(t, topUps)
}

func TestStripeSubscriptionEndpointRequiresPaymentCompliance(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	plan := &model.SubscriptionPlan{
		Title: "Stripe compliance plan", PriceAmount: 12, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		Enabled: true, StripePriceId: "price_subscription_placeholder",
	}
	require.NoError(t, db.Create(plan).Error)
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""

	response := invokeStripeCheckoutHandler(t, SubscriptionRequestStripePay, 1009, `{"plan_id":1}`)

	assert.Equal(t, false, response["success"])
	assert.Equal(t, "payment.compliance_required", response["message"])
	var orders int64
	require.NoError(t, db.Model(&model.SubscriptionOrder{}).Count(&orders).Error)
	assert.Zero(t, orders)
}

func TestStripeSubscriptionEndpointRequiresCompleteConfiguration(t *testing.T) {
	testCases := []struct {
		name            string
		apiSecret       string
		webhookSecret   string
		expectedMessage string
	}{
		{name: "API secret", apiSecret: "", webhookSecret: "whsec_placeholder", expectedMessage: "Stripe 未配置或密钥无效"},
		{name: "webhook secret", apiSecret: "rk_test_placeholder", webhookSecret: "", expectedMessage: "Stripe Webhook 未配置"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeCheckoutHandlerTest(t)
			plan := &model.SubscriptionPlan{
				Title: "Stripe configuration plan", PriceAmount: 12, Currency: "USD",
				DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
				Enabled: true, StripePriceId: "price_subscription_placeholder",
			}
			require.NoError(t, db.Create(plan).Error)
			setting.StripeApiSecret = testCase.apiSecret
			setting.StripeWebhookSecret = testCase.webhookSecret

			response := invokeStripeCheckoutHandler(t, SubscriptionRequestStripePay, 1010, `{"plan_id":1}`)

			assert.Equal(t, testCase.expectedMessage, response["message"])
			var orders int64
			require.NoError(t, db.Model(&model.SubscriptionOrder{}).Count(&orders).Error)
			assert.Zero(t, orders)
		})
	}
}

func TestStripeAmountQuoteRejectsTopUpAboveMaximum(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1008, Username: "stripe_topup_maximum", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	response := invokeStripeCheckoutHandler(t, RequestStripeAmount, user.Id, `{"amount":10001}`)

	assert.Equal(t, "充值数量不能大于 10000", response["data"])
}

func TestStripeTopUpCheckoutFailureLeavesFailedLocalOrder(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1001, Username: "stripe_topup_checkout", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	createStripeCheckoutSession = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		return nil, errors.New("local checkout failure")
	}

	response := invokeStripeCheckoutHandler(t, RequestStripePay, user.Id, `{"amount":1,"payment_method":"stripe"}`)

	assert.Equal(t, "error", response["message"])
	var topUps []model.TopUp
	require.NoError(t, db.Find(&topUps).Error)
	require.Len(t, topUps, 1)
	assert.Equal(t, common.TopUpStatusFailed, topUps[0].Status)
	assert.Empty(t, topUps[0].ProviderOrderId)
}

func TestStripeTopUpCheckoutSuccessBindsImmutableSnapshot(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1003, Username: "stripe_topup_success", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	createStripeCheckoutSession = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID: "cs_topup_success", URL: "https://checkout.stripe.test/topup-success",
			AmountTotal: 100, Currency: stripe.CurrencyUSD, Customer: &stripe.Customer{ID: "cus_topup_success"},
		}, nil
	}

	response := invokeStripeCheckoutHandler(t, RequestStripePay, user.Id, `{"amount":1,"payment_method":"stripe"}`)

	assert.Equal(t, "success", response["message"])
	var topUp model.TopUp
	require.NoError(t, db.First(&topUp).Error)
	assert.Equal(t, "cs_topup_success", topUp.ProviderOrderId)
	assert.Equal(t, "cus_topup_success", topUp.ProviderCustomerId)
	assert.Equal(t, int64(100), topUp.ExpectedAmountMinor)
	assert.Equal(t, "USD", topUp.ExpectedCurrency)
	assert.False(t, topUp.ProviderLivemode)
}

func TestStripeSubscriptionCheckoutFailureLeavesExpiredLocalOrder(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1002, Username: "stripe_subscription_checkout", Email: "user@example.test", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:         "Stripe checkout plan",
		PriceAmount:   12,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		StripePriceId: "price_subscription_placeholder",
	}
	require.NoError(t, db.Create(plan).Error)
	createStripeCheckoutSession = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		return nil, errors.New("local checkout failure")
	}

	response := invokeStripeCheckoutHandler(t, SubscriptionRequestStripePay, user.Id, `{"plan_id":1}`)

	assert.Equal(t, "error", response["message"])
	var orders []model.SubscriptionOrder
	require.NoError(t, db.Find(&orders).Error)
	require.Len(t, orders, 1)
	assert.Equal(t, common.TopUpStatusExpired, orders[0].Status)
	assert.Empty(t, orders[0].ProviderOrderId)
}

func TestStripeSubscriptionCheckoutRejectsMismatchedPriceBeforeCreatingOrder(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1005, Username: "stripe_subscription_price_mismatch", Email: "user@example.test", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title: "Stripe mismatched plan", PriceAmount: 12, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		Enabled: true, StripePriceId: "price_subscription_placeholder",
	}
	require.NoError(t, db.Create(plan).Error)
	retrieveStripePrice = func(_ context.Context, priceId string) (*stripe.Price, error) {
		return &stripe.Price{
			ID: priceId, Active: true, Type: stripe.PriceTypeRecurring,
			BillingScheme: stripe.PriceBillingSchemePerUnit, Currency: stripe.CurrencyUSD, UnitAmount: 1300,
			Recurring: &stripe.PriceRecurring{
				Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1,
				UsageType: stripe.PriceRecurringUsageTypeLicensed,
			},
		}, nil
	}
	createCalled := false
	createStripeCheckoutSession = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		createCalled = true
		return nil, errors.New("should not create Checkout")
	}

	response := invokeStripeCheckoutHandler(t, SubscriptionRequestStripePay, user.Id, `{"plan_id":1}`)

	assert.Equal(t, "Stripe 套餐价格配置与本地套餐不匹配", response["message"])
	assert.False(t, createCalled)
	var orders int64
	require.NoError(t, db.Model(&model.SubscriptionOrder{}).Count(&orders).Error)
	assert.Zero(t, orders)
}

func TestStripeSubscriptionCheckoutSuccessBindsImmutableSnapshot(t *testing.T) {
	db := setupStripeCheckoutHandlerTest(t)
	user := &model.User{Id: 1004, Username: "stripe_subscription_success", Email: "user@example.test", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title: "Stripe checkout plan", PriceAmount: 12, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		Enabled: true, StripePriceId: "price_subscription_placeholder",
	}
	require.NoError(t, db.Create(plan).Error)
	createStripeCheckoutSession = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID: "cs_subscription_success", URL: "https://checkout.stripe.test/subscription-success",
			AmountTotal: 1200, Currency: stripe.CurrencyUSD,
		}, nil
	}

	response := invokeStripeCheckoutHandler(t, SubscriptionRequestStripePay, user.Id, `{"plan_id":1}`)

	assert.Equal(t, "success", response["message"])
	var order model.SubscriptionOrder
	require.NoError(t, db.First(&order).Error)
	assert.Equal(t, "cs_subscription_success", order.ProviderOrderId)
	assert.Equal(t, plan.StripePriceId, order.ProviderProductId)
	assert.Equal(t, int64(1200), order.ExpectedAmountMinor)
	assert.Equal(t, "USD", order.ExpectedCurrency)
	assert.False(t, order.ProviderLivemode)
}
