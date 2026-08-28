package controller

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionPlanAdminTest(t *testing.T) *gorm.DB {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	originalDB := model.DB
	originalGinMode := gin.Mode()
	var sqlDB *sql.DB
	t.Cleanup(func() {
		model.DB = originalDB
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
		&model.SubscriptionPlan{},
		&model.SubscriptionPlanLock{},
	))
	model.DB = db
	gin.SetMode(gin.TestMode)
	return db
}

func performSubscriptionPlanRequest(
	t *testing.T,
	method string,
	target string,
	currencyField string,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"plan":{"title":"Monthly plan","price_amount":399,%s"duration_unit":"day","duration_value":28,"enabled":true,"public_visible":true,"recommended":true,"total_amount":55000000,"quota_reset_period":"custom","quota_reset_custom_seconds":604800}}`, currencyField)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine := gin.New()
	routePath := target
	if method == http.MethodPut {
		routePath = "/plans/:id"
	}
	engine.Handle(method, routePath, handler)
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestAdminCreateSubscriptionPlanDefaultsToCNY(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)

	recorder := performSubscriptionPlanRequest(
		t,
		http.MethodPost,
		"/plans",
		"",
		AdminCreateSubscriptionPlan,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var plan model.SubscriptionPlan
	require.NoError(t, db.First(&plan).Error)
	require.Equal(t, "CNY", plan.Currency)
	require.Equal(t, model.SubscriptionDurationMonth, plan.DurationUnit)
	require.Equal(t, 1, plan.DurationValue)
	require.Equal(t, model.SubscriptionResetBillingCycle, plan.QuotaResetPeriod)
	require.Zero(t, plan.QuotaResetCustomSeconds)
	require.NotNil(t, plan.PublicVisible)
	assert.True(t, *plan.PublicVisible)
	assert.True(t, plan.Recommended)
}

func TestAdminCreateSubscriptionPlanKeepsOnlyLatestRecommendedPlan(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)

	first := performSubscriptionPlanRequest(
		t,
		http.MethodPost,
		"/plans",
		"",
		AdminCreateSubscriptionPlan,
	)
	require.Contains(t, first.Body.String(), `"success":true`)

	second := performSubscriptionPlanRequest(
		t,
		http.MethodPost,
		"/plans",
		"",
		AdminCreateSubscriptionPlan,
	)
	require.Contains(t, second.Body.String(), `"success":true`)

	var recommendedPlans []model.SubscriptionPlan
	require.NoError(t, db.Where("recommended = ?", true).Find(&recommendedPlans).Error)
	require.Len(t, recommendedPlans, 1)
	assert.Equal(t, 2, recommendedPlans[0].Id)
}

func TestAdminUpdateSubscriptionPlanPreservesCNY(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)
	plan := model.SubscriptionPlan{
		Title: "Existing plan", PriceAmount: 1, Currency: "CNY",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
	}
	require.NoError(t, db.Create(&plan).Error)

	target := fmt.Sprintf("/plans/%d", plan.Id)
	recorder := performSubscriptionPlanRequest(
		t,
		http.MethodPut,
		target,
		`"currency":"cny",`,
		AdminUpdateSubscriptionPlan,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NoError(t, db.First(&plan, plan.Id).Error)
	require.Equal(t, "CNY", plan.Currency)
	require.Equal(t, 1, plan.DurationValue)
	require.Equal(t, model.SubscriptionResetBillingCycle, plan.QuotaResetPeriod)
}

func TestAdminUpdateSubscriptionPlanDefaultsOmittedCurrencyToCNY(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)
	plan := model.SubscriptionPlan{
		Title: "Legacy USD plan", PriceAmount: 1, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
	}
	require.NoError(t, db.Create(&plan).Error)

	target := fmt.Sprintf("/plans/%d", plan.Id)
	recorder := performSubscriptionPlanRequest(
		t,
		http.MethodPut,
		target,
		"",
		AdminUpdateSubscriptionPlan,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NoError(t, db.First(&plan, plan.Id).Error)
	require.Equal(t, model.SubscriptionCurrencyCNY, plan.Currency)
}

func TestAdminCreateSubscriptionPlanRejectsNonCNYCurrency(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)

	recorder := performSubscriptionPlanRequest(
		t,
		http.MethodPost,
		"/plans",
		`"currency":"USD",`,
		AdminCreateSubscriptionPlan,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	var count int64
	require.NoError(t, db.Model(&model.SubscriptionPlan{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestGetSubscriptionPlansReturnsOnlyPublicCheckoutFields(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)
	plan := model.SubscriptionPlan{
		Title:                   "Public plan",
		Subtitle:                "Monthly quota",
		Recommended:             true,
		PriceAmount:             399,
		Currency:                model.SubscriptionCurrencyCNY,
		DurationUnit:            model.SubscriptionDurationMonth,
		DurationValue:           1,
		Enabled:                 true,
		SortOrder:               300,
		AllowWalletOverflow:     common.GetPointer(false),
		StripePriceId:           "price_internal",
		CreemProductId:          "creem_internal",
		WaffoPancakeProductId:   "waffo_internal",
		MaxPurchasePerUser:      1,
		UpgradeGroup:            "premium",
		DowngradeGroup:          "default",
		TotalAmount:             55000000,
		QuotaResetPeriod:        model.SubscriptionResetBillingCycle,
		QuotaResetCustomSeconds: 0,
	}
	require.NoError(t, db.Create(&plan).Error)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/plans", nil)
	engine := gin.New()
	engine.GET("/plans", GetSubscriptionPlans)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Plan map[string]any `json:"plan"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	publicPlan := response.Data[0].Plan
	assert.Equal(t, true, publicPlan["stripe_checkout_available"])
	assert.Equal(t, true, publicPlan["recommended"])
	assert.Equal(t, false, publicPlan["creem_checkout_available"])
	assert.Equal(t, false, publicPlan["waffo_checkout_available"])
	for _, internalField := range []string{
		"allow_balance_pay",
		"stripe_price_id",
		"creem_product_id",
		"waffo_pancake_product_id",
		"allow_wallet_overflow",
		"downgrade_group",
		"enabled",
		"public_visible",
		"sort_order",
		"created_at",
		"updated_at",
	} {
		assert.NotContains(t, publicPlan, internalField)
	}
}

