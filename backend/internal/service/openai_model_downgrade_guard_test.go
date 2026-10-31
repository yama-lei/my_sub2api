package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func newModelDowngradePoolAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"pool_mode": true},
	}
}

func TestModelDowngradeGuardModeDefaults(t *testing.T) {
	require.Equal(t, ModelDowngradeGuardReroll, newModelDowngradePoolAccount(1).GetModelDowngradeGuardMode())

	plain := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	require.Equal(t, ModelDowngradeGuardOff, plain.GetModelDowngradeGuardMode())
	plain.Extra = map[string]any{modelDowngradeGuardExtraKey: "failover"}
	require.Equal(t, ModelDowngradeGuardFailover, plain.GetModelDowngradeGuardMode())

	pool := newModelDowngradePoolAccount(2)
	pool.Extra = map[string]any{modelDowngradeGuardExtraKey: "off"}
	require.Equal(t, ModelDowngradeGuardOff, pool.GetModelDowngradeGuardMode())
	pool.Extra = map[string]any{modelDowngradeGuardExtraKey: "bogus"}
	require.Equal(t, ModelDowngradeGuardReroll, pool.GetModelDowngradeGuardMode())

	oauth := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{modelDowngradeGuardExtraKey: "reroll"}}
	require.Equal(t, ModelDowngradeGuardOff, oauth.GetModelDowngradeGuardMode())

	require.Equal(t, defaultModelDowngradeRerollMax, pool.GetModelDowngradeRerollMax())
	pool.Extra = map[string]any{modelDowngradeRerollMaxExtraKey: float64(99)}
	require.Equal(t, maxModelDowngradeRerollMax, pool.GetModelDowngradeRerollMax())
}

func TestModelDowngradeRerollRewritesSessionIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := newModelDowngradePoolAccount(7101)
	body := []byte(`{"model":"gpt-6.1-sol","prompt_cache_key":"pck-1","input":"hi"}`)
	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Request.Header.Set("session_id", "client-session")
		return c
	}

	// No downgrade seen yet: the request is untouched.
	reroll := resolveModelDowngradeReroll(newCtx(), account, body)
	require.False(t, reroll.active())
	require.Equal(t, body, reroll.applyBody(body))

	seed := modelDowngradeSessionSeed(newCtx(), body)
	require.Equal(t, "client-session", seed)
	require.Equal(t, 1, bumpModelDowngradeRerollGeneration(account.ID, seed))

	reroll = resolveModelDowngradeReroll(newCtx(), account, body)
	require.True(t, reroll.active())
	rewritten := reroll.applyBody(body)
	newKey := gjson.GetBytes(rewritten, "prompt_cache_key").String()
	require.NotEqual(t, "pck-1", newKey)
	require.Equal(t, rerolledSessionID("pck-1", 1), newKey)

	h := http.Header{}
	h.Set("session_id", "client-session")
	h.Set("conversation_id", "conv-1")
	reroll.applyHeaders(h)
	require.Equal(t, rerolledSessionID("client-session", 1), h.Get("session_id"))
	require.Equal(t, rerolledSessionID("conv-1", 1), h.Get("conversation_id"))
	require.Empty(t, h.Get("x-session-id"), "absent headers must not be invented")

	// Later turns of the same client session stay on the rerolled identity.
	again := resolveModelDowngradeReroll(newCtx(), account, body).applyBody(body)
	require.Equal(t, newKey, gjson.GetBytes(again, "prompt_cache_key").String())

	// A body without prompt_cache_key gets one injected from the session seed.
	bare := []byte(`{"model":"gpt-6.1-sol","input":"hi"}`)
	injected := resolveModelDowngradeReroll(newCtx(), account, bare).applyBody(bare)
	require.Equal(t, rerolledSessionID("client-session", 1), gjson.GetBytes(injected, "prompt_cache_key").String())
}

const modelDowngradeStream = "event: response.created\n" +
	`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.6-luna"}}` + "\n\n" +
	"event: response.output_text.delta\n" +
	`data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n" +
	"event: response.completed\n" +
	`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-luna","usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"

func newModelDowngradeStreamContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session_id", "stream-session")
	return c, recorder
}

func newModelDowngradeStreamResponse(stream string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}
}

func requireModelDowngradeFailover(t *testing.T, err error, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "want failover error, got %v", err)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, defaultModelDowngradeRerollMax, failoverErr.SameAccountRetryMax)
	require.True(t, failoverErr.RequestScopedTransient)
	require.Equal(t, GatewayFailureReason("upstream_model_downgrade"), failoverErr.Reason)
	require.NotContains(t, recorder.Body.String(), "hello", "no output may reach the client before failover")
}

func TestModelDowngradeGuardAbandonsPassthroughStream(t *testing.T) {
	account := newModelDowngradePoolAccount(7102)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	c, recorder := newModelDowngradeStreamContext()
	// The builder records the session seed before the stream is read.
	resolveModelDowngradeReroll(c, account, []byte(`{"model":"gpt-6.1-sol"}`))

	_, err := svc.handleStreamingResponsePassthrough(context.Background(), newModelDowngradeStreamResponse(modelDowngradeStream), c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	requireModelDowngradeFailover(t, err, recorder)
	require.Equal(t, 1, modelDowngradeRerollGeneration(account.ID, "stream-session"))
	require.Empty(t, observedUpstreamResponseModel(c), "retry must start with a clean observer")
}

func TestModelDowngradeGuardAbandonsNativeStream(t *testing.T) {
	account := newModelDowngradePoolAccount(7103)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}, toolCorrector: NewCodexToolCorrector()}
	c, recorder := newModelDowngradeStreamContext()
	resolveModelDowngradeReroll(c, account, []byte(`{"model":"gpt-6.1-sol"}`))

	_, err := svc.handleStreamingResponse(context.Background(), newModelDowngradeStreamResponse(modelDowngradeStream), c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	requireModelDowngradeFailover(t, err, recorder)
	require.Equal(t, 1, modelDowngradeRerollGeneration(account.ID, "stream-session"))
}

func TestModelDowngradeGuardPassesMatchingModel(t *testing.T) {
	account := newModelDowngradePoolAccount(7104)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	c, recorder := newModelDowngradeStreamContext()
	stream := strings.ReplaceAll(modelDowngradeStream, "gpt-5.6-luna", "gpt-6.1-sol")

	_, err := svc.handleStreamingResponsePassthrough(context.Background(), newModelDowngradeStreamResponse(stream), c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), "hello")
	require.Equal(t, 0, modelDowngradeRerollGeneration(account.ID, "stream-session"))
}

func TestModelDowngradeGuardOffForNonPoolAccount(t *testing.T) {
	account := &Account{ID: 7105, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	c, recorder := newModelDowngradeStreamContext()

	_, err := svc.handleStreamingResponsePassthrough(context.Background(), newModelDowngradeStreamResponse(modelDowngradeStream), c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), "hello")
}

func TestModelDowngradeFailoverModeSwitchesAccount(t *testing.T) {
	account := newModelDowngradePoolAccount(7106)
	account.Extra = map[string]any{modelDowngradeGuardExtraKey: ModelDowngradeGuardFailover}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	c, _ := newModelDowngradeStreamContext()

	_, err := svc.handleStreamingResponsePassthrough(context.Background(), newModelDowngradeStreamResponse(modelDowngradeStream), c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.True(t, failoverErr.ShouldRetryNextAccount())
	require.Equal(t, 0, modelDowngradeRerollGeneration(account.ID, "stream-session"))
}
