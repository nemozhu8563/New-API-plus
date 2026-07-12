package openai

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func IsEventStreamResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.HasPrefix(contentType, "text/event-stream")
}

func jsonResponseFromUpstream(resp *http.Response) *http.Response {
	if resp == nil {
		return nil
	}
	jsonResp := *resp
	jsonResp.Header = resp.Header.Clone()
	jsonResp.Header.Set("Content-Type", "application/json")
	jsonResp.Header.Del("Transfer-Encoding")
	jsonResp.Header.Del("Connection")
	jsonResp.Header.Del("X-Accel-Buffering")
	return &jsonResp
}

func scanBufferedEventStream(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, handleData func(string) error) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("invalid response")
	}
	if handleData == nil {
		return fmt.Errorf("invalid stream data handler")
	}
	if info.StreamStatus == nil {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), helper.DefaultMaxScannerBufferSize)

	activity := make(chan struct{}, 1)
	scanDone := make(chan error, 1)

	go func() {
		var dataLines []string
		flushEvent := func() (bool, error) {
			if len(dataLines) == 0 {
				return false, nil
			}
			data := strings.TrimSpace(strings.Join(dataLines, "\n"))
			dataLines = nil
			if data == "" {
				return false, nil
			}
			if strings.HasPrefix(data, "[DONE]") {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
				return true, nil
			}

			info.SetFirstResponseTime()
			info.ReceivedResponseCount++
			if err := handleData(data); err != nil {
				info.StreamStatus.RecordError(err.Error())
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonHandlerStop, err)
				return false, err
			}
			return false, nil
		}

		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("buffered stream scanner panic: %v", r)
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, err)
				scanDone <- err
			}
		}()

		for scanner.Scan() {
			select {
			case activity <- struct{}{}:
			default:
			}

			if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				scanDone <- c.Request.Context().Err()
				return
			}

			line := strings.TrimRight(scanner.Text(), "\r")
			if line == "" {
				done, err := flushEvent()
				if err != nil || done {
					scanDone <- err
					return
				}
				continue
			}

			switch {
			case strings.HasPrefix(line, "data:"):
				data := line[5:]
				if strings.HasPrefix(data, " ") {
					data = data[1:]
				}
				dataLines = append(dataLines, data)
			case strings.HasPrefix(line, "[DONE]"):
				done, err := flushEvent()
				if err != nil || done {
					scanDone <- err
					return
				}
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
				scanDone <- nil
				return
			default:
				continue
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, err)
			scanDone <- err
			return
		}
		if done, err := flushEvent(); err != nil || done {
			scanDone <- err
			return
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
		scanDone <- nil
	}()

	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	timer := time.NewTimer(streamingTimeout)
	defer timer.Stop()

	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(streamingTimeout)
	}
	closeResponseBodySilently := func() {
		body := resp.Body
		resp.Body = nil
		if body != nil {
			_ = body.Close()
		}
	}
	waitScannerAfterClose := func() {
		select {
		case <-scanDone:
		case <-time.After(5 * time.Second):
			logger.LogError(c, "timeout waiting for buffered stream scanner to exit")
		}
	}
	logEnd := func() {
		if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
			logger.LogInfo(c, fmt.Sprintf("buffered stream ended: %s", info.StreamStatus.Summary()))
		} else {
			logger.LogError(c, fmt.Sprintf("buffered stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
		}
	}

	var requestDone <-chan struct{}
	if c != nil && c.Request != nil {
		requestDone = c.Request.Context().Done()
	}

	for {
		select {
		case err := <-scanDone:
			logEnd()
			return err
		case <-activity:
			resetTimer()
		case <-timer.C:
			err := fmt.Errorf("buffered event stream timed out after %s", streamingTimeout)
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonTimeout, err)
			closeResponseBodySilently()
			waitScannerAfterClose()
			logEnd()
			return err
		case <-requestDone:
			err := c.Request.Context().Err()
			if err == nil {
				err = fmt.Errorf("client disconnected")
			}
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
			closeResponseBodySilently()
			waitScannerAfterClose()
			logEnd()
			return err
		}
	}
}

func openAIErrorFromStreamData(data string) *types.OpenAIError {
	var simpleResponse dto.SimpleResponse
	if err := common.UnmarshalJsonStr(data, &simpleResponse); err != nil {
		return nil
	}
	oaiError := simpleResponse.GetOpenAIError()
	if oaiError == nil || oaiError.Type == "" {
		return nil
	}
	return oaiError
}

