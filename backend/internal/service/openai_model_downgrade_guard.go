package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// Model downgrade guard for pooled upstream gateways (upstream is itself a
// sub2api-like relay that sticky-routes by session identity).
//
// Observed in production: a pooled upstream pins one of our sessions to an
// upstream account that silently serves a cheaper model (sent gpt-6.1-sol,
// response.created declares gpt-5.6-luna). Every later turn of that session
// stays degraded because the upstream keeps the sticky binding.
//
// The Responses stream declares response.model in response.created, before any
// visible output, so the attempt can be abandoned while failover is still safe.
// "reroll" mode then re-sends to the same account with every session identity
// (session_id / conversation_id / x-session-* headers and prompt_cache_key)
// replaced by a derived value, forcing the upstream to re-schedule. The derived
// identity is remembered per (account, client session) so later turns stay on
// the newly selected upstream account and keep its prompt cache warm.
const (
	ModelDowngradeGuardOff      = "off"
	ModelDowngradeGuardReroll   = "reroll"
	ModelDowngradeGuardFailover = "failover"

	modelDowngradeGuardExtraKey     = "model_downgrade_guard"
	modelDowngradeRerollMaxExtraKey = "model_downgrade_reroll_max"
	defaultModelDowngradeRerollMax  = 2
	maxModelDowngradeRerollMax      = 5

	modelDowngradeRerollTTL = 6 * time.Hour
)

// modelDowngradeRerolls maps accountID|client-session-seed -> reroll generation.
var modelDowngradeRerolls = gocache.New(modelDowngradeRerollTTL, 10*time.Minute)

// modelDowngradeSessionHeaders are the sticky identities a sub2api-compatible
// upstream reads (see explicitOpenAIHeaderSessionNames).
var modelDowngradeSessionHeaders = explicitOpenAIHeaderSessionNames

// GetModelDowngradeGuardMode returns the configured guard mode. Pool-mode
// OpenAI API-key accounts default to reroll; everything else defaults to off.
func (a *Account) GetModelDowngradeGuardMode() string {
	if a == nil || !a.IsOpenAI() || a.Type != AccountTypeAPIKey {
		return ModelDowngradeGuardOff
	}
	if a.Extra != nil {
		if raw, ok := a.Extra[modelDowngradeGuardExtraKey].(string); ok {
			switch mode := strings.ToLower(strings.TrimSpace(raw)); mode {
			case ModelDowngradeGuardOff, ModelDowngradeGuardReroll, ModelDowngradeGuardFailover:
				return mode
			}
		}
	}
	if a.IsPoolMode() {
		return ModelDowngradeGuardReroll
	}
	return ModelDowngradeGuardOff
}

// GetModelDowngradeRerollMax bounds same-account rerolls per request before
// the request fails over to another account.
func (a *Account) GetModelDowngradeRerollMax() int {
	if a == nil || a.Extra == nil {
		return defaultModelDowngradeRerollMax
	}
	raw, ok := a.Extra[modelDowngradeRerollMaxExtraKey]
	if !ok || raw == nil {
		return defaultModelDowngradeRerollMax
	}
	n := parsePoolModeRetryCount(raw)
	if n < 0 {
		return 0
	}
	if n > maxModelDowngradeRerollMax {
		return maxModelDowngradeRerollMax
	}
	return n
}

// modelDowngradeSessionSeed is the client's own sticky identity, computed from
// the inbound request so that rewritten outbound values never feed back in.
func modelDowngradeSessionSeed(c *gin.Context, body []byte) string {
	seed := explicitOpenAIRequestSessionID(c, body)
	if seed == "" {
		seed = deriveOpenAIContentSessionSeed(body)
	}
	return seed
}

func modelDowngradeRerollKey(accountID int64, seed string) string {
	return strconv.FormatInt(accountID, 10) + "|" + seed
}

func modelDowngradeRerollGeneration(accountID int64, seed string) int {
	if seed == "" {
		return 0
	}
	if v, ok := modelDowngradeRerolls.Get(modelDowngradeRerollKey(accountID, seed)); ok {
		if n, ok := v.(int); ok {
			return n
		}
	}
	return 0
}

func bumpModelDowngradeRerollGeneration(accountID int64, seed string) int {
	if seed == "" {
		return 0
	}
	key := modelDowngradeRerollKey(accountID, seed)
	next := modelDowngradeRerollGeneration(accountID, seed) + 1
	modelDowngradeRerolls.Set(key, next, gocache.DefaultExpiration)
	return next
}

// rerolledSessionID derives a UUID-shaped identity so upstreams that validate
// the session header format still accept it.
func rerolledSessionID(value string, generation int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("sub2api-downgrade-reroll:%d:%s", generation, value)))
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// modelDowngradeSeedContextKey carries the client session seed from the request
// builder to the stream handler that detects the downgrade.
const modelDowngradeSeedContextKey = "openai_model_downgrade_seed"

// modelDowngradeReroll is the per-attempt rewrite resolved from the inbound request.
type modelDowngradeReroll struct {
	generation int
	seed       string
}

