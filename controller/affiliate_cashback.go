package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type affiliateAmountRequest struct {
	AmountQuota int64 `json:"amount_quota" binding:"required"`
}

type affiliateWithdrawalRequest struct {
	AmountQuota int64  `json:"amount_quota" binding:"required"`
	Note        string `json:"note"`
}

type affiliateAgentConfigRequest struct {
	Enabled               bool `json:"enabled"`
	CommissionRateBps     int  `json:"commission_rate_bps"`
	CashWithdrawalEnabled bool `json:"cash_withdrawal_enabled"`
}

type affiliateWithdrawalReviewRequest struct {
	AdminNote string `json:"admin_note"`
}

func setAffiliatePageResult(c *gin.Context, pageInfo *common.PageInfo, items any, total int64) {
	pageInfo.SetItems(items)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}

func GetAffiliateSummary(c *gin.Context) {
	summary, err := model.GetAffiliateSummary(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetAffiliateCommissions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAffiliateCommissionsForAgent(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func GetAffiliateInvitees(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAffiliateInvitees(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func GetAffiliateRedemptions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAffiliateRedemptionsForInviter(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func ConvertAffiliateCashback(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var request affiliateAmountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	conversion, err := model.ConvertAffiliateCashbackToBalance(c.GetInt("id"), request.AmountQuota)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, conversion)
}

func CreateAffiliateWithdrawal(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var request affiliateWithdrawalRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	withdrawal, err := model.CreateAffiliateWithdrawal(c.GetInt("id"), request.AmountQuota, request.Note)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, withdrawal)
}

func GetAffiliateWithdrawals(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAffiliateWithdrawalsForAgent(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func GetAffiliateConversions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAffiliateConversionsForAgent(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func parseAffiliateUserId(c *gin.Context) (int, bool) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return 0, false
	}
	return userId, true
}

func canManageAffiliateUser(c *gin.Context, userId int) bool {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if !canManageTargetRole(c.GetInt("role"), user.Role) {
		common.ApiErrorMsg(c, "无权管理该账号")
		return false
	}
	return true
}

func AdminGetAffiliateAgent(c *gin.Context) {
	userId, ok := parseAffiliateUserId(c)
	if !ok || !canManageAffiliateUser(c, userId) {
		return
	}
	agent, err := model.GetAffiliateAgentForAdmin(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agent)
}

func AdminUpdateAffiliateAgent(c *gin.Context) {
	userId, ok := parseAffiliateUserId(c)
	if !ok || !canManageAffiliateUser(c, userId) {
		return
	}

	var request affiliateAgentConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	agent, err := model.UpsertAffiliateAgentConfig(
		userId,
		request.Enabled,
		request.CommissionRateBps,
		request.CashWithdrawalEnabled,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userId, "affiliate.agent.update", map[string]interface{}{
		"enabled":                 request.Enabled,
		"commission_rate_bps":     request.CommissionRateBps,
		"cash_withdrawal_enabled": request.CashWithdrawalEnabled,
	})
	common.ApiSuccess(c, agent)
}

func AdminListAffiliateAgents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAffiliateAgents(c.Query("keyword"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func AdminListAffiliateCommissions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAllAffiliateCommissions(c.Query("keyword"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func AdminListAffiliateInvitations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAllAffiliateInvitations(c.Query("keyword"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func AdminListAffiliateRedemptions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAllAffiliateRedemptions(c.Query("keyword"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func AdminListAffiliateConversions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAllAffiliateConversions(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func AdminListAffiliateWithdrawals(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAllAffiliateWithdrawals(c.Query("status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setAffiliatePageResult(c, pageInfo, records, total)
}

func reviewAffiliateWithdrawal(c *gin.Context, paid bool) {
	withdrawalId, err := strconv.Atoi(c.Param("id"))
	if err != nil || withdrawalId <= 0 {
		common.ApiErrorMsg(c, "无效的提现申请 ID")
		return
	}

	withdrawal, err := model.GetAffiliateWithdrawalById(withdrawalId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageAffiliateUser(c, withdrawal.AgentUserId) {
		return
	}

	var request affiliateWithdrawalReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.ReviewAffiliateWithdrawal(
		withdrawalId,
		c.GetInt("id"),
		paid,
		request.AdminNote,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	action := "affiliate.withdrawal.reject"
	if paid {
		action = "affiliate.withdrawal.pay"
	}
	recordManageAuditFor(c, withdrawal.AgentUserId, action, map[string]interface{}{
		"withdrawal_id": withdrawalId,
		"amount_quota":  withdrawal.AmountQuota,
		"admin_note":    request.AdminNote,
	})
	common.ApiSuccess(c, updated)
}

func AdminPayAffiliateWithdrawal(c *gin.Context) {
	reviewAffiliateWithdrawal(c, true)
}

func AdminRejectAffiliateWithdrawal(c *gin.Context) {
	reviewAffiliateWithdrawal(c, false)
}
