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
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}))
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
	body := fmt.Sprintf(`{"plan":{"title":"Four week plan","price_amount":399,%s"duration_unit":"day","duration_value":28,"enabled":true,"total_amount":55000000,"quota_reset_period":"custom","quota_reset_custom_seconds":604800}}`, currencyField)
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
	require.Equal(t, model.SubscriptionDurationDay, plan.DurationUnit)
	require.Equal(t, 28, plan.DurationValue)
	require.Equal(t, model.SubscriptionResetCustom, plan.QuotaResetPeriod)
	require.Equal(t, int64(604800), plan.QuotaResetCustomSeconds)
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
	require.Equal(t, 28, plan.DurationValue)
}

func TestAdminUpdateSubscriptionPlanPreservesExistingCurrencyWhenCurrencyOmitted(t *testing.T) {
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
	require.Equal(t, "USD", plan.Currency)
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
		Subtitle:                "Four weeks",
		PriceAmount:             399,
		Currency:                model.SubscriptionCurrencyCNY,
		DurationUnit:            model.SubscriptionDurationDay,
		DurationValue:           28,
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
		QuotaResetPeriod:        model.SubscriptionResetCustom,
		QuotaResetCustomSeconds: 604800,
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
		"sort_order",
		"created_at",
		"updated_at",
	} {
		assert.NotContains(t, publicPlan, internalField)
	}
}

func TestGetSubscriptionPlansFiltersIncompatiblePlans(t *testing.T) {
	db := setupSubscriptionPlanAdminTest(t)
	for _, plan := range []model.SubscriptionPlan{
		{
			Title: "CNY plan", PriceAmount: 399, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationDay, DurationValue: 28, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetCustom, QuotaResetCustomSeconds: 604800,
			StripePriceId: "price_cny_plan",
		},
		{
			Title: "Legacy USD plan", PriceAmount: 99, Currency: "USD",
			DurationUnit: model.SubscriptionDurationDay, DurationValue: 28, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetCustom, QuotaResetCustomSeconds: 604800,
			StripePriceId: "price_legacy_usd",
		},
		{
			Title: "Thirty day plan", PriceAmount: 399, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationDay, DurationValue: 30, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetCustom, QuotaResetCustomSeconds: 604800,
			StripePriceId: "price_thirty_days",
		},
		{
			Title: "Daily quota plan", PriceAmount: 399, Currency: model.SubscriptionCurrencyCNY,
			DurationUnit: model.SubscriptionDurationDay, DurationValue: 28, Enabled: true,
			QuotaResetPeriod: model.SubscriptionResetCustom, QuotaResetCustomSeconds: 86400,
			StripePriceId: "price_daily_quota",
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
	require.Len(t, response.Data, 1)
	assert.Equal(t, "CNY plan", response.Data[0].Plan.Title)
	assert.Equal(t, model.SubscriptionCurrencyCNY, response.Data[0].Plan.Currency)
}