// resolveModelDowngradeReroll computes the client session seed from the
// outbound-to-be body (before any rewrite), remembers it on the gin context and
// returns the rewrite for the current reroll generation.
func resolveModelDowngradeReroll(c *gin.Context, account *Account, body []byte) modelDowngradeReroll {
	if account == nil || account.GetModelDowngradeGuardMode() == ModelDowngradeGuardOff {
		return modelDowngradeReroll{}
	}
	seed := modelDowngradeSessionSeed(c, body)
	if c != nil {
		c.Set(modelDowngradeSeedContextKey, seed)
	}
	if account.GetModelDowngradeGuardMode() != ModelDowngradeGuardReroll {
		return modelDowngradeReroll{seed: seed}
	}
	return modelDowngradeReroll{generation: modelDowngradeRerollGeneration(account.ID, seed), seed: seed}
}

func (r modelDowngradeReroll) active() bool { return r.generation > 0 }

// applyBody rewrites prompt_cache_key. When the client sent no explicit
// identity at all, the upstream falls back to a content hash, so a fresh
// prompt_cache_key is injected to give it something new to route on.
func (r modelDowngradeReroll) applyBody(body []byte) []byte {
	if !r.active() || len(body) == 0 {
		return body
	}
	current := strings.TrimSpace(openAIRequestPayloadView(body).Get("prompt_cache_key").String())
	if current == "" {
		current = r.seed
	}
	if current == "" {
		return body
	}
	updated, err := sjson.SetBytes(body, "prompt_cache_key", rerolledSessionID(current, r.generation))
	if err != nil {
		return body
	}
	return updated
}

func (r modelDowngradeReroll) applyHeaders(h http.Header) {
	if !r.active() || h == nil {
		return
	}
	for _, name := range modelDowngradeSessionHeaders {
		if v := strings.TrimSpace(h.Get(name)); v != "" {
			h.Set(name, rerolledSessionID(v, r.generation))
		}
	}
}

// SetAccountQualityService wires the recorder for guard-abandoned attempts.
func (s *OpenAIGatewayService) SetAccountQualityService(value *AccountQualityService) {
	s.accountQuality = value
}

// checkModelDowngrade returns a failover error when the upstream declared a
// different model than the one sent, the account has the guard enabled, and no
// client-visible output has been written yet.
func (s *OpenAIGatewayService) checkModelDowngrade(
	c *gin.Context,
	account *Account,
	sentModel string,
	observedModel string,
	passthrough bool,
	upstreamRequestID string,
) *UpstreamFailoverError {
	if c == nil || account == nil || strings.TrimSpace(observedModel) == "" || strings.TrimSpace(sentModel) == "" {
		return nil
	}
	mode := account.GetModelDowngradeGuardMode()
	if mode == ModelDowngradeGuardOff {
		return nil
	}
	if upstreamModelsMatchForAudit(sentModel, observedModel) {
		return nil
	}

	// The observer outlives this attempt on the gin context; start the retry clean
	// so the served attempt's usage log reports its own response model.
	beginUpstreamResponseModelObservation(c)

	message := fmt.Sprintf("Upstream served %s instead of requested %s", observedModel, sentModel)
	generation := 0
	if mode == ModelDowngradeGuardReroll {
		generation = bumpModelDowngradeRerollGeneration(account.ID, c.GetString(modelDowngradeSeedContextKey))
	}
	s.recordOpenAIStreamUpstreamError(c, account, passthrough, upstreamRequestID, "model_downgrade", nil, message)
	logger.L().Warn("openai.model_downgrade_detected",
		zap.Int64("account_id", account.ID),
		zap.String("mode", mode),
		zap.String("sent_model", sentModel),
		zap.String("response_model", observedModel),
		zap.Int("reroll_generation", generation),
		zap.String("upstream_request_id", upstreamRequestID),
	)
	s.accountQuality.RecordDowngrade(account.ID, sentModel, observedModel, mode, generation)

	errBody := []byte(fmt.Sprintf(`{"error":{"type":"upstream_error","code":"model_downgrade","message":%q}}`, message))
	failoverErr := &UpstreamFailoverError{
		StatusCode:             http.StatusBadGateway,
		ResponseBody:           errBody,
		RequestScopedTransient: true,
		Stage:                  GatewayFailureStageInference,
		Scope:                  GatewayFailureScopeRequest,
		Reason:                 "upstream_model_downgrade",
		ClientStatusCode:       http.StatusBadGateway,
		ClientMessage:          message,
	}
	if mode == ModelDowngradeGuardReroll {
		if max := account.GetModelDowngradeRerollMax(); max > 0 {
			failoverErr.RetryableOnSameAccount = true
			failoverErr.SameAccountRetryMax = max
		}
	}
	return failoverErr
}

// observeModelDowngradeEvent checks the model declared by one upstream stream
// payload (response.created / chat chunk). The payload, not the shared observer,
// is consulted because the observer may still hold an earlier attempt's model.
func (s *OpenAIGatewayService) observeModelDowngradeEvent(
	c *gin.Context,
	account *Account,
	payload []byte,
	sentModel string,
	passthrough bool,
	upstreamRequestID string,
) (checked bool, failoverErr *UpstreamFailoverError) {
	declared := firstValidTrimmedGJSONString(payload, "response.model", "model")
	if declared == "" {
		return false, nil
	}
	return true, s.checkModelDowngrade(c, account, sentModel, declared, passthrough, upstreamRequestID)
}
