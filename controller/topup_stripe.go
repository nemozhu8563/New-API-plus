package controller

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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
	"github.com/stripe/stripe-go/v86/webhook"
	"github.com/thanhpk/randstr"
)

var stripeAdaptor = &StripeAdaptor{}

const stripeWebhookLeaseDuration = time.Minute

const (
	stripeTopUpCreditUnit      int64 = 20
	stripeTopUpUnitAmountMinor int64 = 2000
	stripeMaxTopUp             int64 = 10000
	stripeTopUpCurrency              = "CNY"
)

type permanentStripeWebhookError struct {
	err error
}

func (e *permanentStripeWebhookError) Error() string {
	return e.err.Error()
}

func (e *permanentStripeWebhookError) Unwrap() error {
	return e.err
}

func rejectStripeWebhook(message string) error {
	return &permanentStripeWebhookError{err: errors.New(message)}
}

func isPermanentStripeWebhookError(err error) bool {
	var permanentError *permanentStripeWebhookError
	if errors.As(err, &permanentError) {
		return true
	}
	return errors.Is(err, model.ErrPaymentMethodMismatch) ||
		errors.Is(err, model.ErrTopUpStatusInvalid) ||
		errors.Is(err, model.ErrStripeSnapshotMismatch) ||
		errors.Is(err, model.ErrStripeAdjustmentMismatch) ||
		errors.Is(err, model.ErrStripeSubscriptionMismatch)
}

var createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	client := stripe.NewClient(setting.StripeApiSecret)
	return client.V1CheckoutSessions.Create(params.Context, params)
}

var fetchStripeRefundsForCharge = func(ctx context.Context, chargeId string) ([]*stripe.Refund, error) {
	client := stripe.NewClient(setting.StripeApiSecret)
	params := &stripe.RefundListParams{Charge: stripe.String(chargeId)}
	refunds := make([]*stripe.Refund, 0)
	for refund, err := range client.V1Refunds.List(ctx, params).All(ctx) {
		if err != nil {
			return nil, err
		}
		refunds = append(refunds, refund)
	}
	return refunds, nil
}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the displayed credit amount to add to the local wallet.
	// It must be a whole number of Stripe top-up packages.
	Amount int64 `json:"amount"`
	// PaymentMethod specifies the payment method (e.g., "stripe").
	PaymentMethod string `json:"payment_method"`
	// SuccessURL is the optional custom URL to redirect after successful payment.
	// If empty, defaults to the server's console log page.
	SuccessURL string `json:"success_url,omitempty"`
	// CancelURL is the optional custom URL to redirect when payment is canceled.
	// If empty, defaults to the server's console topup page.
	CancelURL string `json:"cancel_url,omitempty"`
}

type StripeAdaptor struct {
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	_, expectedAmountMinor, err := stripeTopUpPricing(req.Amount)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	payMoney := decimal.NewFromInt(expectedAmountMinor).Div(decimal.NewFromInt(100))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": payMoney.StringFixed(2)})
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	if req.PaymentMethod != model.PaymentMethodStripe {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	quantity, expectedAmountMinor, err := stripeTopUpPricing(req.Amount)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	if req.SuccessURL != "" && common.ValidateRedirectURL(req.SuccessURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付成功重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	if req.CancelURL != "" && common.ValidateRedirectURL(req.CancelURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付取消重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 获取用户失败 user_id=%d error=%q", id, err))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户失败"})
		return
	}
	stripePrice, err := retrieveStripePrice(c.Request.Context(), setting.StripePriceId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 获取充值 Price 失败 price_id=%s error=%q", setting.StripePriceId, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Stripe 充值价格校验失败"})
		return
	}
	if err := validateStripeTopUpPrice(stripePrice, setting.StripePriceId, stripeLivemodeForSecret(setting.StripeApiSecret)); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 充值 Price 与本地套餐不匹配 price_id=%s error=%q", setting.StripePriceId, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Stripe 充值价格配置与本地套餐不匹配"})
		return
	}

	chargedMoney := float64(req.Amount)
	creditedQuota, quotaClamp := common.QuotaFromDecimalChecked(decimal.NewFromFloat(chargedMoney).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if quotaClamp != nil || creditedQuota <= 0 {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 计算充值额度失败 user_id=%d amount=%d money=%.2f clamp=%v", id, req.Amount, chargedMoney, quotaClamp))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值额度无效"})
		return
	}
	reference := fmt.Sprintf("new-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	topUp := &model.TopUp{
		UserId:              id,
		Amount:              req.Amount,
		Money:               chargedMoney,
		CreditedQuota:       int64(creditedQuota),
		ExpectedAmountMinor: expectedAmountMinor,
		ExpectedCurrency:    stripeTopUpCurrency,
		TradeNo:             referenceId,
		PaymentMethod:       model.PaymentMethodStripe,
		PaymentProvider:     model.PaymentProviderStripe,
		ProviderProductId:   setting.StripePriceId,
		CreateTime:          time.Now().Unix(),
		Status:              common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, referenceId, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	checkoutSession, err := genStripeLink(c.Request.Context(), referenceId, user.StripeCustomer, user.Email, quantity, req.SuccessURL, req.CancelURL)
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建 Checkout Session 失败 user_id=%d trade_no=%s amount=%d error=%q", id, referenceId, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if checkoutSession.AmountTotal != expectedAmountMinor ||
		!strings.EqualFold(string(checkoutSession.Currency), topUp.ExpectedCurrency) ||
		checkoutSession.Livemode != stripeLivemodeForSecret(setting.StripeApiSecret) {
		_ = model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe Checkout Session 响应与充值订单不匹配 user_id=%d trade_no=%s session_id=%s", id, referenceId, checkoutSession.ID))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	customerId := ""
	if checkoutSession.Customer != nil {
		customerId = checkoutSession.Customer.ID
	}
	if err := topUp.BindStripeCheckout(model.StripeCheckoutBinding{
		OrderId: checkoutSession.ID, ProductId: setting.StripePriceId, CustomerId: customerId,
		AmountMinor: checkoutSession.AmountTotal, Currency: string(checkoutSession.Currency), Livemode: checkoutSession.Livemode,
	}); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 保存 Checkout Session 失败 user_id=%d trade_no=%s session_id=%s error=%q", id, referenceId, checkoutSession.ID, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Stripe 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f", id, referenceId, req.Amount, chargedMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": checkoutSession.URL,
		},
	})
}