type bufferedToolCall struct {
	id        string
	callType  any
	name      string
	arguments strings.Builder
}

type bufferedChatChoice struct {
	role         string
	content      strings.Builder
	reasoning    strings.Builder
	finishReason string
	toolCalls    map[int]*bufferedToolCall
}

type bufferedChatCompletion struct {
	id      string
	created int64
	model   string
	choices map[int]*bufferedChatChoice
}

func newBufferedChatCompletion(model string) *bufferedChatCompletion {
	return &bufferedChatCompletion{
		model:   model,
		choices: make(map[int]*bufferedChatChoice),
	}
}

func (b *bufferedChatCompletion) choice(index int) *bufferedChatChoice {
	choice := b.choices[index]
	if choice == nil {
		choice = &bufferedChatChoice{
			role:      "assistant",
			toolCalls: make(map[int]*bufferedToolCall),
		}
		b.choices[index] = choice
	}
	return choice
}

func (b *bufferedChatCompletion) addChunk(chunk *dto.ChatCompletionsStreamResponse) {
	if chunk == nil {
		return
	}
	if chunk.Id != "" {
		b.id = chunk.Id
	}
	if chunk.Created != 0 {
		b.created = chunk.Created
	}
	if chunk.Model != "" {
		b.model = chunk.Model
	}
	for _, streamChoice := range chunk.Choices {
		choice := b.choice(streamChoice.Index)
		if streamChoice.Delta.Role != "" {
			choice.role = streamChoice.Delta.Role
		}
		if content := streamChoice.Delta.GetContentString(); content != "" {
			choice.content.WriteString(content)
		}
		if reasoning := streamChoice.Delta.GetReasoningContent(); reasoning != "" {
			choice.reasoning.WriteString(reasoning)
		}
		if streamChoice.FinishReason != nil {
			choice.finishReason = *streamChoice.FinishReason
		}

		for ordinal, tool := range streamChoice.Delta.ToolCalls {
			toolIndex := ordinal
			if tool.Index != nil {
				toolIndex = *tool.Index
			}
			bufferedTool := choice.toolCalls[toolIndex]
			if bufferedTool == nil {
				bufferedTool = &bufferedToolCall{}
				choice.toolCalls[toolIndex] = bufferedTool
			}
			if tool.ID != "" {
				bufferedTool.id = tool.ID
			}
			if tool.Type != nil {
				bufferedTool.callType = tool.Type
			}
			if tool.Function.Name != "" {
				bufferedTool.name = tool.Function.Name
			}
			if tool.Function.Arguments != "" {
				bufferedTool.arguments.WriteString(tool.Function.Arguments)
			}
		}
	}
}

func (b *bufferedChatCompletion) toResponse(fallbackID string, fallbackCreated int64, usage dto.Usage) *dto.OpenAITextResponse {
	id := b.id
	if id == "" {
		id = fallbackID
	}
	created := b.created
	if created == 0 {
		created = fallbackCreated
	}
	model := b.model

	choiceIndexes := make([]int, 0, len(b.choices))
	for index := range b.choices {
		choiceIndexes = append(choiceIndexes, index)
	}
	sort.Ints(choiceIndexes)
	if len(choiceIndexes) == 0 {
		choiceIndexes = append(choiceIndexes, 0)
		b.choice(0)
	}

	choices := make([]dto.OpenAITextResponseChoice, 0, len(choiceIndexes))
	for _, choiceIndex := range choiceIndexes {
		choice := b.choices[choiceIndex]
		role := choice.role
		if role == "" {
			role = "assistant"
		}
		message := dto.Message{
			Role:    role,
			Content: choice.content.String(),
		}
		if reasoning := choice.reasoning.String(); reasoning != "" {
			message.ReasoningContent = &reasoning
		}

		toolIndexes := make([]int, 0, len(choice.toolCalls))
		for index := range choice.toolCalls {
			toolIndexes = append(toolIndexes, index)
		}
		sort.Ints(toolIndexes)
		if len(toolIndexes) > 0 {
			toolCalls := make([]dto.ToolCallResponse, 0, len(toolIndexes))
			for _, toolIndex := range toolIndexes {
				tool := choice.toolCalls[toolIndex]
				callType := tool.callType
				if callType == nil {
					callType = "function"
				}
				toolCalls = append(toolCalls, dto.ToolCallResponse{
					ID:   tool.id,
					Type: callType,
					Function: dto.FunctionResponse{
						Name:      tool.name,
						Arguments: tool.arguments.String(),
					},
				})
			}
			if toolCallsJSON, err := common.Marshal(toolCalls); err == nil {
				message.ToolCalls = toolCallsJSON
			}
		}

		finishReason := choice.finishReason
		if finishReason == "" {
			if len(toolIndexes) > 0 && choice.content.Len() == 0 {
				finishReason = "tool_calls"
			} else {
				finishReason = "stop"
			}
		}
		choices = append(choices, dto.OpenAITextResponseChoice{
			Index:        choiceIndex,
			Message:      message,
			FinishReason: finishReason,
		})
	}

	return &dto.OpenAITextResponse{
		Id:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: choices,
		Usage:   usage,
	}
}

func writeBufferedChatResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, chatResp *dto.OpenAITextResponse) (*dto.Usage, *types.NewAPIError) {
	if chatResp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("empty buffered chat response"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	var (
		responseBody []byte
		err          error
	)
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		claudeResp := service.ResponseOpenAI2Claude(chatResp, info)
		responseBody, err = common.Marshal(claudeResp)
	case types.RelayFormatGemini:
		geminiResp := service.ResponseOpenAI2Gemini(chatResp, info)
		responseBody, err = common.Marshal(geminiResp)
	default:
		responseBody, err = common.Marshal(chatResp)
	}
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, jsonResponseFromUpstream(resp), responseBody)
	return &chatResp.Usage, nil
}

type bufferedCompletionChoice struct {
	text         strings.Builder
	finishReason string
}

type bufferedCompletion struct {
	id      string
	created int64
	model   string
	choices map[int]*bufferedCompletionChoice
}

type completionsStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Text         string  `json:"text"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *dto.Usage `json:"usage,omitempty"`
}

func newBufferedCompletion(model string) *bufferedCompletion {
	return &bufferedCompletion{
		model:   model,
		choices: make(map[int]*bufferedCompletionChoice),
	}
}

func (b *bufferedCompletion) choice(index int) *bufferedCompletionChoice {
	choice := b.choices[index]
	if choice == nil {
		choice = &bufferedCompletionChoice{}
		b.choices[index] = choice
	}
	return choice
}

func (b *bufferedCompletion) addChunk(chunk *completionsStreamChunk) bool {
	if chunk == nil {
		return false
	}
	if chunk.ID != "" {
		b.id = chunk.ID
	}
	if chunk.Created != 0 {
		b.created = chunk.Created
	}
	if chunk.Model != "" {
		b.model = chunk.Model
	}
	terminalFinish := false
	for _, streamChoice := range chunk.Choices {
		choice := b.choice(streamChoice.Index)
		if streamChoice.Text != "" {
			choice.text.WriteString(streamChoice.Text)
		}
		if streamChoice.FinishReason != nil {
			choice.finishReason = *streamChoice.FinishReason
			terminalFinish = true
		}
	}
	return terminalFinish
}

func (b *bufferedCompletion) toResponse(fallbackID string, fallbackCreated int64, usage dto.Usage) map[string]any {
	id := b.id
	if id == "" {
		id = fallbackID
	}
	created := b.created
	if created == 0 {
		created = fallbackCreated
	}
	model := b.model

	choiceIndexes := make([]int, 0, len(b.choices))
	for index := range b.choices {
		choiceIndexes = append(choiceIndexes, index)
	}
	sort.Ints(choiceIndexes)
	if len(choiceIndexes) == 0 {
		choiceIndexes = append(choiceIndexes, 0)
		b.choice(0)
	}

	choices := make([]map[string]any, 0, len(choiceIndexes))
	for _, choiceIndex := range choiceIndexes {
		choice := b.choices[choiceIndex]
		finishReason := choice.finishReason
		if finishReason == "" {
			finishReason = "stop"
		}
		choices = append(choices, map[string]any{
			"text":          choice.text.String(),
			"index":         choiceIndex,
			"finish_reason": finishReason,
		})
	}

	return map[string]any{
		"id":      id,
		"object":  "text_completion",
		"created": created,
		"model":   model,
		"choices": choices,
		"usage":   usage,
	}
}

func writeBufferedCompletionResponse(c *gin.Context, resp *http.Response, completionResp map[string]any, usage *dto.Usage) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := common.Marshal(completionResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, jsonResponseFromUpstream(resp), responseBody)
	return usage, nil
}

func emptyCreated(created any) bool {
	switch value := created.(type) {
	case nil:
		return true
	case int:
		return value == 0
	case int64:
		return value == 0
	case int32:
		return value == 0
	case float64:
		return value == 0
	case float32:
		return value == 0
	default:
		return false
	}
}

func OaiStreamBufferedHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	if info.RelayMode == relayconstant.RelayModeCompletions {
		return oaiCompletionsStreamBufferedHandler(c, info, resp)
	}

	buffered := newBufferedChatCompletion(info.UpstreamModelName)
	var (
		usage              = &dto.Usage{}
		containStreamUsage bool
		responseText       strings.Builder
		toolCount          int
		streamErr          *types.NewAPIError
		terminalFinish     bool
	)

	err := scanBufferedEventStream(c, info, resp, func(data string) error {
		if oaiError := openAIErrorFromStreamData(data); oaiError != nil {
			streamErr = types.WithOpenAIError(*oaiError, http.StatusInternalServerError)
			return streamErr
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			return err
		}
		if chunk.Usage != nil && service.ValidUsage(chunk.Usage) {
			usage = chunk.Usage
			containStreamUsage = true
		}
		buffered.addChunk(&chunk)
		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil {
				terminalFinish = true
				break
			}
		}
		if err := ProcessStreamResponse(chunk, &responseText, &toolCount); err != nil {
			logger.LogError(c, "error processing buffered stream tokens: "+err.Error())
		}
		return nil
	})
	if streamErr != nil {
		return nil, streamErr
	}
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if info.ReceivedResponseCount == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream event stream ended without data"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if info.StreamStatus.EndReason != relaycommon.StreamEndReasonDone && !terminalFinish {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream event stream ended before a terminal chat completion event"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	if !containStreamUsage {
		usage = service.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		usage.CompletionTokens += toolCount * 7
	}

	applyUsagePostProcessing(info, usage, nil)
	chatResp := buffered.toResponse(helper.GetResponseID(c), time.Now().Unix(), *usage)
	if chatResp.Model == "" {
		chatResp.Model = info.UpstreamModelName
	}
	return writeBufferedChatResponse(c, info, resp, chatResp)
}

func oaiCompletionsStreamBufferedHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	buffered := newBufferedCompletion(info.UpstreamModelName)
	var (
		usage              = &dto.Usage{}
		containStreamUsage bool
		responseText       strings.Builder
		streamErr          *types.NewAPIError
		terminalFinish     bool
	)

	err := scanBufferedEventStream(c, info, resp, func(data string) error {
		if oaiError := openAIErrorFromStreamData(data); oaiError != nil {
			streamErr = types.WithOpenAIError(*oaiError, http.StatusInternalServerError)
			return streamErr
		}
		var chunk completionsStreamChunk
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			return err
		}
		if chunk.Usage != nil && service.ValidUsage(chunk.Usage) {
			usage = chunk.Usage
			containStreamUsage = true
		}
		if buffered.addChunk(&chunk) {
			terminalFinish = true
		}
		for _, choice := range chunk.Choices {
			responseText.WriteString(choice.Text)
		}
		return nil
	})
	if streamErr != nil {
		return nil, streamErr
	}
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if info.ReceivedResponseCount == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream event stream ended without data"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if info.StreamStatus.EndReason != relaycommon.StreamEndReasonDone && !terminalFinish {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream event stream ended before a terminal completion event"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	if !containStreamUsage {
		usage = service.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	applyUsagePostProcessing(info, usage, nil)
	completionResp := buffered.toResponse(helper.GetResponseID(c), time.Now().Unix(), *usage)
	return writeBufferedCompletionResponse(c, resp, completionResp, usage)
}

type bufferedResponsesStream struct {
	completedResponse    *dto.OpenAIResponsesResponse
	completedResponseRaw json.RawMessage
	usage                *dto.Usage
	outputText           strings.Builder
	usageText            strings.Builder
	toolCallIndex        map[string]int
	toolCallName         map[string]string
	toolCallArgs         map[string]*strings.Builder
	toolCallItemID       map[string]string
	sawData              bool
}

func newBufferedResponsesStream() *bufferedResponsesStream {
	return &bufferedResponsesStream{
		usage:          &dto.Usage{},
		toolCallIndex:  make(map[string]int),
		toolCallName:   make(map[string]string),
		toolCallArgs:   make(map[string]*strings.Builder),
		toolCallItemID: make(map[string]string),
	}
}

func (b *bufferedResponsesStream) applyUsage(respUsage *dto.Usage) {
	if respUsage == nil {
		return
	}
	if respUsage.InputTokens != 0 {
		b.usage.PromptTokens = respUsage.InputTokens
		b.usage.InputTokens = respUsage.InputTokens
	}
	if respUsage.OutputTokens != 0 {
		b.usage.CompletionTokens = respUsage.OutputTokens
		b.usage.OutputTokens = respUsage.OutputTokens
	}
	if respUsage.TotalTokens != 0 {
		b.usage.TotalTokens = respUsage.TotalTokens
	} else {
		b.usage.TotalTokens = b.usage.PromptTokens + b.usage.CompletionTokens
	}
	if respUsage.InputTokensDetails != nil {
		b.usage.PromptTokensDetails.CachedTokens = respUsage.InputTokensDetails.CachedTokens
		b.usage.PromptTokensDetails.ImageTokens = respUsage.InputTokensDetails.ImageTokens
		b.usage.PromptTokensDetails.AudioTokens = respUsage.InputTokensDetails.AudioTokens
	}
	if respUsage.CompletionTokenDetails.ReasoningTokens != 0 {
		b.usage.CompletionTokenDetails.ReasoningTokens = respUsage.CompletionTokenDetails.ReasoningTokens
	}
}

func (b *bufferedResponsesStream) addToolCall(callID string, name string, argsDelta string) {
	if callID == "" {
		return
	}
	if _, ok := b.toolCallIndex[callID]; !ok {
		b.toolCallIndex[callID] = len(b.toolCallIndex)
	}
	if name != "" {
		b.toolCallName[callID] = name
	}
	if argsDelta != "" {
		builder := b.toolCallArgs[callID]
		if builder == nil {
			builder = &strings.Builder{}
			b.toolCallArgs[callID] = builder
		}
		builder.WriteString(argsDelta)
		b.usageText.WriteString(argsDelta)
	}
	if name != "" {
		b.usageText.WriteString(name)
	}
}

func (b *bufferedResponsesStream) toolCallArgsString(callID string) string {
	builder := b.toolCallArgs[callID]
	if builder == nil {
		return ""
	}
	return builder.String()
}

func (b *bufferedResponsesStream) addEvent(c *gin.Context, info *relaycommon.RelayInfo, streamResp *dto.ResponsesStreamResponse) {
	if streamResp == nil {
		return
	}
	b.sawData = true
	switch streamResp.Type {
	case "response.output_text.delta":
		if streamResp.Delta != "" {
			b.outputText.WriteString(streamResp.Delta)
			b.usageText.WriteString(streamResp.Delta)
		}
	case "response.output_item.added", "response.output_item.done":
		if streamResp.Item == nil {
			return
		}
		switch streamResp.Item.Type {
		case "function_call":
			itemID := strings.TrimSpace(streamResp.Item.ID)
			callID := strings.TrimSpace(streamResp.Item.CallId)
			if callID == "" {
				callID = itemID
			}
			if itemID != "" && callID != "" {
				b.toolCallItemID[itemID] = callID
			}
			name := strings.TrimSpace(streamResp.Item.Name)
			args := streamResp.Item.ArgumentsString()
			prev := b.toolCallArgsString(callID)
			argsDelta := ""
			if args != "" {
				if strings.HasPrefix(args, prev) {
					argsDelta = args[len(prev):]
				} else {
					argsDelta = args
				}
			}
			b.addToolCall(callID, name, argsDelta)
		case dto.BuildInCallWebSearchCall:
			if streamResp.Type == dto.ResponsesOutputTypeItemDone && info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
				if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
					webSearchTool.CallCount++
				}
			}
		}
	case "response.function_call_arguments.delta":
		itemID := strings.TrimSpace(streamResp.ItemID)
		callID := b.toolCallItemID[itemID]
		if callID == "" {
			callID = itemID
		}
		b.addToolCall(callID, "", streamResp.Delta)
	case "response.completed":
		if streamResp.Response != nil {
			b.completedResponse = streamResp.Response
			b.applyUsage(streamResp.Response.Usage)
			if streamResp.Response.HasImageGenerationCall() {
				c.Set("image_generation_call", true)
				c.Set("image_generation_call_quality", streamResp.Response.GetQuality())
				c.Set("image_generation_call_size", streamResp.Response.GetSize())
			}
		}
	case "response.error", "response.failed":
		// Let the caller convert this into a relay error after unmarshalling.
	default:
	}
}

func (b *bufferedResponsesStream) toolCalls() []dto.ToolCallResponse {
	callIDs := make([]string, 0, len(b.toolCallIndex))
	for callID := range b.toolCallIndex {
		callIDs = append(callIDs, callID)
	}
	sort.Slice(callIDs, func(i, j int) bool {
		return b.toolCallIndex[callIDs[i]] < b.toolCallIndex[callIDs[j]]
	})

	toolCalls := make([]dto.ToolCallResponse, 0, len(callIDs))
	for _, callID := range callIDs {
		toolCalls = append(toolCalls, dto.ToolCallResponse{
			ID:   callID,
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      b.toolCallName[callID],
				Arguments: b.toolCallArgsString(callID),
			},
		})
	}
	return toolCalls
}

func bufferResponsesEventStream(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*bufferedResponsesStream, *types.NewAPIError) {
	buffered := newBufferedResponsesStream()
	err := scanBufferedEventStream(c, info, resp, func(data string) error {
		var streamResp dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResp); err != nil {
			return err
		}
		if streamResp.Type == "response.completed" {
			var rawEvent struct {
				Response json.RawMessage `json:"response"`
			}
			if err := common.UnmarshalJsonStr(data, &rawEvent); err != nil {
				return err
			}
			if len(rawEvent.Response) > 0 && string(rawEvent.Response) != "null" {
				buffered.completedResponseRaw = append(buffered.completedResponseRaw[:0], rawEvent.Response...)
			}
		}
		if streamResp.Type == "response.error" || streamResp.Type == "response.failed" {
			if streamResp.Response != nil {
				if oaiErr := streamResp.Response.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
					return types.WithOpenAIError(*oaiErr, http.StatusInternalServerError)
				}
			}
			return fmt.Errorf("responses stream error: %s", streamResp.Type)
		}
		buffered.addEvent(c, info, &streamResp)
		return nil
	})
	if err != nil {
		var newAPIError *types.NewAPIError
		if errors.As(err, &newAPIError) {
			return nil, newAPIError
		}
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if !buffered.sawData {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream event stream ended without data"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	return buffered, nil
}

func OaiResponsesStreamBufferedHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	buffered, newAPIError := bufferResponsesEventStream(c, info, resp)
	if newAPIError != nil {
		return nil, newAPIError
	}
	if buffered.completedResponse == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream responses stream ended without a completed response"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	responseBody := []byte(buffered.completedResponseRaw)
	if len(responseBody) == 0 {
		var err error
		responseBody, err = common.Marshal(buffered.completedResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
	}
	service.IOCopyBytesGracefully(c, jsonResponseFromUpstream(resp), responseBody)

	if buffered.completedResponse.Usage != nil {
		buffered.applyUsage(buffered.completedResponse.Usage)
	}
	if buffered.usage.TotalTokens == 0 {
		buffered.usage = service.ResponseText2Usage(c, buffered.usageText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	return buffered.usage, nil
}

func OaiResponsesToChatStreamBufferedHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	buffered, newAPIError := bufferResponsesEventStream(c, info, resp)
	if newAPIError != nil {
		return nil, newAPIError
	}
	if buffered.completedResponse == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream responses stream ended without a completed response"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	var (
		chatResp *dto.OpenAITextResponse
		usage    *dto.Usage
		err      error
	)
	chatResp, usage, err = service.ResponsesResponseToChatCompletionsResponse(buffered.completedResponse, helper.GetResponseID(c))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if usage == nil || usage.TotalTokens == 0 {
		usage = buffered.usage
	}

	if usage == nil || usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, buffered.usageText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	chatResp.Usage = *usage
	if chatResp.Model == "" {
		chatResp.Model = info.UpstreamModelName
	}
	if emptyCreated(chatResp.Created) {
		chatResp.Created = time.Now().Unix()
	}
	if chatResp.Id == "" {
		chatResp.Id = helper.GetResponseID(c)
	}

	applyUsagePostProcessing(info, usage, nil)
	chatResp.Usage = *usage
	return writeBufferedChatResponse(c, info, resp, chatResp)
}
