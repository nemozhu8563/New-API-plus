package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
	"github.com/thanhpk/randstr"
)

var stripeAdaptor = &StripeAdaptor{}

const stripeWebhookLeaseDuration = time.Minute

const stripeMaxTopUp int64 = 10000

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
		errors.Is(err, model.ErrStripeSubscriptionMismatch) ||
		errors.Is(err, model.ErrStripeInvoiceAlreadyBound) ||
		errors.Is(err, model.ErrStripeSubscriptionPeriodOverlap)
}

var createStripeCheckoutSession = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	client := stripe.NewClient(setting.StripeApiSecret)
	return client.V1CheckoutSessions.Create(params.Context, params)
}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the quantity of units to purchase.
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

type stripeInvoiceAuditSnapshot struct {
	InvoiceId            string `json:"invoice_id"`
	CustomerId           string `json:"customer_id"`
	SubscriptionId       string `json:"subscription_id"`
	ProductId            string `json:"product_id"`
	Quantity             int64  `json:"quantity"`
	UnitAmountMinor      int64  `json:"unit_amount_minor"`
	InvoiceTotalMinor    int64  `json:"invoice_total_minor"`
	AmountPaidMinor      int64  `json:"amount_paid_minor"`
	AmountRemainingMinor int64  `json:"amount_remaining_minor"`
	Currency             string `json:"currency"`
	Livemode             bool   `json:"livemode"`
	PeriodStart          int64  `json:"period_start"`
	PeriodEnd            int64  `json:"period_end"`
	EventCreated         int64  `json:"event_created"`
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup())})
		return
	}
	if req.Amount > stripeMaxTopUp {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能大于 %d", stripeMaxTopUp)})
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getStripePayMoney(float64(req.Amount), group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	if req.PaymentMethod != model.PaymentMethodStripe {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup()), "data": 10})
		return
	}
	if req.Amount > stripeMaxTopUp {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("充值数量不能大于 %d", stripeMaxTopUp), "data": 10})
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
	chargedMoney := GetChargedAmount(float64(req.Amount), *user)
	creditedQuota, quotaClamp := common.QuotaFromDecimalChecked(decimal.NewFromFloat(chargedMoney).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if quotaClamp != nil || creditedQuota <= 0 {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 计算充值额度失败 user_id=%d amount=%d money=%.2f clamp=%v", id, req.Amount, chargedMoney, quotaClamp))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值额度无效"})
		return
	}
	expectedAmountMinor, err := stripeTopUpAmountMinor(req.Amount, user.Group)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 计算支付金额失败 user_id=%d amount=%d error=%q", id, req.Amount, err))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付金额无效"})
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
		ExpectedCurrency:    "USD",
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

	checkoutSession, err := genStripeLink(c.Request.Context(), referenceId, user.StripeCustomer, user.Email, req.Amount, req.SuccessURL, req.CancelURL)
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
	case stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed:
		if event.Data == nil || len(event.Data.Raw) == 0 {
			return rejectStripeWebhook("Stripe webhook 缺少 Invoice 数据")
		}
		if event.Type == stripe.EventTypeInvoicePaid {
			var rawInvoice map[string]json.RawMessage
			if err := common.Unmarshal(event.Data.Raw, &rawInvoice); err != nil {
				return rejectStripeWebhook("Stripe webhook Invoice 无效")
			}
			rawAmountPaidOffStripe, ok := rawInvoice["amount_paid_off_stripe"]
			if !ok || strings.TrimSpace(string(rawAmountPaidOffStripe)) == "" || strings.TrimSpace(string(rawAmountPaidOffStripe)) == "null" {
				return rejectStripeWebhook("Stripe invoice.paid 缺少 amount_paid_off_stripe")
			}
			var amountPaidOffStripe int64
			if err := common.Unmarshal(rawAmountPaidOffStripe, &amountPaidOffStripe); err != nil {
				return rejectStripeWebhook("Stripe invoice.paid 的 amount_paid_off_stripe 无效")
			}
		}
		var invoice stripe.Invoice
		if err := common.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return rejectStripeWebhook("Stripe webhook Invoice 无效")
		}
		return processStripeInvoice(event, &invoice)
	case stripe.EventTypeCustomerSubscriptionUpdated, stripe.EventTypeCustomerSubscriptionDeleted:
		if event.Data == nil || len(event.Data.Raw) == 0 {
			return rejectStripeWebhook("Stripe webhook 缺少 Subscription 数据")
		}
		var subscription stripe.Subscription
		if err := common.Unmarshal(event.Data.Raw, &subscription); err != nil {
			return rejectStripeWebhook("Stripe webhook Subscription 无效")
		}
		return processStripeSubscriptionLifecycle(event, &subscription)
	case stripe.EventTypeChargeRefunded,
		stripe.EventTypeChargeDisputeCreated,
		stripe.EventTypeChargeDisputeFundsWithdrawn,
		stripe.EventTypeRefundCreated,
		stripe.EventTypeRefundUpdated:
		logger.LogWarn(ctx, fmt.Sprintf(
			"Stripe webhook 需要人工处理，当前不会自动撤销余额或订阅权益 event_id=%s event_type=%s",
			event.ID,
			event.Type,
		))
		return nil
	default:
		logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 忽略事件 event_id=%s event_type=%s", event.ID, event.Type))
		return nil
	}
}