func RequestStripeAmount(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isStripeTopUpEnabled() {
		common.ApiErrorMsg(c, "Stripe 充值未配置")
		return
	}
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestAmount(c, &req)
}

func RequestStripePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isStripeTopUpEnabled() {
		common.ApiErrorMsg(c, "Stripe 充值未配置")
		return
	}
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestPay(c, &req)
}

func StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isStripeWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 收到请求 path=%q client_ip=%s body_bytes=%d", c.Request.RequestURI, c.ClientIP(), len(payload)))
	event, err := webhook.ConstructEvent(payload, c.GetHeader("Stripe-Signature"), setting.StripeWebhookSecret)

	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook 验签失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if event.ID == "" || event.Type == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	payloadDigest := sha256.Sum256(payload)
	claim, err := model.ClaimStripeWebhookEvent(event.ID, string(event.Type), event.Livemode, fmt.Sprintf("%x", payloadDigest), stripeWebhookLeaseDuration)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe webhook 事件入账失败 event_id=%s event_type=%s error=%q", event.ID, event.Type, err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if claim.PayloadMismatch {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook Event ID payload 不一致 event_id=%s event_type=%s", event.ID, event.Type))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if claim.AlreadyFinal {
		c.Status(http.StatusOK)
		return
	}
	if claim.InProgress || !claim.ShouldProcess {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 验签成功 event_id=%s event_type=%s client_ip=%s path=%q", event.ID, string(event.Type), c.ClientIP(), c.Request.RequestURI))
	handleErr := processStripeWebhookEvent(ctx, event, c.ClientIP())
	if handleErr == nil {
		if err := model.FinishStripeWebhookEvent(event.ID, claim.Attempt, model.StripeWebhookEventStatusSucceeded, ""); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Stripe webhook 更新事件状态失败 event_id=%s event_type=%s error=%q", event.ID, event.Type, err.Error()))
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
		return
	}

	status := model.StripeWebhookEventStatusFailed
	responseStatus := http.StatusServiceUnavailable
	if isPermanentStripeWebhookError(handleErr) {
		status = model.StripeWebhookEventStatusRejected
		responseStatus = http.StatusOK
	}
	if err := model.FinishStripeWebhookEvent(event.ID, claim.Attempt, status, handleErr.Error()); err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe webhook 更新事件状态失败 event_id=%s event_type=%s process_error=%q finish_error=%q", event.ID, event.Type, handleErr.Error(), err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	logger.LogError(ctx, fmt.Sprintf("Stripe webhook 处理失败 event_id=%s event_type=%s error=%q", event.ID, event.Type, handleErr.Error()))
	c.AbortWithStatus(responseStatus)
}

func stripeLivemodeForSecret(secret string) bool {
	return strings.HasPrefix(secret, "sk_live_") || strings.HasPrefix(secret, "rk_live_")
}

