package controller

import (
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"net/http"
)

func prepareRelayFirstAttempt(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam, promptTokens int, meta *types.TokenCountMeta) (*model.Channel, hosttypes.PriceData, *types.NewAPIError) {
	ch, err := getChannel(c, info, retryParam)
	if err != nil {
		logger.LogError(c, err.Error())
		return nil, hosttypes.PriceData{}, err
	}
	price, e := helper.ModelPriceHelper(c, info, promptTokens, meta)
	if e != nil {
		return nil, hosttypes.PriceData{}, types.NewError(e, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	return ch, price, nil
}
func newContentPolicyViolationError() *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New("request blocked by content policy"), types.ErrorCodeContentPolicyViolation, http.StatusForbidden, types.ErrOptionWithSkipRetry())
}
func checkPromptSensitivePolicy(c *gin.Context, text string) *types.NewAPIError {
	r := service.CheckSensitiveTextPolicy(text)
	if !r.Matched || r.Action == service.SensitiveWordActionAudit {
		return nil
	}
	logger.LogWarn(c, fmt.Sprintf("sensitive word policy hit: category=%s action=%s word=%q", r.Category, r.Action, r.Word))
	return newContentPolicyViolationError()
}
