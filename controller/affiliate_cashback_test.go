package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type affiliateControllerResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type affiliateControllerPage[T any] struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
	Items    []T `json:"items"`
}

func setupAffiliateControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.Redemption{},
		&model.AffiliateAgent{},
		&model.AffiliateFirstRewardClaim{},
		&model.AffiliateCommission{},
		&model.AffiliateConversion{},
		&model.AffiliateWithdrawal{},
	))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		_ = sqlDB.Close()
	})
	return db
}

func performAffiliateControllerRequest(
	t *testing.T,
	handler gin.HandlerFunc,
	method string,
	target string,
	body string,
	userId int,
	role int,
	params map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", userId)
	context.Set("role", role)
	context.Set("username", "affiliate-test-operator")
	for key, value := range params {
		context.Params = append(context.Params, gin.Param{Key: key, Value: value})
	}
	handler(context)
	return recorder
}

func decodeAffiliateControllerResponse[T any](
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) affiliateControllerResponse[T] {
	t.Helper()
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response affiliateControllerResponse[T]
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func setAffiliateComplianceForControllerTest(t *testing.T, confirmed bool) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	previousConfirmed := paymentSetting.ComplianceConfirmed
	previousTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = previousConfirmed
		paymentSetting.ComplianceTermsVersion = previousTermsVersion
	})
	paymentSetting.ComplianceConfirmed = confirmed
	if confirmed {
		paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	} else {
		paymentSetting.ComplianceTermsVersion = ""
	}
}