func processStripeWebhookEvent(ctx context.Context, event stripe.Event, callerIp string) error {
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted,
		stripe.EventTypeCheckoutSessionExpired,
		stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded,
		stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		if event.Data == nil || len(event.Data.Raw) == 0 {
			return rejectStripeWebhook("Stripe webhook 缺少事件数据")
		}
		var checkoutSession stripe.CheckoutSession
		if err := common.Unmarshal(event.Data.Raw, &checkoutSession); err != nil {
			return rejectStripeWebhook("Stripe webhook Checkout Session 无效")
		}
		if checkoutSession.Livemode != event.Livemode {
			return rejectStripeWebhook("Stripe Checkout Session livemode 不匹配")
		}
		switch event.Type {
		case stripe.EventTypeCheckoutSessionCompleted:
			return sessionCompleted(ctx, event, &checkoutSession, callerIp)
		case stripe.EventTypeCheckoutSessionExpired:
			return sessionExpired(ctx, &checkoutSession)
		case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
			return sessionAsyncPaymentSucceeded(ctx, event, &checkoutSession, callerIp)
		default:
			return sessionAsyncPaymentFailed(ctx, event, &checkoutSession, callerIp)
		}
	case stripe.EventTypeChargeRefunded:
		if event.Data == nil || len(event.Data.Raw) == 0 {
			return rejectStripeWebhook("Stripe webhook 缺少 Charge 数据")
		}
		var charge stripe.Charge
		if err := common.Unmarshal(event.Data.Raw, &charge); err != nil {
			return rejectStripeWebhook("Stripe webhook Charge 无效")
		}
		return processStripeChargeRefunded(ctx, event, &charge)
	case stripe.EventTypeRefundCreated, stripe.EventTypeRefundUpdated, stripe.EventTypeRefundFailed:
		if event.Data == nil || len(event.Data.Raw) == 0 {
			return rejectStripeWebhook("Stripe webhook 缺少 Refund 数据")
		}
		var refund stripe.Refund
		if err := common.Unmarshal(event.Data.Raw, &refund); err != nil {
			return rejectStripeWebhook("Stripe webhook Refund 无效")
		}
		var refundMode struct {
			Livemode *bool `json:"livemode"`
		}
		if err := common.Unmarshal(event.Data.Raw, &refundMode); err != nil {
			return rejectStripeWebhook("Stripe webhook Refund 无效")
		}
		if refundMode.Livemode != nil && *refundMode.Livemode != event.Livemode {
			return rejectStripeWebhook("Stripe Refund livemode 不匹配")
		}
		return processStripeRefund(ctx, event, &refund, "", "")
	case stripe.EventTypeChargeDisputeCreated,
		stripe.EventTypeChargeDisputeFundsWithdrawn,
		stripe.EventTypeChargeDisputeUpdated,
		stripe.EventTypeChargeDisputeClosed,
		stripe.EventTypeChargeDisputeFundsReinstated:
		if event.Data == nil || len(event.Data.Raw) == 0 {
			return rejectStripeWebhook("Stripe webhook 缺少 Dispute 数据")
		}
		var dispute stripe.Dispute
		if err := common.Unmarshal(event.Data.Raw, &dispute); err != nil {
			return rejectStripeWebhook("Stripe webhook Dispute 无效")
		}
		return processStripeDispute(ctx, event, &dispute)
	default:
		logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 忽略事件 event_id=%s event_type=%s", event.ID, event.Type))
		return nil
	}
}

func processStripeChargeRefunded(ctx context.Context, event stripe.Event, charge *stripe.Charge) error {
	if charge == nil || charge.ID == "" || charge.Livemode != event.Livemode || charge.AmountRefunded <= 0 {
		return rejectStripeWebhook("Stripe charge.refunded 数据无效")
	}
	var refunds []*stripe.Refund
	if charge.Refunds == nil || len(charge.Refunds.Data) == 0 || charge.Refunds.HasMore {
		var err error
		refunds, err = fetchStripeRefundsForCharge(ctx, charge.ID)
		if err != nil {
			return fmt.Errorf("获取 Stripe Charge %s 的完整退款列表失败: %w", charge.ID, err)
		}
		if len(refunds) == 0 {
			return fmt.Errorf("Stripe Charge %s 的完整退款列表暂不可用", charge.ID)
		}
	} else {
		refunds = charge.Refunds.Data
	}
	paymentIntentId := ""
	if charge.PaymentIntent != nil {
		paymentIntentId = charge.PaymentIntent.ID
	}
	for _, refund := range refunds {
		if refund == nil {
			continue
		}
		if err := processStripeRefund(ctx, event, refund, paymentIntentId, charge.ID); err != nil {
			return err
		}
	}
	return nil
}

