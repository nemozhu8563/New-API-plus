package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionCheckoutRejectsActiveSubscriptionAcrossPaymentProviders(t *testing.T) {
	testCases := []struct {
		name      string
		handler   gin.HandlerFunc
		body      string
		configure func(*model.SubscriptionPlan)
	}{
		{
			name:    "Creem",
			handler: SubscriptionRequestCreemPay,
			body:    `{"plan_id":1}`,
			configure: func(plan *model.SubscriptionPlan) {
				plan.CreemProductId = "prod_subscription_placeholder"
				setting.CreemWebhookSecret = "creem_webhook_placeholder"
			},
		},
		{
			name:    "Epay",
			handler: SubscriptionRequestEpay,
			body:    `{"plan_id":1,"payment_method":"alipay"}`,
		},
		{
			name:    "Waffo Pancake",
			handler: SubscriptionRequestWaffoPancakePay,
			body:    `{"plan_id":1}`,
			configure: func(plan *model.SubscriptionPlan) {
				plan.WaffoPancakeProductId = "product_subscription_placeholder"
				setting.WaffoPancakeMerchantID = "merchant_placeholder"
				setting.WaffoPancakePrivateKey = "private_key_placeholder"
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStripeCheckoutHandlerTest(t)
			originalCreemWebhookSecret := setting.CreemWebhookSecret
			originalWaffoMerchantID := setting.WaffoPancakeMerchantID
			originalWaffoPrivateKey := setting.WaffoPancakePrivateKey
			paymentSetting := operation_setting.GetPaymentSetting()
			originalComplianceConfirmed := paymentSetting.ComplianceConfirmed
			originalComplianceTermsVersion := paymentSetting.ComplianceTermsVersion
			t.Cleanup(func() {
				setting.CreemWebhookSecret = originalCreemWebhookSecret
				setting.WaffoPancakeMerchantID = originalWaffoMerchantID
				setting.WaffoPancakePrivateKey = originalWaffoPrivateKey
				paymentSetting.ComplianceConfirmed = originalComplianceConfirmed
				paymentSetting.ComplianceTermsVersion = originalComplianceTermsVersion
			})
			paymentSetting.ComplianceConfirmed = true
			paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

			user := &model.User{
				Id:       1015,
				Username: "active_subscription_purchase_guard",
				Email:    "user@example.test",
				Status:   common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(user).Error)
			plan := &model.SubscriptionPlan{
				Title:                   "Active plan",
				PriceAmount:             399,
				Currency:                model.SubscriptionCurrencyCNY,
				DurationUnit:            model.SubscriptionDurationDay,
				DurationValue:           28,
				QuotaResetPeriod:        model.SubscriptionResetCustom,
				QuotaResetCustomSeconds: 604800,
				Enabled:                 true,
			}
			if testCase.configure != nil {
				testCase.configure(plan)
			}
			require.NoError(t, db.Create(plan).Error)
			now := common.GetTimestamp()
			require.NoError(t, db.Create(&model.UserSubscription{
				UserId: user.Id, PlanId: plan.Id, Status: "active",
				StartTime: now - 60, EndTime: now + 3600,
			}).Error)

			response := invokeStripeCheckoutHandler(t, testCase.handler, user.Id, testCase.body)

			assert.Equal(t, "已有有效订阅，暂不支持变更套餐", response["message"])
			var orders int64
			require.NoError(t, db.Model(&model.SubscriptionOrder{}).Count(&orders).Error)
			assert.Zero(t, orders)
		})
	}
}
