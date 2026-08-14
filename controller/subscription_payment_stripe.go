package controller

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v86"
	"github.com/thanhpk/randstr"
)

var retrieveStripePrice = func(ctx context.Context, priceId string) (*stripe.Price, error) {
	client := stripe.NewClient(setting.StripeApiSecret)
	return client.V1Prices.Retrieve(ctx, priceId, &stripe.PriceRetrieveParams{})
}

type SubscriptionStripePayRequest struct {
	PlanId int `json:"plan_id"`
}

func SubscriptionRequestStripePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req SubscriptionStripePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.StripePriceId == "" {
		common.ApiErrorMsg(c, "该套餐未配置 StripePriceId")
		return
	}
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}
	if setting.StripeWebhookSecret == "" {
		common.ApiErrorMsg(c, "Stripe Webhook 未配置")
		return
	}
	expectedAmountMinor, err := stripeSubscriptionAmountMinor(plan.PriceAmount)
	expectedCurrency := strings.ToUpper(strings.TrimSpace(plan.Currency))
	if err != nil || expectedCurrency == "" {
		common.ApiErrorMsg(c, "套餐价格配置无效")
		return
	}
	stripePrice, err := retrieveStripePrice(c.Request.Context(), plan.StripePriceId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 获取订阅 Price 失败 plan_id=%d price_id=%s error=%q", plan.Id, plan.StripePriceId, err.Error()))
		common.ApiErrorMsg(c, "Stripe 套餐价格校验失败")
		return
	}
	if err := validateStripeSubscriptionPrice(plan, stripePrice, expectedAmountMinor, expectedCurrency, stripeLivemodeForSecret(setting.StripeApiSecret)); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 订阅 Price 与本地套餐不匹配 plan_id=%d price_id=%s error=%q", plan.Id, plan.StripePriceId, err.Error()))
		common.ApiErrorMsg(c, "Stripe 套餐价格配置与本地套餐不匹配")
		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}

	reference := fmt.Sprintf("sub-stripe-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "sub_ref_" + common.Sha1([]byte(reference))
	allowWalletOverflow := true
	if plan.AllowWalletOverflow != nil {
		allowWalletOverflow = *plan.AllowWalletOverflow
	}

	order := &model.SubscriptionOrder{
		UserId:                  userId,
		PlanId:                  plan.Id,
		Money:                   plan.PriceAmount,
		TradeNo:                 referenceId,
		PaymentMethod:           model.PaymentMethodStripe,
		PaymentProvider:         model.PaymentProviderStripe,
		ProviderProductId:       plan.StripePriceId,
		ExpectedAmountMinor:     expectedAmountMinor,
		ExpectedCurrency:        expectedCurrency,
		PlanTitle:               plan.Title,
		PlanDurationUnit:        plan.DurationUnit,
		PlanDurationValue:       plan.DurationValue,
		PlanCustomSeconds:       plan.CustomSeconds,
		PlanTotalAmount:         plan.TotalAmount,
		PlanResetPeriod:         model.NormalizeResetPeriod(plan.QuotaResetPeriod),
		PlanResetCustomSeconds:  plan.QuotaResetCustomSeconds,
		PlanUpgradeGroup:        strings.TrimSpace(plan.UpgradeGroup),
		PlanDowngradeGroup:      strings.TrimSpace(plan.DowngradeGroup),
		PlanAllowWalletOverflow: allowWalletOverflow,
		CreateTime:              time.Now().Unix(),
		Status:                  common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	checkoutSession, err := genStripeSubscriptionLink(c.Request.Context(), referenceId, user.StripeCustomer, user.Email, plan.StripePriceId)
	if err != nil {
		_ = model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 订阅支付链接创建失败 trade_no=%s plan_id=%d error=%q", referenceId, plan.Id, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if checkoutSession.AmountTotal != expectedAmountMinor ||
		!strings.EqualFold(string(checkoutSession.Currency), order.ExpectedCurrency) ||
		checkoutSession.Livemode != stripeLivemodeForSecret(setting.StripeApiSecret) {
		_ = model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe Checkout Session 响应与订阅订单不匹配 trade_no=%s plan_id=%d session_id=%s", referenceId, plan.Id, checkoutSession.ID))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	if err := order.BindStripeCheckout(model.StripeSubscriptionCheckoutBinding{
		CheckoutSessionId: checkoutSession.ID,
		PriceId:           plan.StripePriceId,
		AmountMinor:       checkoutSession.AmountTotal,
		Currency:          string(checkoutSession.Currency),
		Livemode:          checkoutSession.Livemode,
	}); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 保存订阅 Checkout Session 失败 trade_no=%s plan_id=%d session_id=%s error=%q", referenceId, plan.Id, checkoutSession.ID, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": checkoutSession.URL,
		},
	})
}