func processStripeRefund(ctx context.Context, event stripe.Event, refund *stripe.Refund, fallbackPaymentIntentId string, fallbackChargeId string) error {
	if refund == nil || refund.ID == "" || refund.Amount <= 0 || strings.TrimSpace(string(refund.Currency)) == "" || event.Created <= 0 {
		return rejectStripeWebhook("Stripe Refund 数据无效")
	}
	paymentIntentId := strings.TrimSpace(fallbackPaymentIntentId)
	if refund.PaymentIntent != nil && refund.PaymentIntent.ID != "" {
		paymentIntentId = refund.PaymentIntent.ID
	}
	chargeId := strings.TrimSpace(fallbackChargeId)
	if refund.Charge != nil && refund.Charge.ID != "" {
		chargeId = refund.Charge.ID
	}
	if paymentIntentId == "" && chargeId == "" {
		return rejectStripeWebhook("Stripe Refund 缺少 PaymentIntent 或 Charge")
	}
	active := refund.Status == stripe.RefundStatusSucceeded
	priority := 1
	switch refund.Status {
	case stripe.RefundStatusPending, stripe.RefundStatusRequiresAction:
		priority = 2
	case stripe.RefundStatusSucceeded:
		priority = 3
	case stripe.RefundStatusFailed, stripe.RefundStatusCanceled:
		priority = 4
	}
	if event.Type == stripe.EventTypeRefundFailed {
		active = false
		priority = 5
	}
	result, err := model.ApplyStripePaymentAdjustment(model.StripePaymentAdjustmentInput{
		ObjectType: model.StripeAdjustmentRefund, ObjectId: refund.ID, EventId: event.ID,
		PaymentIntentId: paymentIntentId, ChargeId: chargeId, AmountMinor: refund.Amount,
		Currency: string(refund.Currency), Livemode: event.Livemode, Status: string(refund.Status),
		Active: active, EventCreated: event.Created, EventPriority: priority,
	})
	if errors.Is(err, model.ErrStripePaymentUnmanaged) {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe Refund 不属于本站订单，已忽略 event_id=%s refund_id=%s", event.ID, refund.ID))
		return nil
	}
	if err != nil {
		return err
	}
	logger.LogWarn(ctx, fmt.Sprintf(
		"Stripe Refund 权益同步完成 event_id=%s refund_id=%s target=%s:%d recovered_quota=%d outstanding_quota=%d",
		event.ID, refund.ID, result.TargetKind, result.TargetId, result.RecoveredQuota, result.OutstandingQuota,
	))
	return nil
}

func processStripeDispute(ctx context.Context, event stripe.Event, dispute *stripe.Dispute) error {
	if dispute == nil || dispute.ID == "" || dispute.Amount <= 0 || strings.TrimSpace(string(dispute.Currency)) == "" ||
		dispute.Livemode != event.Livemode || event.Created <= 0 {
		return rejectStripeWebhook("Stripe Dispute 数据无效")
	}
	paymentIntentId := ""
	if dispute.PaymentIntent != nil {
		paymentIntentId = dispute.PaymentIntent.ID
	}
	chargeId := ""
	if dispute.Charge != nil {
		chargeId = dispute.Charge.ID
	}
	if paymentIntentId == "" && chargeId == "" {
		return rejectStripeWebhook("Stripe Dispute 缺少 PaymentIntent 或 Charge")
	}
	active := dispute.Status == stripe.DisputeStatusNeedsResponse ||
		dispute.Status == stripe.DisputeStatusUnderReview ||
		dispute.Status == stripe.DisputeStatusLost
	priority := 1
	switch event.Type {
	case stripe.EventTypeChargeDisputeFundsWithdrawn:
		active = true
		priority = 2
	case stripe.EventTypeChargeDisputeUpdated:
		priority = 3
	case stripe.EventTypeChargeDisputeClosed:
		active = dispute.Status == stripe.DisputeStatusLost
		priority = 4
	case stripe.EventTypeChargeDisputeFundsReinstated:
		active = false
		priority = 5
	}
	result, err := model.ApplyStripePaymentAdjustment(model.StripePaymentAdjustmentInput{
		ObjectType: model.StripeAdjustmentDispute, ObjectId: dispute.ID, EventId: event.ID,
		PaymentIntentId: paymentIntentId, ChargeId: chargeId, AmountMinor: dispute.Amount,
		Currency: string(dispute.Currency), Livemode: dispute.Livemode, Status: string(dispute.Status),
		Active: active, EventCreated: event.Created, EventPriority: priority,
	})
	if errors.Is(err, model.ErrStripePaymentUnmanaged) {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe Dispute 不属于本站订单，已忽略 event_id=%s dispute_id=%s", event.ID, dispute.ID))
		return nil
	}
	if err != nil {
		return err
	}
	logger.LogWarn(ctx, fmt.Sprintf(
		"Stripe Dispute 权益同步完成 event_id=%s dispute_id=%s target=%s:%d recovered_quota=%d outstanding_quota=%d",
		event.ID, dispute.ID, result.TargetKind, result.TargetId, result.RecoveredQuota, result.OutstandingQuota,
	))
	return nil
}

