package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func newJSONContext(method, path, body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestGetAndValidateTextRequestRejectsImageModelOnChatCompletions(t *testing.T) {
	c := newJSONContext(http.MethodPost, "/v1/chat/completions", `{
		"model": "gpt-image-1",
		"messages": [{"role": "user", "content": "draw a cat"}]
	}`)

	_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
	if err == nil {
		t.Fatal("expected image generation model to be rejected on chat completions")
	}
	if !strings.Contains(err.Error(), "must use /v1/images/generations") {
		t.Fatalf("expected endpoint guidance error, got %v", err)
	}
}

func TestGetAndValidateTextRequestAllowsTextModelOnChatCompletions(t *testing.T) {
	c := newJSONContext(http.MethodPost, "/v1/chat/completions", `{
		"model": "gpt-5.4",
		"messages": [{"role": "user", "content": "hello"}]
	}`)

	_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
	if err != nil {
		t.Fatalf("expected text model to pass chat validation, got %v", err)
	}
}

func TestGetAndValidateResponsesRequestRejectsImageModel(t *testing.T) {
	c := newJSONContext(http.MethodPost, "/v1/responses", `{
		"model": "flux-1.1-pro",
		"input": [{"role": "user", "content": "draw a cat"}]
	}`)

	_, err := GetAndValidateResponsesRequest(c)
	if err == nil {
		t.Fatal("expected image generation model to be rejected on responses")
	}
	if !strings.Contains(err.Error(), "must use /v1/images/generations") {
		t.Fatalf("expected endpoint guidance error, got %v", err)
	}
}
