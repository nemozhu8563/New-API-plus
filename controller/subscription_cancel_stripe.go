package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v86"
)

type StripeSubscriptionCancelRequest struct {
	SubscriptionId string `json:"subscription_id"`
}

var updateStripeSubscription = func(ctx context.Context, subscriptionId string, params *stripe.SubscriptionUpdateParams) (*stripe.Subscription, error) {
	client := stripe.NewClient(setting.StripeApiSecret)
	return client.V1Subscriptions.Update(ctx, subscriptionId, params)
}

func stripeSubscriptionCurrentPeriodEnd(subscription *stripe.Subscription) int64 {
	if subscription == nil || subscription.Items == nil {
		return 0
	}
	currentPeriodEnd := int64(0)
	for _, item := range subscription.Items.Data {
		if item == nil || item.Deleted || item.CurrentPeriodEnd <= 0 {
			continue
		}
		if currentPeriodEnd == 0 || item.CurrentPeriodEnd < currentPeriodEnd {
			currentPeriodEnd = item.CurrentPeriodEnd
		}
	}
	return currentPeriodEnd
}

func CancelStripeSubscription(c *gin.Context) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}
	var request StripeSubscriptionCancelRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.SubscriptionId) == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	request.SubscriptionId = strings.TrimSpace(request.SubscriptionId)
	userId := c.GetInt("id")
	livemode := stripeLivemodeForSecret(setting.StripeApiSecret)
	order, err := model.GetStripeSubscriptionOrderForUser(userId, request.SubscriptionId, livemode)
	if err != nil || strings.TrimSpace(order.ProviderCustomerId) == "" {
		common.ApiErrorMsg(c, "当前账户没有可取消的 Stripe 订阅")
		return
	}

	params := &stripe.SubscriptionUpdateParams{
		Params:            stripe.Params{Context: c.Request.Context()},
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	subscription, err := updateStripeSubscription(c.Request.Context(), request.SubscriptionId, params)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf(
			"Stripe 设置周期末取消失败 user_id=%d subscription_id=%s error=%q",
			userId, request.SubscriptionId, err.Error(),
		))
		common.ApiErrorMsg(c, "无法取消 Stripe 订阅，请稍后重试")
		return
	}
	currentPeriodEnd := stripeSubscriptionCurrentPeriodEnd(subscription)
	if subscription == nil || subscription.ID != request.SubscriptionId || subscription.Customer == nil ||
		subscription.Customer.ID != order.ProviderCustomerId || subscription.Livemode != livemode ||
		!subscription.CancelAtPeriodEnd || strings.TrimSpace(string(subscription.Status)) == "" || currentPeriodEnd <= 0 {
		logger.LogError(c.Request.Context(), fmt.Sprintf(
			"Stripe 周期末取消响应无效 user_id=%d subscription_id=%s",
			userId, request.SubscriptionId,
		))
		common.ApiErrorMsg(c, "无法确认 Stripe 订阅取消状态")
		return
	}
	if err := model.MarkStripeSubscriptionCancellationRequested(
		userId,
		subscription.ID,
		subscription.Customer.ID,
		subscription.Livemode,
		string(subscription.Status),
		subscription.CancelAt,
		currentPeriodEnd,
	); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf(
			"Stripe 订阅取消状态落库失败 user_id=%d subscription_id=%s error=%q",
			userId, request.SubscriptionId, err.Error(),
		))
		common.ApiErrorMsg(c, "Stripe 已收到取消请求，但本地状态同步失败，请联系支持")
		return
	}

	common.ApiSuccess(c, model.StripeSubscriptionSummary{
		SubscriptionId:    subscription.ID,
		CustomerId:        subscription.Customer.ID,
		PlanId:            order.PlanId,
		PlanTitle:         order.PlanTitle,
		Status:            string(subscription.Status),
		CancelAtPeriodEnd: true,
		CancelAt:          subscription.CancelAt,
		CurrentPeriodEnd:  currentPeriodEnd,
		Livemode:          subscription.Livemode,
	})
}
