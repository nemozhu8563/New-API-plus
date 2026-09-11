package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// SubscriptionRequestBalancePay is retained for clients using the legacy
// balance-payment route. Subscription checkout is currently provider-backed;
// return an explicit unsupported response instead of silently creating an
// entitlement without a payment record.
func SubscriptionRequestBalancePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req struct {
		PlanId int `json:"plan_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.PurchaseSubscriptionWithBalance(c.GetInt("id"), req.PlanId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