func sessionCompleted(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	if checkoutSession.Status != stripe.CheckoutSessionStatusComplete {
		return rejectStripeWebhook(fmt.Sprintf("checkout.completed 状态异常 trade_no=%s status=%s", checkoutSession.ClientReferenceID, checkoutSession.Status))
	}

	if checkoutSession.Mode != stripe.CheckoutSessionModePayment {
		return rejectStripeWebhook("Stripe Checkout Session 订单类型无效")
	}

	if checkoutSession.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe Checkout 支付未完成，等待异步结果 trade_no=%s payment_status=%s client_ip=%s", checkoutSession.ClientReferenceID, checkoutSession.PaymentStatus, callerIp))
		return nil
	}

	if isStripeSubscriptionPayment(checkoutSession) {
		return fulfillSubscriptionOrder(ctx, event, checkoutSession, callerIp)
	}
	return fulfillOrder(ctx, event, checkoutSession, callerIp)
}

// sessionAsyncPaymentSucceeded handles delayed payment methods (bank transfer, SEPA, etc.)
// that confirm payment after the checkout session completes.
func sessionAsyncPaymentSucceeded(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	if checkoutSession.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return rejectStripeWebhook("Stripe 异步支付成功事件的 payment_status 不是 paid")
	}
	logger.LogInfo(ctx, fmt.Sprintf("Stripe 异步支付成功 trade_no=%s client_ip=%s", checkoutSession.ClientReferenceID, callerIp))
	if checkoutSession.Mode != stripe.CheckoutSessionModePayment {
		return rejectStripeWebhook("Stripe Checkout Session 订单类型无效")
	}

	if isStripeSubscriptionPayment(checkoutSession) {
		return fulfillSubscriptionOrder(ctx, event, checkoutSession, callerIp)
	}
	return fulfillOrder(ctx, event, checkoutSession, callerIp)
}

// sessionAsyncPaymentFailed records a delayed one-time payment failure.
func sessionAsyncPaymentFailed(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	referenceId := checkoutSession.ClientReferenceID
	logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败 trade_no=%s client_ip=%s", referenceId, callerIp))

	if referenceId == "" {
		return errors.New("Stripe 异步支付失败事件缺少订单号")
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	switch checkoutSession.Mode {
	case stripe.CheckoutSessionModePayment:
		if isStripeSubscriptionPayment(checkoutSession) {
			return expireStripeSubscriptionPaymentOrder(ctx, checkoutSession, "异步支付失败")
		}
	default:
		return rejectStripeWebhook("Stripe Checkout Session 订单类型无效")
	}

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		return fmt.Errorf("Stripe 异步支付失败但本地订单不存在 trade_no=%s", referenceId)
	}
	if err := validateStripeTopUp(topUp, checkoutSession); err != nil {
		return err
	}
	if topUp.Status != common.TopUpStatusPending {
		return nil
	}
	if err := model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusFailed); err != nil {
		return err
	}
	logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值订单已标记为失败 trade_no=%s client_ip=%s", referenceId, callerIp))
	return nil
}

func isStripeSubscriptionPayment(checkoutSession *stripe.CheckoutSession) bool {
	return checkoutSession != nil &&
		checkoutSession.Mode == stripe.CheckoutSessionModePayment &&
		checkoutSession.Metadata["order_kind"] == "subscription"
}

// expireStripeSubscriptionPaymentOrder must be called while the order lock is held.
func expireStripeSubscriptionPaymentOrder(ctx context.Context, checkoutSession *stripe.CheckoutSession, reason string) error {
	referenceId := checkoutSession.ClientReferenceID
	order := model.GetSubscriptionOrderByTradeNo(referenceId)
	if order == nil {
		return model.ErrSubscriptionOrderNotFound
	}
	if err := validateStripeSubscriptionPaymentOrder(order, checkoutSession); err != nil {
		return err
	}
	if order.Status != common.TopUpStatusPending {
		return nil
	}
	if err := model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe); err != nil {
		return err
	}
	logger.LogInfo(ctx, fmt.Sprintf("Stripe 一次性订阅订单已标记为%s trade_no=%s", reason, referenceId))
	return nil
}

