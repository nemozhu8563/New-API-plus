package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/vertex"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyClaudeAssistantPrefillContinuationAppendsForStringContent(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model: "claude-opus-4-6",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "search Grok"},
			{Role: "assistant", Content: "I will check that."},
		},
	}

	appended := applyClaudeAssistantPrefillContinuation(req)

	require.True(t, appended)
	require.Len(t, req.Messages, 3)
	require.Equal(t, "assistant", req.Messages[1].Role)
	require.Equal(t, "I will check that.", req.Messages[1].Content)
	require.Equal(t, "user", req.Messages[2].Role)
	require.Equal(t, claudeAssistantPrefillContinuationText, req.Messages[2].Content)
}

func TestApplyClaudeAssistantPrefillContinuationAppendsForTextBlocks(t *testing.T) {
	part1 := "continue "
	part2 := "this"
	req := &dto.ClaudeRequest{
		Model: "claude-opus-4-7-high",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "start"},
			{Role: "assistant", Content: []dto.ClaudeMediaMessage{
				{Type: dto.ContentTypeText, Text: &part1},
				{Type: dto.ContentTypeText, Text: &part2},
			}},
		},
	}

	appended := applyClaudeAssistantPrefillContinuation(req)

	require.True(t, appended)
	require.Len(t, req.Messages, 3)
	require.Equal(t, "assistant", req.Messages[1].Role)
	require.Equal(t, []dto.ClaudeMediaMessage{
		{Type: dto.ContentTypeText, Text: &part1},
		{Type: dto.ContentTypeText, Text: &part2},
	}, req.Messages[1].Content)
}

func TestApplyClaudeAssistantPrefillContinuationSkipsIneligibleRequests(t *testing.T) {
	text := "plain"
	tests := []struct {
		name string
		req  dto.ClaudeRequest
	}{
		{
			name: "older Claude model",
			req: dto.ClaudeRequest{
				Model: "claude-opus-4-5-20251101",
				Messages: []dto.ClaudeMessage{
					{Role: "user", Content: "start"},
					{Role: "assistant", Content: "plain"},
				},
			},
		},
		{
			name: "already user final",
			req: dto.ClaudeRequest{
				Model: "claude-opus-4-6",
				Messages: []dto.ClaudeMessage{
					{Role: "assistant", Content: "plain"},
					{Role: "user", Content: "continue"},
				},
			},
		},
		{
			name: "native tool use block",
			req: dto.ClaudeRequest{
				Model: "claude-opus-4-6",
				Messages: []dto.ClaudeMessage{
					{Role: "user", Content: "start"},
					{Role: "assistant", Content: []dto.ClaudeMediaMessage{
						{Type: "tool_use", Id: "toolu_1", Name: "search"},
					}},
				},
			},
		},
		{
			name: "native thinking block",
			req: dto.ClaudeRequest{
				Model: "claude-opus-4-6",
				Messages: []dto.ClaudeMessage{
					{Role: "user", Content: "start"},
					{Role: "assistant", Content: []dto.ClaudeMediaMessage{
						{Type: "thinking", Thinking: &text},
						{Type: dto.ContentTypeText, Text: &text},
					}},
				},
			},
		},
		{
			name: "empty messages",
			req: dto.ClaudeRequest{
				Model:    "claude-opus-4-6",
				Messages: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := mustMarshalClaudeRequest(t, &tt.req)
			appended := applyClaudeAssistantPrefillContinuation(&tt.req)

			require.False(t, appended)
			require.JSONEq(t, string(before), string(mustMarshalClaudeRequest(t, &tt.req)))
		})
	}
}

func TestApplyClaudeAssistantPrefillCompatibilityRecordsMarkerForConvertedClaudeRequest(t *testing.T) {
	original := &dto.ClaudeRequest{
		Model: "claude-opus-4-6",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "search"},
			{Role: "assistant", Content: "checking"},
		},
	}
	info := &relaycommon.RelayInfo{}

	outbound := cloneClaudeRequest(t, original)
	appended := applyClaudeAssistantPrefillCompatibility(outbound, info)

	require.True(t, appended)
	require.Len(t, outbound.Messages, 3)
	require.Len(t, original.Messages, 2)
	require.Equal(t, "assistant_prefill_continuation", info.RequestConversionMeta[0])
}

func TestApplyClaudeAssistantPrefillCompatibilitySkipsNilRequest(t *testing.T) {
	info := &relaycommon.RelayInfo{}

	appended := applyClaudeAssistantPrefillCompatibility(nil, info)

	require.False(t, appended)
	require.Empty(t, info.RequestConversionMeta)
}

func TestApplyClaudeAssistantPrefillCompatibilityBeforeWrappedVertexRequest(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model: "claude-opus-4-6",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "start"},
			{Role: "assistant", Content: "continue"},
		},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-4-6"}}

	appended := applyClaudeAssistantPrefillCompatibility(req, info)
	require.True(t, appended)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	converted, err := (&vertex.Adaptor{}).ConvertClaudeRequest(c, info, req)
	require.NoError(t, err)

	data, err := common.Marshal(converted)
	require.NoError(t, err)
	var body struct {
		Messages []dto.ClaudeMessage `json:"messages"`
	}
	require.NoError(t, common.Unmarshal(data, &body))
	require.Len(t, body.Messages, 3)
	require.Equal(t, "assistant", body.Messages[1].Role)
	require.Equal(t, "user", body.Messages[2].Role)
	require.Equal(t, claudeAssistantPrefillContinuationText, body.Messages[2].Content)
}