func createAffiliateControllerUser(
	t *testing.T,
	db *gorm.DB,
	username string,
	role int,
	inviterId int,
) *model.User {
	t.Helper()
	user := &model.User{
		Username:  username,
		Password:  "password",
		Role:      role,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   username + "-code",
		InviterId: inviterId,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestConvertAffiliateCashbackAPIRequiresComplianceAndValidAmount(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	setAffiliateComplianceForControllerTest(t, false)
	user := createAffiliateControllerUser(t, db, "convert-api-user", common.RoleCommonUser, 0)
	require.NoError(t, db.Create(&model.AffiliateAgent{
		UserId:                user.Id,
		Enabled:               true,
		CommissionRateBps:     model.AffiliateAgentMaxRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        1_000,
		TotalCommissionQuota:  1_000,
	}).Error)

	recorder := performAffiliateControllerRequest(
		t,
		ConvertAffiliateCashback,
		http.MethodPost,
		"/api/user/affiliate/convert",
		`{"amount_quota":500}`,
		user.Id,
		common.RoleCommonUser,
		nil,
	)
	blocked := decodeAffiliateControllerResponse[model.AffiliateConversion](t, recorder)
	assert.False(t, blocked.Success)

	var agent model.AffiliateAgent
	require.NoError(t, db.First(&agent, "user_id = ?", user.Id).Error)
	assert.Equal(t, int64(1_000), agent.AvailableQuota)

	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	invalidBodies := []string{
		`{}`,
		`{"amount_quota":0}`,
		`{"amount_quota":-1}`,
		fmt.Sprintf(`{"amount_quota":%d}`, int64(common.MaxQuota)+1),
		`{"amount_quota":"500"}`,
	}
	for _, body := range invalidBodies {
		recorder = performAffiliateControllerRequest(
			t,
			ConvertAffiliateCashback,
			http.MethodPost,
			"/api/user/affiliate/convert",
			body,
			user.Id,
			common.RoleCommonUser,
			nil,
		)
		response := decodeAffiliateControllerResponse[model.AffiliateConversion](t, recorder)
		assert.False(t, response.Success, "body %s must be rejected", body)
	}

	var conversionCount int64
	require.NoError(t, db.Model(&model.AffiliateConversion{}).Count(&conversionCount).Error)
	assert.Zero(t, conversionCount)

	recorder = performAffiliateControllerRequest(
		t,
		ConvertAffiliateCashback,
		http.MethodPost,
		"/api/user/affiliate/convert",
		`{"amount_quota":500}`,
		user.Id,
		common.RoleCommonUser,
		nil,
	)
	converted := decodeAffiliateControllerResponse[model.AffiliateConversion](t, recorder)
	require.True(t, converted.Success)
	assert.Equal(t, int64(500), converted.Data.AmountQuota)

	require.NoError(t, db.First(&agent, "user_id = ?", user.Id).Error)
	assert.Equal(t, int64(500), agent.AvailableQuota)
	assert.Equal(t, int64(500), agent.ConvertedQuota)
	var updatedUser model.User
	require.NoError(t, db.First(&updatedUser, user.Id).Error)
	assert.Equal(t, 500, updatedUser.Quota)
}

func TestCreateAffiliateWithdrawalAPIReservesBalanceAndTrimsNote(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	setAffiliateComplianceForControllerTest(t, true)
	user := createAffiliateControllerUser(t, db, "withdraw-api-user", common.RoleCommonUser, 0)
	require.NoError(t, db.Create(&model.AffiliateAgent{
		UserId:                user.Id,
		Enabled:               true,
		CommissionRateBps:     model.AffiliateAgentMinRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        2_000,
		TotalCommissionQuota:  2_000,
	}).Error)

	recorder := performAffiliateControllerRequest(
		t,
		CreateAffiliateWithdrawal,
		http.MethodPost,
		"/api/user/affiliate/withdrawals",
		`{"amount_quota":750,"note":"  PayPal payout  "}`,
		user.Id,
		common.RoleCommonUser,
		nil,
	)
	response := decodeAffiliateControllerResponse[model.AffiliateWithdrawal](t, recorder)
	require.True(t, response.Success)
	assert.Equal(t, int64(750), response.Data.AmountQuota)
	assert.Equal(t, "PayPal payout", response.Data.ApplicantNote)
	assert.Equal(t, model.AffiliateWithdrawalStatusPending, response.Data.Status)

	var agent model.AffiliateAgent
	require.NoError(t, db.First(&agent, "user_id = ?", user.Id).Error)
	assert.Equal(t, int64(1_250), agent.AvailableQuota)
	assert.Equal(t, int64(750), agent.PendingWithdrawalQuota)
}

func TestAffiliateInviteeAPIUsesAuthenticatedUserScopeAndPagination(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	inviterA := createAffiliateControllerUser(t, db, "scope-inviter-a", common.RoleCommonUser, 0)
	inviterB := createAffiliateControllerUser(t, db, "scope-inviter-b", common.RoleCommonUser, 0)
	inviteeA1 := createAffiliateControllerUser(t, db, "scope-alice-one", common.RoleCommonUser, inviterA.Id)
	inviteeA2 := createAffiliateControllerUser(t, db, "scope-alice-two", common.RoleCommonUser, inviterA.Id)
	_ = createAffiliateControllerUser(t, db, "scope-bob-other", common.RoleCommonUser, inviterB.Id)

	recorder := performAffiliateControllerRequest(
		t,
		GetAffiliateInvitees,
		http.MethodGet,
		"/api/user/affiliate/invitees?p=1&page_size=1",
		"",
		inviterA.Id,
		common.RoleCommonUser,
		nil,
	)
	firstPageResponse := decodeAffiliateControllerResponse[affiliateControllerPage[model.AffiliateInviteeRecord]](t, recorder)
	require.True(t, firstPageResponse.Success)
	assert.Equal(t, 1, firstPageResponse.Data.Page)
	assert.Equal(t, 1, firstPageResponse.Data.PageSize)
	assert.Equal(t, 2, firstPageResponse.Data.Total)
	require.Len(t, firstPageResponse.Data.Items, 1)
	assert.Contains(t, firstPageResponse.Data.Items[0].Username, "***")

	recorder = performAffiliateControllerRequest(
		t,
		GetAffiliateInvitees,
		http.MethodGet,
		"/api/user/affiliate/invitees?p=2&page_size=1",
		"",
		inviterA.Id,
		common.RoleCommonUser,
		nil,
	)
	secondPageResponse := decodeAffiliateControllerResponse[affiliateControllerPage[model.AffiliateInviteeRecord]](t, recorder)
	require.True(t, secondPageResponse.Success)
	require.Len(t, secondPageResponse.Data.Items, 1)
	assert.ElementsMatch(
		t,
		[]int{inviteeA1.Id, inviteeA2.Id},
		[]int{
			firstPageResponse.Data.Items[0].UserId,
			secondPageResponse.Data.Items[0].UserId,
		},
	)
}

func TestAdminAffiliateAgentConfigEnforcesRoleHierarchy(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	adminOperator := createAffiliateControllerUser(t, db, "agent-admin", common.RoleAdminUser, 0)
	rootOperator := createAffiliateControllerUser(t, db, "agent-root", common.RoleRootUser, 0)
	commonTarget := createAffiliateControllerUser(t, db, "agent-common-target", common.RoleCommonUser, 0)
	adminTarget := createAffiliateControllerUser(t, db, "agent-admin-target", common.RoleAdminUser, 0)
	body := `{"enabled":true,"commission_rate_bps":750,"cash_withdrawal_enabled":true}`

	recorder := performAffiliateControllerRequest(
		t,
		AdminUpdateAffiliateAgent,
		http.MethodPut,
		"/api/affiliate/agents",
		body,
		adminOperator.Id,
		common.RoleAdminUser,
		map[string]string{"user_id": fmt.Sprint(commonTarget.Id)},
	)
	commonResponse := decodeAffiliateControllerResponse[model.AffiliateAgent](t, recorder)
	require.True(t, commonResponse.Success)
	assert.Equal(t, 750, commonResponse.Data.CommissionRateBps)

	recorder = performAffiliateControllerRequest(
		t,
		AdminUpdateAffiliateAgent,
		http.MethodPut,
		"/api/affiliate/agents",
		body,
		adminOperator.Id,
		common.RoleAdminUser,
		map[string]string{"user_id": fmt.Sprint(adminTarget.Id)},
	)
	adminDenied := decodeAffiliateControllerResponse[model.AffiliateAgent](t, recorder)
	assert.False(t, adminDenied.Success)
	assert.Contains(t, adminDenied.Message, "无权管理")
	var adminTargetAgentCount int64
	require.NoError(t, db.Model(&model.AffiliateAgent{}).
		Where("user_id = ?", adminTarget.Id).
		Count(&adminTargetAgentCount).Error)
	assert.Zero(t, adminTargetAgentCount)

	recorder = performAffiliateControllerRequest(
		t,
		AdminUpdateAffiliateAgent,
		http.MethodPut,
		"/api/affiliate/agents",
		body,
		rootOperator.Id,
		common.RoleRootUser,
		map[string]string{"user_id": fmt.Sprint(adminTarget.Id)},
	)
	rootResponse := decodeAffiliateControllerResponse[model.AffiliateAgent](t, recorder)
	require.True(t, rootResponse.Success)
	assert.Equal(t, adminTarget.Id, rootResponse.Data.UserId)
}

func TestAdminAffiliateAgentConfigRejectsInvalidTargetAndRate(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	adminOperator := createAffiliateControllerUser(t, db, "validation-admin", common.RoleAdminUser, 0)
	target := createAffiliateControllerUser(t, db, "validation-target", common.RoleCommonUser, 0)

	for _, userId := range []string{"0", "not-a-number", "999999"} {
		recorder := performAffiliateControllerRequest(
			t,
			AdminUpdateAffiliateAgent,
			http.MethodPut,
			"/api/affiliate/agents",
			`{"enabled":true,"commission_rate_bps":750,"cash_withdrawal_enabled":true}`,
			adminOperator.Id,
			common.RoleAdminUser,
			map[string]string{"user_id": userId},
		)
		response := decodeAffiliateControllerResponse[model.AffiliateAgent](t, recorder)
		assert.False(t, response.Success)
	}

	for _, rate := range []int{
		model.AffiliateAgentMinRateBps - 1,
		model.AffiliateAgentMaxRateBps + 1,
	} {
		recorder := performAffiliateControllerRequest(
			t,
			AdminUpdateAffiliateAgent,
			http.MethodPut,
			"/api/affiliate/agents",
			fmt.Sprintf(
				`{"enabled":true,"commission_rate_bps":%d,"cash_withdrawal_enabled":true}`,
				rate,
			),
			adminOperator.Id,
			common.RoleAdminUser,
			map[string]string{"user_id": fmt.Sprint(target.Id)},
		)
		response := decodeAffiliateControllerResponse[model.AffiliateAgent](t, recorder)
		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "5%")
	}

	var agentCount int64
	require.NoError(t, db.Model(&model.AffiliateAgent{}).
		Where("user_id = ?", target.Id).
		Count(&agentCount).Error)
	assert.Zero(t, agentCount)
}

func TestAdminWithdrawalReviewEnforcesRoleAndSingleDecision(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	adminOperator := createAffiliateControllerUser(t, db, "review-admin", common.RoleAdminUser, 0)
	rootOperator := createAffiliateControllerUser(t, db, "review-root", common.RoleRootUser, 0)
	adminAgent := createAffiliateControllerUser(t, db, "review-admin-agent", common.RoleAdminUser, 0)
	require.NoError(t, db.Create(&model.AffiliateAgent{
		UserId:                adminAgent.Id,
		Enabled:               true,
		CommissionRateBps:     model.AffiliateAgentMaxRateBps,
		CashWithdrawalEnabled: true,
		AvailableQuota:        1_000,
		TotalCommissionQuota:  1_000,
	}).Error)
	withdrawal, err := model.CreateAffiliateWithdrawal(adminAgent.Id, 1_000, "bank")
	require.NoError(t, err)

	params := map[string]string{"id": fmt.Sprint(withdrawal.Id)}
	recorder := performAffiliateControllerRequest(
		t,
		AdminPayAffiliateWithdrawal,
		http.MethodPost,
		"/api/affiliate/withdrawals/pay",
		`{"admin_note":"paid"}`,
		adminOperator.Id,
		common.RoleAdminUser,
		params,
	)
	denied := decodeAffiliateControllerResponse[model.AffiliateWithdrawal](t, recorder)
	assert.False(t, denied.Success)
	assert.Contains(t, denied.Message, "无权管理")

	var stored model.AffiliateWithdrawal
	require.NoError(t, db.First(&stored, withdrawal.Id).Error)
	assert.Equal(t, model.AffiliateWithdrawalStatusPending, stored.Status)

	recorder = performAffiliateControllerRequest(
		t,
		AdminPayAffiliateWithdrawal,
		http.MethodPost,
		"/api/affiliate/withdrawals/pay",
		`{"admin_note":"offline transfer complete"}`,
		rootOperator.Id,
		common.RoleRootUser,
		params,
	)
	paid := decodeAffiliateControllerResponse[model.AffiliateWithdrawal](t, recorder)
	require.True(t, paid.Success)
	assert.Equal(t, model.AffiliateWithdrawalStatusPaid, paid.Data.Status)
	assert.Equal(t, rootOperator.Id, paid.Data.ReviewerUserId)

	recorder = performAffiliateControllerRequest(
		t,
		AdminRejectAffiliateWithdrawal,
		http.MethodPost,
		"/api/affiliate/withdrawals/reject",
		`{"admin_note":"second decision"}`,
		rootOperator.Id,
		common.RoleRootUser,
		params,
	)
	repeated := decodeAffiliateControllerResponse[model.AffiliateWithdrawal](t, recorder)
	assert.False(t, repeated.Success)
	assert.Contains(t, repeated.Message, "已处理")

	var agent model.AffiliateAgent
	require.NoError(t, db.First(&agent, "user_id = ?", adminAgent.Id).Error)
	assert.Zero(t, agent.AvailableQuota)
	assert.Zero(t, agent.PendingWithdrawalQuota)
	assert.Equal(t, int64(1_000), agent.WithdrawnQuota)
}

func TestAdminWithdrawalListAPIRejectsUnknownStatus(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	admin := createAffiliateControllerUser(t, db, "list-status-admin", common.RoleAdminUser, 0)

	recorder := performAffiliateControllerRequest(
		t,
		AdminListAffiliateWithdrawals,
		http.MethodGet,
		"/api/affiliate/withdrawals?status=processing",
		"",
		admin.Id,
		common.RoleAdminUser,
		nil,
	)
	response := decodeAffiliateControllerResponse[affiliateControllerPage[model.AffiliateWithdrawalRecord]](t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "无效的提现状态")
}