func fulfillSubscriptionOrder(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	referenceId := checkoutSession.ClientReferenceID
	if referenceId == "" {
		return errors.New("Stripe 完成订阅订单时缺少订单号")
	}
	if checkoutSession.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return rejectStripeWebhook("Stripe 一次性订阅 Checkout Session 尚未支付")
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	order := model.GetSubscriptionOrderByTradeNo(referenceId)
	if order == nil {
		return model.ErrSubscriptionOrderNotFound
	}
	if err := validateStripeSubscriptionPaymentOrder(order, checkoutSession); err != nil {
		return err
	}
	paymentIntentId := ""
	chargeId := ""
	if checkoutSession.PaymentIntent != nil {
		paymentIntentId = checkoutSession.PaymentIntent.ID
		if checkoutSession.PaymentIntent.LatestCharge != nil {
			chargeId = checkoutSession.PaymentIntent.LatestCharge.ID
		}
	}
	if paymentIntentId == "" && chargeId == "" {
		return rejectStripeWebhook("Stripe 一次性订阅 Checkout Session 缺少 PaymentIntent 或 Charge")
	}
	customerId := ""
	if checkoutSession.Customer != nil {
		customerId = checkoutSession.Customer.ID
	}
	if err := model.CompleteStripeOneTimeSubscriptionOrder(
		referenceId,
		common.GetJsonString(checkoutSession),
		model.StripeOneTimeSubscriptionPayment{
			PaymentIntentId: paymentIntentId,
			ChargeId:        chargeId,
			CustomerId:      customerId,
			AmountMinor:     checkoutSession.AmountTotal,
			Currency:        string(checkoutSession.Currency),
			Livemode:        checkoutSession.Livemode,
		},
	); err != nil {
		return err
	}
	logger.LogInfo(ctx, fmt.Sprintf(
		"Stripe 一次性订阅权益发放成功 trade_no=%s event_type=%s client_ip=%s",
		referenceId, string(event.Type), callerIp,
	))
	return nil
}

// fulfillOrder is the shared logic for crediting quota after payment is confirmed.
func fulfillOrder(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	referenceId := checkoutSession.ClientReferenceID
	if referenceId == "" {
		return errors.New("Stripe 完成订单时缺少订单号")
	}
	customerId := ""
	if checkoutSession.Customer != nil {
		customerId = checkoutSession.Customer.ID
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		return fmt.Errorf("Stripe 本地订单不存在 trade_no=%s", referenceId)
	}
	if err := validateStripeTopUp(topUp, checkoutSession); err != nil {
		return err
	}
	paymentIntentId := ""
	chargeId := ""
	if checkoutSession.PaymentIntent != nil {
		paymentIntentId = checkoutSession.PaymentIntent.ID
		if checkoutSession.PaymentIntent.LatestCharge != nil {
			chargeId = checkoutSession.PaymentIntent.LatestCharge.ID
		}
	}
	if paymentIntentId == "" {
		return errors.New("Stripe Checkout Session 尚未包含 PaymentIntent")
	}
	if err := model.Recharge(referenceId, model.StripeTopUpSettlement{
		CustomerId: customerId, PaymentIntentId: paymentIntentId, ChargeId: chargeId,
		AmountMinor: checkoutSession.AmountTotal, Currency: string(checkoutSession.Currency), Livemode: checkoutSession.Livemode,
	}, callerIp); err != nil {
		return err
	}

	logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值成功 trade_no=%s amount_total=%.2f currency=%s event_type=%s client_ip=%s", referenceId, float64(checkoutSession.AmountTotal)/100, strings.ToUpper(string(checkoutSession.Currency)), string(event.Type), callerIp))
	return nil
}

func sessionExpired(ctx context.Context, checkoutSession *stripe.CheckoutSession) error {
	referenceId := checkoutSession.ClientReferenceID
	if checkoutSession.Status != stripe.CheckoutSessionStatusExpired {
		return rejectStripeWebhook(fmt.Sprintf("checkout.expired 状态异常 trade_no=%s status=%s", referenceId, checkoutSession.Status))
	}
	if checkoutSession.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
		return rejectStripeWebhook("Stripe checkout.expired 不应包含已支付状态")
	}

	if referenceId == "" {
		return errors.New("Stripe checkout.expired 缺少订单号")
	}

	// Subscription order expiration
	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	switch checkoutSession.Mode {
	case stripe.CheckoutSessionModePayment:
		if isStripeSubscriptionPayment(checkoutSession) {
			return expireStripeSubscriptionPaymentOrder(ctx, checkoutSession, "Checkout 已过期")
		}
	default:
		return rejectStripeWebhook("Stripe Checkout Session 订单类型无效")
	}

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		return fmt.Errorf("Stripe 充值订单不存在，无法标记过期 trade_no=%s", referenceId)
	}
	if err := validateStripeTopUp(topUp, checkoutSession); err != nil {
		return err
	}
	if topUp.Status != common.TopUpStatusPending {
		return nil
	}
	if err := model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusExpired); err != nil {
		return err
	}

	logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值订单已过期 trade_no=%s", referenceId))
	return nil
}