func TestGetSubscriptionPlansFiltersDisabledAndHiddenPlans(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)
	for _, plan := range []model.SubscriptionPlan{
		{
			Title: "Visible plan", PriceAmount: 399, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetBillingCycle,
			StripePriceId:    "price_visible_plan", SortOrder: 20,
		},
		{
			Title: "Visible without Stripe", PriceAmount: 99, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetBillingCycle, SortOrder: 10,
		},
		{
			Title: "Hidden plan", PriceAmount: 399, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetBillingCycle,
			PublicVisible:    common.GetPointer(false), StripePriceId: "price_hidden",
		},
		{
			Title: "Disabled plan", PriceAmount: 399, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: false,
			QuotaResetPeriod: model.SubscriptionResetBillingCycle,
			StripePriceId:    "price_disabled",
		},
		{
			Title: "USD plan", PriceAmount: 399, Currency: "USD",
			DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetBillingCycle,
			StripePriceId:    "price_usd",
		},
	} {
		require.NoError(t, db.Create(&plan).Error)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/plans", nil)
	engine := gin.New()
	engine.GET("/plans", GetSubscriptionPlans)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Plan PublicSubscriptionPlan `json:"plan"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 2)
	assert.Equal(t, "Visible plan", response.Data[0].Plan.Title)
	assert.Equal(t, model.SubscriptionCurrencyCNY, response.Data[0].Plan.Currency)
	assert.True(t, response.Data[0].Plan.StripeCheckoutAvailable)
	assert.Equal(t, "Visible without Stripe", response.Data[1].Plan.Title)
	assert.False(t, response.Data[1].Plan.StripeCheckoutAvailable)
}