func stripeSubscriptionAmountMinor(amount float64) (int64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, fmt.Errorf("invalid Stripe subscription amount")
	}
	minor := decimal.NewFromFloat(amount).Mul(decimal.NewFromInt(100)).Round(0)
	if !minor.IsPositive() || minor.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, fmt.Errorf("Stripe subscription amount is out of range")
	}
	return minor.IntPart(), nil
}

func validateStripeSubscriptionPrice(plan *model.SubscriptionPlan, stripePrice *stripe.Price, expectedAmountMinor int64, expectedCurrency string, expectedLivemode bool) error {
	if plan == nil || stripePrice == nil || stripePrice.ID == "" || stripePrice.Deleted {
		return fmt.Errorf("Stripe Price is missing or deleted")
	}
	if stripePrice.ID != strings.TrimSpace(plan.StripePriceId) {
		return fmt.Errorf("Stripe Price ID does not match the plan")
	}
	if !stripePrice.Active || stripePrice.Type != stripe.PriceTypeRecurring || stripePrice.Recurring == nil {
		return fmt.Errorf("Stripe Price must be an active recurring Price")
	}
	if stripePrice.BillingScheme != stripe.PriceBillingSchemePerUnit ||
		stripePrice.Recurring.UsageType != stripe.PriceRecurringUsageTypeLicensed ||
		stripePrice.TransformQuantity != nil {
		return fmt.Errorf("Stripe Price must use fixed per-unit licensed billing")
	}
	if stripePrice.UnitAmount != expectedAmountMinor ||
		!strings.EqualFold(string(stripePrice.Currency), expectedCurrency) ||
		stripePrice.Livemode != expectedLivemode {
		return fmt.Errorf("Stripe Price amount, currency, or livemode does not match the plan")
	}

	intervalCount := stripePrice.Recurring.IntervalCount
	if plan.DurationValue <= 0 || intervalCount <= 0 {
		return fmt.Errorf("subscription interval is invalid")
	}
	intervalMatches := false
	switch plan.DurationUnit {
	case model.SubscriptionDurationYear:
		intervalMatches = stripePrice.Recurring.Interval == stripe.PriceRecurringIntervalYear && intervalCount == int64(plan.DurationValue)
	case model.SubscriptionDurationMonth:
		intervalMatches = stripePrice.Recurring.Interval == stripe.PriceRecurringIntervalMonth && intervalCount == int64(plan.DurationValue)
	case model.SubscriptionDurationDay:
		intervalMatches = stripePrice.Recurring.Interval == stripe.PriceRecurringIntervalDay && intervalCount == int64(plan.DurationValue)
		if plan.DurationValue%7 == 0 && stripePrice.Recurring.Interval == stripe.PriceRecurringIntervalWeek {
			intervalMatches = intervalCount == int64(plan.DurationValue/7)
		}
	}
	if !intervalMatches {
		return fmt.Errorf("Stripe Price recurrence does not match the plan duration")
	}
	return nil
}

func genStripeSubscriptionLink(ctx context.Context, referenceId string, customerId string, email string, priceId string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionCreateParams{
		Params:            stripe.Params{Context: ctx},
		ClientReferenceID: stripe.String(referenceId),
		IntegrationIdentifier: stripe.String(
			"tryvalo_subscription_" + randstr.String(8, "abcdefghijklmnopqrstuvwxyz"),
		),
		SuccessURL: stripe.String(paymentReturnPath("/wallet")),
		CancelURL:  stripe.String(paymentReturnPath("/wallet")),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(priceId),
				Quantity: stripe.Int64(1),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Metadata: map[string]string{
			"trade_no":   referenceId,
			"order_kind": "subscription",
			"price_id":   priceId,
		},
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{
			Metadata: map[string]string{
				"trade_no":   referenceId,
				"order_kind": "subscription",
				"price_id":   priceId,
			},
		},
	}
	params.SetIdempotencyKey("checkout-" + referenceId)

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := createStripeCheckoutSession(params)
	if err != nil {
		return nil, err
	}
	if result == nil || result.ID == "" || result.URL == "" {
		return nil, fmt.Errorf("Stripe Checkout Session 响应不完整")
	}
	return result, nil
}