func validateStripeTopUp(topUp *model.TopUp, checkoutSession *stripe.CheckoutSession) error {
	if topUp.PaymentProvider != model.PaymentProviderStripe {
		return model.ErrPaymentMethodMismatch
	}
	if err := validateStripeCheckoutOrder(topUp.TradeNo, "topup", topUp.ProviderOrderId, topUp.ProviderProductId, stripe.CheckoutSessionModePayment, checkoutSession); err != nil {
		return err
	}
	if topUp.CreditedQuota <= 0 || topUp.ExpectedAmountMinor <= 0 || topUp.ExpectedCurrency == "" {
		return rejectStripeWebhook("Stripe 充值订单缺少不可变支付快照")
	}
	if checkoutSession.AmountTotal != topUp.ExpectedAmountMinor ||
		!strings.EqualFold(string(checkoutSession.Currency), topUp.ExpectedCurrency) ||
		checkoutSession.Livemode != topUp.ProviderLivemode {
		return rejectStripeWebhook("Stripe Checkout Session 金额、币种或模式不匹配")
	}
	if topUp.ProviderCustomerId != "" && (checkoutSession.Customer == nil || checkoutSession.Customer.ID != topUp.ProviderCustomerId) {
		return rejectStripeWebhook("Stripe Checkout Session Customer 不匹配")
	}
	return nil
}

func validateStripeSubscriptionPaymentOrder(order *model.SubscriptionOrder, checkoutSession *stripe.CheckoutSession) error {
	if order.PaymentProvider != model.PaymentProviderStripe {
		return model.ErrPaymentMethodMismatch
	}
	if err := validateStripeCheckoutOrder(order.TradeNo, "subscription", order.ProviderOrderId, order.ProviderProductId, stripe.CheckoutSessionModePayment, checkoutSession); err != nil {
		return err
	}
	if order.ExpectedAmountMinor <= 0 || order.ExpectedCurrency == "" ||
		checkoutSession.AmountTotal != order.ExpectedAmountMinor ||
		!strings.EqualFold(string(checkoutSession.Currency), order.ExpectedCurrency) ||
		checkoutSession.Livemode != order.ProviderLivemode {
		return rejectStripeWebhook("Stripe 一次性订阅 Checkout Session 金额、币种或模式不匹配")
	}
	if order.ProviderCustomerId != "" && (checkoutSession.Customer == nil || checkoutSession.Customer.ID != order.ProviderCustomerId) {
		return rejectStripeWebhook("Stripe 一次性订阅 Checkout Session Customer 不匹配")
	}
	if checkoutSession.Subscription != nil && checkoutSession.Subscription.ID != "" {
		return rejectStripeWebhook("Stripe 一次性订阅 Checkout Session 不应包含 Subscription")
	}
	return nil
}