func TestBuildClaudeRequestBodyTransformsNonPassthroughWireBodyOnly(t *testing.T) {
	c := newClaudePrefillTestContext(nil)
	originalCountToken := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = originalCountToken })
	originalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = originalPassThrough })

	original := &dto.ClaudeRequest{
		Model: "claude-opus-4-6",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "start"},
			{Role: "assistant", Content: "continue"},
		},
	}
	request := cloneClaudeRequest(t, original)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}
	originalEstimate := estimateClaudeTokensForTest(t, c, original, info)
	info.SetEstimatePromptTokens(originalEstimate)

	body, closer, apiErr := buildClaudeRequestBody(c, info, &claude.Adaptor{}, request)
	if closer != nil {
		defer closer.Close()
	}

	require.Nil(t, apiErr)
	bodyBytes, err := io.ReadAll(body)
	require.NoError(t, err)
	var outbound dto.ClaudeRequest
	require.NoError(t, common.Unmarshal(bodyBytes, &outbound))
	require.Len(t, outbound.Messages, 3)
	require.Equal(t, "assistant", outbound.Messages[1].Role)
	require.Equal(t, "user", outbound.Messages[2].Role)
	require.Equal(t, claudeAssistantPrefillContinuationText, outbound.Messages[2].Content)
	require.Len(t, original.Messages, 2)
	require.Equal(t, "assistant_prefill_continuation", info.RequestConversionMeta[0])
	require.Greater(t, info.GetEstimatePromptTokens(), originalEstimate)
}

func TestBuildClaudeRequestBodyLeavesPassthroughBodyUnchanged(t *testing.T) {
	rawBody := []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"start"},{"role":"assistant","content":"continue"}]}`)
	c := newClaudePrefillTestContext(rawBody)
	originalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = originalPassThrough })
	request := &dto.ClaudeRequest{
		Model: "claude-opus-4-6",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "start"},
			{Role: "assistant", Content: "continue"},
		},
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}

	body, closer, apiErr := buildClaudeRequestBody(c, info, &claude.Adaptor{}, request)
	if closer != nil {
		defer closer.Close()
	}

	require.Nil(t, apiErr)
	bodyBytes, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(t, string(rawBody), string(bodyBytes))
	require.Len(t, request.Messages, 2)
	require.Empty(t, info.RequestConversionMeta)
}

func TestApplyClaudeAssistantPrefillContinuationProductionQwenPawShape(t *testing.T) {
	stream := true
	req := &dto.ClaudeRequest{
		Model:  "claude-opus-4-6",
		Stream: &stream,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "Grok 最新模型是什么？"},
			{Role: "assistant", Content: "我来帮你查一下 Grok 最新模型的信息。"},
		},
	}

	appended := applyClaudeAssistantPrefillContinuation(req)
	appendedAgain := applyClaudeAssistantPrefillContinuation(req)

	require.True(t, appended)
	require.False(t, appendedAgain)
	require.Len(t, req.Messages, 3)
	require.Equal(t, "user", req.Messages[0].Role)
	require.Equal(t, "assistant", req.Messages[1].Role)
	require.Equal(t, "我来帮你查一下 Grok 最新模型的信息。", req.Messages[1].Content)
	require.Equal(t, "user", req.Messages[2].Role)
	continuation, ok := req.Messages[2].Content.(string)
	require.True(t, ok)
	require.Equal(t, claudeAssistantPrefillContinuationText, continuation)
	require.NotContains(t, continuation, "Grok")
	require.NotContains(t, continuation, "QwenPaw")
	require.NotContains(t, continuation, "CoPaw")
	require.False(t, strings.Contains(continuation, req.Messages[1].Content.(string)))
}

func cloneClaudeRequest(t *testing.T, req *dto.ClaudeRequest) *dto.ClaudeRequest {
	t.Helper()
	data := mustMarshalClaudeRequest(t, req)
	var cloned dto.ClaudeRequest
	require.NoError(t, common.Unmarshal(data, &cloned))
	return &cloned
}

func mustMarshalClaudeRequest(t *testing.T, req *dto.ClaudeRequest) []byte {
	t.Helper()
	data, err := common.Marshal(req)
	require.NoError(t, err)
	return data
}

func newClaudePrefillTestContext(body []byte) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body == nil {
		body = []byte("{}")
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-opus-4-6")
	return c
}

func estimateClaudeTokensForTest(t *testing.T, c *gin.Context, req *dto.ClaudeRequest, info *relaycommon.RelayInfo) int {
	t.Helper()
	tokens, err := service.EstimateRequestToken(c, req.GetTokenCountMeta(), info)
	require.NoError(t, err)
	return tokens
}