func sessionCompleted(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	if checkoutSession.Status != stripe.CheckoutSessionStatusComplete {
		return rejectStripeWebhook(fmt.Sprintf("checkout.completed 状态异常 trade_no=%s status=%s", checkoutSession.ClientReferenceID, checkoutSession.Status))
	}

	switch checkoutSession.Mode {
	case stripe.CheckoutSessionModeSubscription:
		return bindStripeSubscriptionCheckout(checkoutSession)
	case stripe.CheckoutSessionModePayment:
	default:
		return rejectStripeWebhook("Stripe Checkout Session 订单类型无效")
	}

	if checkoutSession.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe Checkout 支付未完成，等待异步结果 trade_no=%s payment_status=%s client_ip=%s", checkoutSession.ClientReferenceID, checkoutSession.PaymentStatus, callerIp))
		return nil
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
	switch checkoutSession.Mode {
	case stripe.CheckoutSessionModeSubscription:
		return bindStripeSubscriptionCheckout(checkoutSession)
	case stripe.CheckoutSessionModePayment:
	default:
		return rejectStripeWebhook("Stripe Checkout Session 订单类型无效")
	}

	return fulfillOrder(ctx, event, checkoutSession, callerIp)
}

// sessionAsyncPaymentFailed records a delayed payment failure. Subscription
// invoices can still be retried by Stripe, so their local order remains open.
func sessionAsyncPaymentFailed(ctx context.Context, event stripe.Event, checkoutSession *stripe.CheckoutSession, callerIp string) error {
	referenceId := checkoutSession.ClientReferenceID
	logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败 trade_no=%s client_ip=%s", referenceId, callerIp))

	if referenceId == "" {
		return errors.New("Stripe 异步支付失败事件缺少订单号")
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	switch checkoutSession.Mode {
	case stripe.CheckoutSessionModeSubscription:
		order := model.GetSubscriptionOrderByTradeNo(referenceId)
		if order == nil {
			return model.ErrSubscriptionOrderNotFound
		}
		if err := validateStripeSubscriptionOrder(order, checkoutSession); err != nil {
			return err
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		if checkoutSession.Customer == nil || checkoutSession.Customer.ID == "" ||
			checkoutSession.Subscription == nil || checkoutSession.Subscription.ID == "" {
			return model.ErrStripeCheckoutUnbound
		}
		return model.MarkStripeSubscriptionPaymentFailed(
			referenceId,
			checkoutSession.Subscription.ID,
			checkoutSession.Customer.ID,
			checkoutSession.Livemode,
			event.Created,
		)
	case stripe.CheckoutSessionModePayment:
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
	case stripe.CheckoutSessionModeSubscription:
		order := model.GetSubscriptionOrderByTradeNo(referenceId)
		if order == nil {
			return model.ErrSubscriptionOrderNotFound
		}
		if err := validateStripeSubscriptionOrder(order, checkoutSession); err != nil {
			return err
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		if err := model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe); err != nil {
			return err
		}
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 订阅订单已过期 trade_no=%s", referenceId))
		return nil
	case stripe.CheckoutSessionModePayment:
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

func validateStripeSubscriptionOrder(order *model.SubscriptionOrder, checkoutSession *stripe.CheckoutSession) error {
	if order.PaymentProvider != model.PaymentProviderStripe {
		return model.ErrPaymentMethodMismatch
	}
	if err := validateStripeCheckoutOrder(order.TradeNo, "subscription", order.ProviderOrderId, order.ProviderProductId, stripe.CheckoutSessionModeSubscription, checkoutSession); err != nil {
		return err
	}
	if order.ExpectedAmountMinor <= 0 || order.ExpectedCurrency == "" ||
		checkoutSession.AmountTotal != order.ExpectedAmountMinor ||
		!strings.EqualFold(string(checkoutSession.Currency), order.ExpectedCurrency) ||
		checkoutSession.Livemode != order.ProviderLivemode {
		return rejectStripeWebhook("Stripe 订阅 Checkout Session 金额、币种或模式不匹配")
	}
	if order.ProviderCustomerId != "" && (checkoutSession.Customer == nil || checkoutSession.Customer.ID != order.ProviderCustomerId) {
		return rejectStripeWebhook("Stripe 订阅 Checkout Session Customer 不匹配")
	}
	if order.ProviderSubscriptionId != nil && (checkoutSession.Subscription == nil || checkoutSession.Subscription.ID != *order.ProviderSubscriptionId) {
		return rejectStripeWebhook("Stripe 订阅 Checkout Session Subscription 不匹配")
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

func bindStripeSubscriptionCheckout(checkoutSession *stripe.CheckoutSession) error {
	if checkoutSession == nil || checkoutSession.ClientReferenceID == "" {
		return rejectStripeWebhook("Stripe 订阅 Checkout Session 缺少订单号")
	}
	order := model.GetSubscriptionOrderByTradeNo(checkoutSession.ClientReferenceID)
	if order == nil {
		return model.ErrSubscriptionOrderNotFound
	}
	if err := validateStripeSubscriptionOrder(order, checkoutSession); err != nil {
		return err
	}
	if checkoutSession.Customer == nil || checkoutSession.Customer.ID == "" ||
		checkoutSession.Subscription == nil || checkoutSession.Subscription.ID == "" {
		return errors.New("Stripe 订阅 Checkout Session 尚未包含 Customer 或 Subscription")
	}
	return order.BindStripeSubscription(checkoutSession.Customer.ID, checkoutSession.Subscription.ID, checkoutSession.Livemode)
}

func processStripeInvoice(event stripe.Event, invoice *stripe.Invoice) error {
	if invoice == nil || invoice.ID == "" || invoice.Livemode != event.Livemode {
		return rejectStripeWebhook("Stripe Invoice 数据无效")
	}
	tradeNo := ""
	orderKind := ""
	metadataPriceId := ""
	subscriptionId := ""
	if invoice.Parent != nil &&
		invoice.Parent.Type == stripe.InvoiceParentTypeSubscriptionDetails &&
		invoice.Parent.SubscriptionDetails != nil {
		subscriptionDetails := invoice.Parent.SubscriptionDetails
		tradeNo = strings.TrimSpace(subscriptionDetails.Metadata["trade_no"])
		orderKind = strings.TrimSpace(subscriptionDetails.Metadata["order_kind"])
		metadataPriceId = strings.TrimSpace(subscriptionDetails.Metadata["price_id"])
		if subscriptionDetails.Subscription != nil {
			subscriptionId = strings.TrimSpace(subscriptionDetails.Subscription.ID)
		}
	}
	if tradeNo == "" && subscriptionId == "" {
		return rejectStripeWebhook("Stripe Invoice 缺少订阅订单标识")
	}
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil && subscriptionId != "" {
		order = model.GetStripeSubscriptionOrderByProviderSubscriptionId(subscriptionId)
	}
	if order == nil {
		return model.ErrSubscriptionOrderNotFound
	}
	if orderKind != "subscription" || metadataPriceId == "" || metadataPriceId != order.ProviderProductId {
		return rejectStripeWebhook("Stripe Invoice metadata 与订阅订单不匹配")
	}
	if tradeNo != "" && tradeNo != order.TradeNo {
		return rejectStripeWebhook("Stripe Invoice metadata 订单号不匹配")
	}
	if event.Type == stripe.EventTypeInvoicePaymentFailed {
		if subscriptionId == "" || invoice.Customer == nil || invoice.Customer.ID == "" {
			return rejectStripeWebhook("Stripe 失败账单缺少 Customer 或 Subscription")
		}
		return model.MarkStripeSubscriptionPaymentFailed(order.TradeNo, subscriptionId, invoice.Customer.ID, invoice.Livemode, event.Created)
	}
	if event.Type != stripe.EventTypeInvoicePaid || invoice.Status != stripe.InvoiceStatusPaid {
		return rejectStripeWebhook("Stripe invoice.paid 状态无效")
	}
	if invoice.AmountRemaining != 0 || invoice.AmountPaidOffStripe != 0 ||
		invoice.CollectionMethod != stripe.InvoiceCollectionMethodChargeAutomatically ||
		(invoice.BillingReason != stripe.InvoiceBillingReasonSubscriptionCreate &&
			invoice.BillingReason != stripe.InvoiceBillingReasonSubscriptionCycle) {
		return rejectStripeWebhook("Stripe invoice.paid 结算方式或账单原因无效")
	}
	if subscriptionId == "" || invoice.Customer == nil || invoice.Customer.ID == "" {
		return rejectStripeWebhook("Stripe invoice.paid 缺少 Customer 或 Subscription")
	}
	var priceId string
	var quantity int64
	var unitAmountMinor int64
	var periodStart int64
	var periodEnd int64
	if invoice.Lines != nil {
		for _, line := range invoice.Lines.Data {
			if line == nil || line.Parent == nil ||
				line.Parent.Type != stripe.InvoiceLineItemParentTypeSubscriptionItemDetails ||
				line.Parent.SubscriptionItemDetails == nil || line.Parent.SubscriptionItemDetails.Proration ||
				line.Pricing == nil || line.Pricing.Type != stripe.InvoiceLineItemPricingTypePriceDetails ||
				line.Pricing.PriceDetails == nil || line.Pricing.PriceDetails.Price == nil ||
				line.Pricing.PriceDetails.Price.ID == "" || line.Period == nil {
				continue
			}
			lineSubscriptionId := strings.TrimSpace(line.Parent.SubscriptionItemDetails.Subscription)
			if lineSubscriptionId == "" || lineSubscriptionId != subscriptionId {
				continue
			}
			if priceId != "" {
				return rejectStripeWebhook("Stripe Invoice 包含多个订阅 Price")
			}
			unitAmountDecimal := line.Pricing.UnitAmountDecimal
			if line.Quantity != 1 || unitAmountDecimal <= 0 || math.IsNaN(unitAmountDecimal) ||
				math.IsInf(unitAmountDecimal, 0) || unitAmountDecimal != math.Trunc(unitAmountDecimal) ||
				unitAmountDecimal >= float64(math.MaxInt64) ||
				!strings.EqualFold(string(line.Currency), string(invoice.Currency)) {
				return rejectStripeWebhook("Stripe Invoice 订阅行的数量、单价或币种无效")
			}
			priceId = line.Pricing.PriceDetails.Price.ID
			quantity = line.Quantity
			unitAmountMinor = int64(unitAmountDecimal)
			periodStart = line.Period.Start
			periodEnd = line.Period.End
		}
	}
	if priceId == "" || quantity != 1 || unitAmountMinor <= 0 || periodStart <= 0 || periodEnd <= periodStart {
		return rejectStripeWebhook("Stripe Invoice 缺少有效的订阅行项目")
	}
	if invoice.Total != order.ExpectedAmountMinor {
		return rejectStripeWebhook("Stripe Invoice 总额与订阅订单不匹配")
	}
	payload, err := common.Marshal(stripeInvoiceAuditSnapshot{
		InvoiceId: invoice.ID, CustomerId: invoice.Customer.ID, SubscriptionId: subscriptionId,
		ProductId: priceId, Quantity: quantity, UnitAmountMinor: unitAmountMinor,
		InvoiceTotalMinor: invoice.Total, AmountPaidMinor: invoice.AmountPaid, AmountRemainingMinor: invoice.AmountRemaining,
		Currency: strings.ToUpper(string(invoice.Currency)), Livemode: invoice.Livemode,
		PeriodStart: periodStart, PeriodEnd: periodEnd, EventCreated: event.Created,
	})
	if err != nil {
		return err
	}
	return model.CompleteStripeSubscriptionInvoice(model.StripeInvoiceSettlementInput{
		InvoiceId: invoice.ID, TradeNo: tradeNo, CustomerId: invoice.Customer.ID,
		SubscriptionId: subscriptionId, ProductId: priceId, Quantity: quantity,
		UnitAmountMinor: unitAmountMinor, InvoiceTotalMinor: invoice.Total, AmountPaidMinor: invoice.AmountPaid,
		Currency: string(invoice.Currency), Livemode: invoice.Livemode,
		PeriodStart: periodStart, PeriodEnd: periodEnd, EventCreated: event.Created, ProviderPayload: string(payload),
	})
}

func processStripeSubscriptionLifecycle(event stripe.Event, subscription *stripe.Subscription) error {
	if subscription == nil || subscription.ID == "" || subscription.Customer == nil || subscription.Customer.ID == "" {
		return rejectStripeWebhook("Stripe Subscription 生命周期事件数据无效")
	}
	if subscription.Livemode != event.Livemode {
		return rejectStripeWebhook("Stripe Subscription livemode 不匹配")
	}
	if event.Created <= 0 {
		return rejectStripeWebhook("Stripe Subscription 生命周期事件缺少创建时间")
	}
	tradeNo := strings.TrimSpace(subscription.Metadata["trade_no"])
	order := model.GetStripeSubscriptionOrderByProviderSubscriptionId(subscription.ID)
	if order == nil && tradeNo != "" {
		order = model.GetSubscriptionOrderByTradeNo(tradeNo)
		if order != nil {
			if err := order.BindStripeSubscription(subscription.Customer.ID, subscription.ID, subscription.Livemode); err != nil {
				return err
			}
		}
	}
	if order == nil {
		return model.ErrSubscriptionOrderNotFound
	}
	return model.UpdateStripeSubscriptionLifecycle(
		subscription.ID,
		subscription.Customer.ID,
		string(subscription.Status),
		subscription.Livemode,
		event.Created,
		event.Type == stripe.EventTypeCustomerSubscriptionDeleted,
	)
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(ctx context.Context, referenceId string, customerId string, email string, amount int64, successURL string, cancelURL string) (*stripe.CheckoutSession, error) {
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
				Quantity: stripe.Int64(amount),
			},
		},
		Mode:                stripe.String(string(stripe.CheckoutSessionModePayment)),
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

func stripeTopUpAmountMinor(amount int64, group string) (int64, error) {
	payMoney := getStripePayMoney(float64(amount), group)
	if math.IsNaN(payMoney) || math.IsInf(payMoney, 0) || payMoney <= 0 {
		return 0, errors.New("invalid Stripe top-up amount")
	}
	minor := decimal.NewFromFloat(payMoney).Mul(decimal.NewFromInt(100)).Round(0)
	if !minor.IsPositive() || minor.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, errors.New("Stripe top-up amount is out of range")
	}
	return minor.IntPart(), nil
}

func GetChargedAmount(count float64, user model.User) float64 {
	topUpGroupRatio := common.GetTopupGroupRatio(user.Group)
	if topUpGroupRatio == 0 {
		topUpGroupRatio = 1
	}

	return count * topUpGroupRatio
}

func getStripePayMoney(amount float64, group string) float64 {
	originalAmount := amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = amount / common.QuotaPerUnit
	}
	// Using float64 for monetary calculations is acceptable here due to the small amounts involved
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	// apply optional preset discount by the original request amount (if configured), default 1.0
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	payMoney := amount * setting.StripeUnitPrice * topupGroupRatio * discount
	return payMoney
}

func getStripeMinTopup() int64 {
	minTopup := setting.StripeMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}
