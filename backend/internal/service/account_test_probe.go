package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Background fingerprint probes (ModelTrace) reuse the account connection test
// so routing, auth, proxies and header overrides match the real account. The
// probe prompt rides on the gin context and replaces the "hi" test message.
const (
	accountTestProbePromptKey   = "account_test_probe_prompt"
	accountTestObservedModelKey = "account_test_observed_model"

	// accountTestProbeMaxTokens leaves room for ~330 integers plus separators
	// (same budget as the ModelTrace reference client).
	accountTestProbeMaxTokens = 4096
)

// AccountProbeRun is the outcome of one probe request.
type AccountProbeRun struct {
	Text          string
	ObservedModel string
	ErrorMessage  string
	LatencyMs     int64
}

// RunProbeBackground sends prompt to the account through the connection test
// path and returns the generated text.
func (s *AccountTestService) RunProbeBackground(ctx context.Context, accountID int64, modelID string, prompt string) AccountProbeRun {
	startedAt := time.Now()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = (&http.Request{}).WithContext(ctx)
	ginCtx.Set(accountTestBackgroundKey, true)
	ginCtx.Set(accountTestProbePromptKey, prompt)

	testErr := s.TestAccountConnection(ginCtx, accountID, modelID, prompt, AccountTestModeDefault)
	text, errMsg := parseTestSSEOutput(w.Body.String())
	if errMsg == "" && testErr != nil {
		errMsg = testErr.Error()
	}
	return AccountProbeRun{
		Text:          text,
		ObservedModel: ginCtx.GetString(accountTestObservedModelKey),
		ErrorMessage:  errMsg,
		LatencyMs:     time.Since(startedAt).Milliseconds(),
	}
}

func accountTestProbePrompt(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString(accountTestProbePromptKey))
}

// applyAccountTestProbeToResponsesPayload swaps the Responses test input for the
// probe prompt. API-key upstreams get no Codex instructions so the sampled
// distribution matches a plain API call, which is how the bank was collected.
func applyAccountTestProbeToResponsesPayload(c *gin.Context, payload map[string]any, isOAuth bool) {
	prompt := accountTestProbePrompt(c)
	if prompt == "" {
		return
	}
	payload["input"] = []map[string]any{{
		"role":    "user",
		"content": []map[string]any{{"type": "input_text", "text": prompt}},
	}}
	if !isOAuth {
		delete(payload, "instructions")
	}
}

// applyAccountTestProbeToClaudePayload swaps the Messages test input for the
// probe prompt and raises max_tokens.
func applyAccountTestProbeToClaudePayload(c *gin.Context, payload map[string]any) {
	prompt := accountTestProbePrompt(c)
	if prompt == "" {
		return
	}
	payload["messages"] = []map[string]any{{
		"role":    "user",
		"content": []map[string]any{{"type": "text", "text": prompt}},
	}}
	payload["max_tokens"] = accountTestProbeMaxTokens
}

// recordAccountTestObservedModel keeps the model the upstream declared.
func recordAccountTestObservedModel(c *gin.Context, model string) {
	if c == nil || strings.TrimSpace(model) == "" || c.GetString(accountTestObservedModelKey) != "" {
		return
	}
	c.Set(accountTestObservedModelKey, strings.TrimSpace(model))
}
