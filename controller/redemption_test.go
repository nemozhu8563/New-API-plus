package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopUpKeepsLegacyNumericQuotaResponse(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	confirmPaymentComplianceForTest(t)

	user := &model.User{
		Username: "legacy-topup-response-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	redemption := &model.Redemption{
		Name:        "legacy-topup-response-code",
		Key:         "50000000000000000000000000000001",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       700,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(redemption).Error)

	recorder := performAffiliateControllerRequest(
		t,
		TopUp,
		http.MethodPost,
		"/api/user/topup",
		`{"key":"50000000000000000000000000000001"}`,
		user.Id,
		common.RoleCommonUser,
		nil,
	)
	response := decodeAffiliateControllerResponse[int](t, recorder)
	require.True(t, response.Success)
	assert.Equal(t, 700, response.Data)

	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, 700, storedUser.Quota)
}

func TestRedeemCodeReturnsTypedQuotaResult(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	confirmPaymentComplianceForTest(t)

	user := &model.User{
		Username: "typed-redeem-response-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	redemption := &model.Redemption{
		Name:        "typed-redeem-response-code",
		Key:         "50000000000000000000000000000002",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       900,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(redemption).Error)

	recorder := performAffiliateControllerRequest(
		t,
		RedeemCode,
		http.MethodPost,
		"/api/user/redeem",
		`{"key":"50000000000000000000000000000002"}`,
		user.Id,
		common.RoleCommonUser,
		nil,
	)
	response := decodeAffiliateControllerResponse[model.RedemptionResult](t, recorder)
	require.True(t, response.Success)
	assert.Equal(t, model.RedemptionBenefitQuota, response.Data.Type)
	assert.Equal(t, 900, response.Data.Quota)
}
