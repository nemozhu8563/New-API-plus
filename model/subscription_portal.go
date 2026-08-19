package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

func GetLatestStripeCustomerIdForUser(userId int, livemode bool) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid user id")
	}

	var order SubscriptionOrder
	err := DB.Select("provider_customer_id").
		Where(
			"user_id = ? AND payment_provider = ? AND provider_livemode = ? AND provider_customer_id <> ? AND provider_subscription_id IS NOT NULL",
			userId,
			PaymentProviderStripe,
			livemode,
			"",
		).
		Order("complete_time DESC").
		Order("id DESC").
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(order.ProviderCustomerId), nil
}
