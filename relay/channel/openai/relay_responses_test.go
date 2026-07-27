package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesHandlerAggregatesUnexpectedSSEForNonStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := `event: response.created
data: {"type":"response.created","response":{"id":"resp_test","object":"response","created_at":1,"status":"in_progress","model":"gpt-5.4","output":[]}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_test","object":"response","created_at":1,"status":"completed","model":"gpt-5.4","output":[],"usage":{"input_tokens":7,"output_tokens":5,"total_tokens":12}}}

`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := OaiResponsesHandler(c, &relaycommon.RelayInfo{}, resp)

	require.Nil(t, err)
	require.Equal(t, 7, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 12, usage.TotalTokens)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), `"id":"resp_test"`)
	require.NotContains(t, recorder.Body.String(), "event:")
}

type blockingReadCloser struct {
	closed chan struct{}
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func (b *blockingReadCloser) Read(_ []byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingReadCloser) Close() error {
	select {
	case <-b.closed:
	default:
		close(b.closed)
	}
	return nil
}

func TestScanBufferedEventStreamTimesOutBlockedUpstream(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := &relaycommon.RelayInfo{}
	body := newBlockingReadCloser()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	start := time.Now()
	handlerCalled := false
	err := scanBufferedEventStream(c, info, resp, func(string) error {
		handlerCalled = true
		return nil
	})

	require.Error(t, err)
	require.Less(t, time.Since(start), 3*time.Second)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonTimeout, info.StreamStatus.EndReason)
	require.False(t, handlerCalled)
}

func TestOaiResponsesStreamBufferedHandlerPreservesCompletedResponseRawJSON(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := `data: {"type":"response.completed","response":{"id":"resp_raw","object":"response","created_at":1,"status":"completed","model":"gpt-5.4","output":[],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3},"unknown_top_level":{"kept":true}}}

`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := OaiResponsesStreamBufferedHandler(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.4"},
	}, resp)

	require.Nil(t, err)
	require.Equal(t, 1, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"unknown_top_level":{"kept":true}`)
	require.Contains(t, recorder.Body.String(), `"id":"resp_raw"`)
}

func TestOaiResponsesStreamBufferedHandlerCountsCompletedImageOutputs(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"image_generation_call","id":"img_1","status":"completed","result":"base64-a"}}

data: {"type":"response.completed","response":{"id":"resp_image","object":"response","created_at":1,"status":"completed","model":"gpt-5.4","output":[{"type":"image_generation_call","id":"img_1","status":"completed","result":"base64-a"}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}

`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.4"},
	}

	usage, err := OaiResponsesStreamBufferedHandler(c, info, resp)

	require.Nil(t, err)
	require.Equal(t, 3, usage.TotalTokens)
	require.NotNil(t, info.ResponsesUsageInfo)
	require.NotNil(t, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration])
	require.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
}
