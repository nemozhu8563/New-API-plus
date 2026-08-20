package controller

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v86"
)

var createStripeBillingPortalSession = func(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	client := stripe.NewClient(setting.StripeApiSecret)
	return client.V1BillingPortalSessions.Create(ctx, params)
}

func CreateStripeBillingPortalSession(c *gin.Context) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}

	userId := c.GetInt("id")
	livemode := stripeLivemodeForSecret(setting.StripeApiSecret)
	customerId, err := model.GetLatestStripeCustomerIdForUser(userId, livemode)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if customerId == "" {
		common.ApiErrorMsg(c, "当前账户没有可管理的 Stripe 订阅")
		return
	}
	returnURL := paymentReturnPath("/wallet")
	parsedReturnURL, err := url.Parse(returnURL)
	if err != nil || parsedReturnURL.Host == "" || parsedReturnURL.Scheme != "https" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe Customer Portal 返回地址无效 user_id=%d return_url=%q", userId, returnURL))
		common.ApiErrorMsg(c, "无法打开 Stripe 账单管理页面")
		return
	}

	params := &stripe.BillingPortalSessionCreateParams{
		Params:    stripe.Params{Context: c.Request.Context()},
		Customer:  stripe.String(customerId),
		ReturnURL: stripe.String(returnURL),
	}
	portalSession, err := createStripeBillingPortalSession(c.Request.Context(), params)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建 Customer Portal Session 失败 user_id=%d customer_id=%s error=%q", userId, customerId, err.Error()))
		common.ApiErrorMsg(c, "无法打开 Stripe 账单管理页面")
		return
	}
	if portalSession == nil || portalSession.URL == "" || portalSession.Livemode != livemode {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe Customer Portal Session 响应无效 user_id=%d customer_id=%s", userId, customerId))
		common.ApiErrorMsg(c, "无法打开 Stripe 账单管理页面")
		return
	}

	common.ApiSuccess(c, gin.H{"portal_url": portalSession.URL})
}
