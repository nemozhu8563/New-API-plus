package controller

import (
 "net/http"
 "github.com/gin-gonic/gin"
)

// SubscriptionRequestBalancePay is retained for clients using the legacy
// balance-payment route. Subscription checkout is currently provider-backed;
// return an explicit unsupported response instead of silently creating an
// entitlement without a payment record.
func SubscriptionRequestBalancePay(c *gin.Context) {
 c.JSON(http.StatusNotImplemented, gin.H{"success": false, "message": "balance payment is not supported"})
}
