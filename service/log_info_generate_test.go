package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesRequestConversionMeta(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta:           &relaycommon.ChannelMeta{},
		RequestConversionMeta: []string{"assistant_prefill_continuation"},
	}

	other := GenerateTextOtherInfo(c, info, 0, 0, 0, 0, 0, 0, 0)

	require.Equal(t, []string{"assistant_prefill_continuation"}, other["request_conversion_meta"])
}