func validateStripeCheckoutOrder(tradeNo string, orderKind string, providerOrderId string, providerProductId string, expectedMode stripe.CheckoutSessionMode, checkoutSession *stripe.CheckoutSession) error {
	if checkoutSession == nil || checkoutSession.ID == "" {
		return rejectStripeWebhook("Stripe Checkout Session 无效")
	}
	if providerOrderId == "" || checkoutSession.ID != providerOrderId {
		return rejectStripeWebhook("Stripe Checkout Session 与本地订单不匹配")
	}
	if checkoutSession.ClientReferenceID != tradeNo {
		return rejectStripeWebhook("Stripe Checkout Session 订单号不匹配")
	}
	if checkoutSession.Mode != expectedMode {
		return rejectStripeWebhook("Stripe Checkout Session 订单类型不匹配")
	}
	if checkoutSession.Metadata["trade_no"] != tradeNo {
		return rejectStripeWebhook("Stripe Checkout Session metadata 订单号不匹配")
	}
	if checkoutSession.Metadata["order_kind"] != orderKind {
		return rejectStripeWebhook("Stripe Checkout Session metadata 订单类型不匹配")
	}
	if providerProductId == "" || checkoutSession.Metadata["price_id"] != providerProductId {
		return rejectStripeWebhook("Stripe Checkout Session Price 不匹配")
	}
	return nil
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - quantity: number of fixed-price credit packages to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(ctx context.Context, referenceId string, customerId string, email string, quantity int64, successURL string, cancelURL string) (*stripe.CheckoutSession, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return nil, fmt.Errorf("无效的Stripe API密钥")
	}
	if successURL != "" {
		if err := common.ValidateRedirectURL(successURL); err != nil {
			return nil, fmt.Errorf("invalid Stripe success redirect URL: %w", err)
		}
	}
	if cancelURL != "" {
		if err := common.ValidateRedirectURL(cancelURL); err != nil {
			return nil, fmt.Errorf("invalid Stripe cancel redirect URL: %w", err)
		}
	}

	// Use custom URLs if provided, otherwise use defaults
	if successURL == "" {
		successURL = paymentReturnPath("/usage-logs")
	}
	if cancelURL == "" {
		cancelURL = paymentReturnPath("/wallet")
	}

	params := &stripe.CheckoutSessionCreateParams{
		Params:            stripe.Params{Context: ctx},
		ClientReferenceID: stripe.String(referenceId),
		IntegrationIdentifier: stripe.String(
			"tryvalo_topup_" + randstr.String(8, "abcdefghijklmnopqrstuvwxyz"),
		),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(setting.StripePriceId),
				Quantity: stripe.Int64(quantity),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		ManagedPayments: &stripe.CheckoutSessionCreateManagedPaymentsParams{
			Enabled: stripe.Bool(false),
		},
		PaymentMethodOptions: &stripe.CheckoutSessionCreatePaymentMethodOptionsParams{
			WeChatPay: &stripe.CheckoutSessionCreatePaymentMethodOptionsWeChatPayParams{
				Client: stripe.String(string(stripe.CheckoutSessionPaymentMethodOptionsWeChatPayClientWeb)),
			},
		},
		AllowPromotionCodes: stripe.Bool(false),
		Metadata: map[string]string{
			"trade_no":   referenceId,
			"order_kind": "topup",
			"price_id":   setting.StripePriceId,
		},
		Expand: []*string{stripe.String("payment_intent.latest_charge")},
		PaymentIntentData: &stripe.CheckoutSessionCreatePaymentIntentDataParams{
			Metadata: map[string]string{
				"trade_no":   referenceId,
				"order_kind": "topup",
				"price_id":   setting.StripePriceId,
			},
		},
	}
	params.SetIdempotencyKey("checkout-" + referenceId)

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := createStripeCheckoutSession(params)
	if err != nil {
		return nil, err
	}
	if result == nil || result.ID == "" || result.URL == "" {
		return nil, errors.New("Stripe Checkout Session 响应不完整")
	}

	return result, nil
}

func stripeTopUpPricing(amount int64) (quantity int64, expectedAmountMinor int64, err error) {
	if amount < stripeTopUpCreditUnit {
		return 0, 0, fmt.Errorf("充值额度不能小于 %d", stripeTopUpCreditUnit)
	}
	if amount > stripeMaxTopUp {
		return 0, 0, fmt.Errorf("充值额度不能大于 %d", stripeMaxTopUp)
	}
	if amount%stripeTopUpCreditUnit != 0 {
		return 0, 0, fmt.Errorf("充值额度必须是 %d 的整数倍", stripeTopUpCreditUnit)
	}

	quantity = amount / stripeTopUpCreditUnit
	expectedAmountMinor = quantity * stripeTopUpUnitAmountMinor
	return quantity, expectedAmountMinor, nil
}

func validateStripeTopUpPrice(price *stripe.Price, expectedPriceId string, expectedLivemode bool) error {
	if price == nil {
		return errors.New("Stripe Price 为空")
	}
	if price.ID != strings.TrimSpace(expectedPriceId) {
		return errors.New("Stripe Price ID 不匹配")
	}
	if !price.Active {
		return errors.New("Stripe Price 未启用")
	}
	if price.Type != stripe.PriceTypeOneTime || price.Recurring != nil {
		return errors.New("Stripe Price 必须是一次性价格")
	}
	if price.BillingScheme != stripe.PriceBillingSchemePerUnit || price.CustomUnitAmount != nil || price.TransformQuantity != nil {
		return errors.New("Stripe Price 必须按固定单价计费")
	}
	if !strings.EqualFold(string(price.Currency), stripeTopUpCurrency) {
		return fmt.Errorf("Stripe Price 币种必须是 %s", stripeTopUpCurrency)
	}
	if price.UnitAmount != stripeTopUpUnitAmountMinor {
		return fmt.Errorf("Stripe Price 单价必须是 %d 分", stripeTopUpUnitAmountMinor)
	}
	if price.Livemode != expectedLivemode {
		return errors.New("Stripe Price 模式与 API 密钥不匹配")
	}
	return nil
}
