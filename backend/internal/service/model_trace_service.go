package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
)

// ModelTrace 检测参数：采集目标份数与最大尝试次数（与上游 test_automatic 一致）。
const (
	modelTraceTargetResponses = 3
	modelTraceMaxAttempts     = 6
	// modelTraceRequestTimeout 单次上游请求超时。长整数序列生成较慢，
	// 与上游 enrollment.py 的 urlopen timeout=240s 对齐。
	modelTraceRequestTimeout = 240 * time.Second
	// modelTraceExcerptChars 结果里附带的每份回答摘录长度。
	modelTraceExcerptChars = 120
)

// ModelTraceModelOption 是检测支持的模型选项。
type ModelTraceModelOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Family      string `json:"family"`
	FamilyName  string `json:"family_name"`
}

// ModelTraceAccountOption 是可用于检测的账号（key）选项。
type ModelTraceAccountOption struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Platform       string `json:"platform"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	CredentialType string `json:"credential_type"`
}

// ModelTraceOptionsResponse 是 GET /admin/model-trace/options 的响应。
type ModelTraceOptionsResponse struct {
	Models   []ModelTraceModelOption   `json:"models"`
	Accounts []ModelTraceAccountOption `json:"accounts"`
	BankInfo ModelTraceBankInfo        `json:"bank_info"`
}

// ModelTraceBankInfo 描述指纹库概况。
type ModelTraceBankInfo struct {
	BuiltAt             string `json:"built_at"`
	ModelCount          int    `json:"model_count"`
	RecommendedQueries  int    `json:"recommended_queries"`
	MinimumValidNumbers int    `json:"minimum_valid_numbers"`
}

// ModelTraceService 提供「模型真假检测」（ModelTrace 指纹归因）能力。
type ModelTraceService struct {
	accountRepo         AccountRepository
	httpUpstream        HTTPUpstream
	tlsFPProfileService *TLSFingerprintProfileService
	pluginManager       *PluginManager
}

// NewModelTraceService 创建 ModelTraceService。
func NewModelTraceService(
	accountRepo AccountRepository,
	httpUpstream HTTPUpstream,
	tlsFPProfileService *TLSFingerprintProfileService,
) *ModelTraceService {
	return &ModelTraceService{
		accountRepo:         accountRepo,
		httpUpstream:        httpUpstream,
		tlsFPProfileService: tlsFPProfileService,
	}
}

// SetPluginManager 注入插件管理器（OpenAI OAuth 能力绑定时走插件传输）。
func (s *ModelTraceService) SetPluginManager(pluginManager *PluginManager) {
	s.pluginManager = pluginManager
}

// Options 返回检测支持的模型与可用账号列表。
func (s *ModelTraceService) Options(ctx context.Context) (*ModelTraceOptionsResponse, error) {
	bank, err := modeltrace.LoadBank()
	if err != nil {
		return nil, err
	}
	resp := &ModelTraceOptionsResponse{
		Models: make([]ModelTraceModelOption, 0),
		Accounts: make([]ModelTraceAccountOption, 0),
		BankInfo: ModelTraceBankInfo{
			BuiltAt:             bank.BuiltAt,
			ModelCount:          len(bank.Models),
			RecommendedQueries:  modelTraceTargetResponses,
			MinimumValidNumbers: modeltrace.MinValidNumbers,
		},
	}
	for _, model := range bank.SupportedModels() {
		resp.Models = append(resp.Models, ModelTraceModelOption{
			ID:          model.ID,
			DisplayName: model.DisplayName,
			Family:      model.Family,
			FamilyName:  model.FamilyName,
		})
	}
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI} {
		accounts, err := s.accountRepo.ListByPlatform(ctx, platform)
		if err != nil {
			return nil, err
		}
		for i := range accounts {
			account := &accounts[i]
			if !modelTraceAccountEligible(account) {
				continue
			}
			resp.Accounts = append(resp.Accounts, ModelTraceAccountOption{
				ID:             account.ID,
				Name:           account.Name,
				Platform:       account.Platform,
				Type:           account.Type,
				Status:         account.Status,
				CredentialType: modelTraceCredentialLabel(account),
			})
		}
	}
	return resp, nil
}

// modelTraceAccountEligible 判断账号是否可用于检测：
// 仅支持 anthropic / openai 平台的 oauth（含 setup-token）与 apikey 类型账号，
// 即指纹库 GPT / Claude 两家族对应的直连账号形态。
func modelTraceAccountEligible(account *Account) bool {
	if account.IsSyntheticUITest() {
		return false
	}
	if account.Type != AccountTypeOAuth && account.Type != AccountTypeAPIKey && account.Type != AccountTypeSetupToken {
		return false
	}
	if account.IsBedrock() || account.Type == AccountTypeServiceAccount {
		return false
	}
	return account.Platform == PlatformAnthropic || account.Platform == PlatformOpenAI
}

func modelTraceCredentialLabel(account *Account) string {
	if account.Type == AccountTypeOAuth || account.Type == AccountTypeSetupToken {
		return "oauth"
	}
	return "api_key"
}

// ---------- SSE 事件 ----------

// ModelTraceEvent 是检测 SSE 事件。
type ModelTraceEvent struct {
	Type string `json:"type"` // start | progress | challenge_done | result | error | end
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

func sendModelTraceEvent(c *gin.Context, event ModelTraceEvent) {
	eventJSON, _ := json.Marshal(event)
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON); err != nil {
		return
	}
	c.Writer.Flush()
}

// Detect 执行一次完整检测并通过 SSE 推送进度与结果。
// 流程与上游 enrollment.test_automatic 一致：顺序发起挑战，
// 收集满 targetResponses 份有效回答后进行全局指纹归因。
func (s *ModelTraceService) Detect(c *gin.Context, accountID int64, modelID string) error {
	ctx := c.Request.Context()

	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("请选择要检测的模型")
	}
	bank, err := modeltrace.LoadBank()
	if err != nil {
		return err
	}
	if !bank.HasModel(modelID) {
		return fmt.Errorf("模型 %s 不在指纹库中，暂不支持检测", modelID)
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("账号不存在")
	}
	if !modelTraceAccountEligible(account) {
		return fmt.Errorf("账号 %s（%s/%s）不支持模型检测：仅支持 Anthropic / OpenAI 平台的 OAuth 或 API Key 账号", account.Name, account.Platform, account.Type)
	}

	// 与网关转发一致的模型映射（apikey 账号常配通配符映射）。
	upstreamModel := modelID
	if account.Type == AccountTypeAPIKey {
		upstreamModel = account.GetMappedModel(upstreamModel)
	}

	// SSE 头。
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	sendModelTraceEvent(c, ModelTraceEvent{Type: "start", Text: modelID})

	challenges, err := modeltrace.GenerateChallenges(modelTraceMaxAttempts)
	if err != nil {
		sendModelTraceEvent(c, ModelTraceEvent{Type: "error", Text: err.Error()})
		return nil
	}

	outputs := make([]modeltrace.Output, 0, modelTraceTargetResponses)
	responses := make([]modelTraceResponseReport, 0, modelTraceMaxAttempts)
	apiErrors := make([]string, 0)

	for i, challenge := range challenges {
		if len(outputs) >= modelTraceTargetResponses {
			break
		}
		sendModelTraceEvent(c, ModelTraceEvent{
			Type: "progress",
			Text: fmt.Sprintf("正在发送探针 %d/%d（要求 %d 个整数）…", i+1, len(challenges), challenge.ExpectedCount),
		})

		text, callErr := s.requestCompletion(ctx, account, upstreamModel, challenge.Prompt)
		if callErr != nil {
			apiErrors = append(apiErrors, fmt.Sprintf("探针 %d: %s", i+1, callErr.Error()))
			sendModelTraceEvent(c, ModelTraceEvent{Type: "progress", Text: fmt.Sprintf("探针 %d 失败：%s", i+1, callErr.Error())})
			continue
		}
		numbers := modeltrace.ParseNumbers(text)
		minimum := modeltrace.MinimumForExpected(challenge.ExpectedCount)
		accepted := len(numbers) >= minimum
		responses = append(responses, newResponseReport(challenge.ID, text, len(numbers), challenge.ExpectedCount, accepted))
		if !accepted {
			apiErrors = append(apiErrors, fmt.Sprintf("探针 %d: 有效数字不足 %d/%d", i+1, len(numbers), minimum))
			sendModelTraceEvent(c, ModelTraceEvent{Type: "progress", Text: fmt.Sprintf("探针 %d 有效数字不足（%d/%d），不计入", i+1, len(numbers), minimum)})
			continue
		}
		outputs = append(outputs, modeltrace.Output{Text: text, ExpectedCount: challenge.ExpectedCount})
		sendModelTraceEvent(c, ModelTraceEvent{
			Type: "challenge_done",
			Text: fmt.Sprintf("探针 %d 有效（解析到 %d 个数字）", i+1, len(numbers)),
			Data: map[string]any{"received": len(outputs), "target": modelTraceTargetResponses},
		})
	}

	if len(outputs) == 0 {
		sendModelTraceEvent(c, ModelTraceEvent{Type: "error", Text: "没有获得可用回答：" + strings.Join(apiErrors, "；")})
		return nil
	}

	attribution, err := modeltrace.AnalyzeOutputs(outputs, bank)
	if err != nil {
		sendModelTraceEvent(c, ModelTraceEvent{Type: "error", Text: err.Error()})
		return nil
	}

	result := buildModelTraceResult(account, modelID, upstreamModel, attribution, responses, apiErrors, modelTraceTargetResponses, len(challenges))
	sendModelTraceEvent(c, ModelTraceEvent{Type: "result", Data: result})
	sendModelTraceEvent(c, ModelTraceEvent{Type: "end"})
	return nil
}

// modelTraceResponseReport 是单份回答的采集报告（不含完整原文）。
type modelTraceResponseReport struct {
	ChallengeID   string `json:"challenge_id"`
	ExpectedCount int    `json:"expected_count"`
	ParsedNumbers int    `json:"parsed_numbers"`
	Accepted      bool   `json:"accepted"`
	Excerpt       string `json:"excerpt"`
}

func newResponseReport(challengeID, text string, parsed, expected int, accepted bool) modelTraceResponseReport {
	excerpt := []rune(strings.TrimSpace(text))
	if len(excerpt) > modelTraceExcerptChars {
		excerpt = excerpt[:modelTraceExcerptChars]
	}
	return modelTraceResponseReport{
		ChallengeID:   challengeID,
		ExpectedCount: expected,
		ParsedNumbers: parsed,
		Accepted:      accepted,
		Excerpt:       string(excerpt),
	}
}

// ModelTraceVerdict 是检测结论。
type ModelTraceVerdict struct {
	Status             string  `json:"status"` // match | family_mismatch | mismatch
	Confidence         string  `json:"confidence"`
	ClaimedModel       string  `json:"claimed_model"`
	PredictedModel     string  `json:"predicted_model"`
	ClaimedProbability float64 `json:"claimed_probability"`
}

// ModelTraceResult 是最终检测结果的负载。
type ModelTraceResult struct {
	Account    ModelTraceAccountOption    `json:"account"`
	Model      string                     `json:"model"`
	UpstreamModel string                  `json:"upstream_model,omitempty"`
	Attribution   *modeltrace.Attribution `json:"attribution"`
	Verdict       ModelTraceVerdict       `json:"verdict"`
	APITest       map[string]any          `json:"api_test"`
	Responses     []modelTraceResponseReport `json:"responses"`
}

func buildModelTraceResult(
	account *Account,
	claimedModel string,
	upstreamModel string,
	attribution *modeltrace.Attribution,
	responses []modelTraceResponseReport,
	apiErrors []string,
	requested int,
	attempted int,
) *ModelTraceResult {
	claimedProbability := 0.0
	for i := range attribution.Results {
		if attribution.Results[i].Model == claimedModel {
			claimedProbability = attribution.Results[i].Probability
			break
		}
	}

	status := "match"
	if attribution.Prediction != claimedModel {
		claimedFamily := ""
		for i := range attribution.Results {
			if attribution.Results[i].Model == claimedModel {
				claimedFamily = attribution.Results[i].Family
				break
			}
		}
		if claimedFamily != "" && claimedFamily == attribution.Results[0].Family {
			status = "family_mismatch"
		} else {
			status = "mismatch"
		}
	}

	confidence := "low"
	if attribution.Probability >= 0.6 {
		confidence = "high"
	} else if attribution.Probability >= 0.3 {
		confidence = "medium"
	}

	return &ModelTraceResult{
		Account: ModelTraceAccountOption{
			ID:       account.ID,
			Name:     account.Name,
			Platform: account.Platform,
			Type:     account.Type,
			Status:   account.Status,
		},
		Model:      claimedModel,
		UpstreamModel: upstreamModel,
		Attribution:   attribution,
		Verdict: ModelTraceVerdict{
			Status:             status,
			Confidence:         confidence,
			ClaimedModel:       claimedModel,
			PredictedModel:     attribution.Prediction,
			ClaimedProbability: claimedProbability,
		},
		APITest: map[string]any{
			"requested": requested,
			"attempted": attempted,
			"received":  len(responses),
			"accepted":  attribution.UsedOutputs,
			"errors":    apiErrors,
		},
		Responses: responses,
	}
}

// ---------- 上游调用 ----------

// requestCompletion 通过指定账号向上游发起一次补全请求，返回完整文本。
// 根据平台与账号类型选择协议：anthropic → Messages；
// openai apikey → Chat Completions / Responses；openai oauth → ChatGPT Codex Responses。
func (s *ModelTraceService) requestCompletion(ctx context.Context, account *Account, model, prompt string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, modelTraceRequestTimeout)
	defer cancel()

	credentialAccount := account
	if account.IsCredentialShadow() {
		resolved, err := resolveCredentialAccount(ctx, s.accountRepo, account)
		if err != nil {
			return "", err
		}
		credentialAccount = resolved
	}

	if account.Platform == PlatformOpenAI {
		return s.callOpenAI(callCtx, account, credentialAccount, model, prompt)
	}
	return s.callAnthropic(callCtx, account, credentialAccount, model, prompt)
}

func modelTraceValidateBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("base URL 无效: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("base URL 必须以 http(s) 开头")
	}
	return strings.TrimRight(trimmed, "/"), nil
}

// callAnthropic 调 Anthropic /v1/messages（SSE 聚合）。
func (s *ModelTraceService) callAnthropic(ctx context.Context, account, credentialAccount *Account, model, prompt string) (string, error) {
	var apiURL, authToken string
	if credentialAccount.IsOAuth() {
		authToken = credentialAccount.GetCredential("access_token")
		if authToken == "" {
			return "", fmt.Errorf("账号没有可用的 access_token")
		}
		apiURL = testClaudeAPIURL
	} else {
		authToken = credentialAccount.GetCredential("api_key")
		if authToken == "" {
			return "", fmt.Errorf("账号没有可用的 api_key")
		}
		baseURL, err := modelTraceValidateBaseURL(credentialAccount.GetBaseURL())
		if err != nil {
			return "", err
		}
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		apiURL = baseURL + "/v1/messages?beta=true"
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
				},
			},
		},
		"max_tokens": 8192,
		"stream":     true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	for key, value := range claude.DefaultHeaders {
		req.Header.Set(key, value)
	}
	if credentialAccount.IsOAuth() {
		req.Header.Set("anthropic-beta", claude.DefaultBetaHeader)
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else {
		req.Header.Set("anthropic-beta", claude.APIKeyBetaHeader)
		setAnthropicAPIKeyAuthHeader(req.Header, credentialAccount, authToken, credentialAccount.GetBaseURL())
	}
	credentialAccount.ApplyHeaderOverrides(req.Header)

	resp, err := s.doUpstream(req, account, credentialAccount)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, modelTraceCompactErrorBody(respBody))
	}
	return modelTraceParseClaudeStream(resp.Body)
}

// callOpenAI 调 OpenAI 平台账号。
func (s *ModelTraceService) callOpenAI(ctx context.Context, account, credentialAccount *Account, model, prompt string) (string, error) {
	if credentialAccount.IsOAuth() {
		return s.callOpenAIResponsesOAuth(ctx, account, credentialAccount, model, prompt)
	}

	authToken := credentialAccount.GetOpenAIProtocolAPIKey()
	if authToken == "" {
		return "", fmt.Errorf("账号没有可用的 api_key")
	}
	baseURL, err := modelTraceValidateBaseURL(credentialAccount.GetOpenAIBaseURL())
	if err != nil {
		return "", err
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if openai_compat.ShouldUseResponsesAPI(credentialAccount.Extra) {
		return s.callOpenAIResponsesAPIKey(ctx, account, credentialAccount, model, prompt, baseURL, authToken)
	}
	return s.callOpenAIChatCompletions(ctx, account, credentialAccount, model, prompt, baseURL, authToken)
}

// callOpenAIChatCompletions 调 /v1/chat/completions（SSE 聚合）。
func (s *ModelTraceService) callOpenAIChatCompletions(ctx context.Context, account, credentialAccount *Account, model, prompt, baseURL, authToken string) (string, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"stream": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	credentialAccount.ApplyHeaderOverrides(req.Header)

	resp, err := s.doUpstream(req, account, credentialAccount)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, modelTraceCompactErrorBody(respBody))
	}
	return modelTraceParseChatCompletionsStream(resp.Body)
}

// callOpenAIResponsesAPIKey 调 apikey 账号的 /v1/responses 端点。
func (s *ModelTraceService) callOpenAIResponsesAPIKey(ctx context.Context, account, credentialAccount *Account, model, prompt, baseURL, authToken string) (string, error) {
	apiURL := buildOpenAIResponsesURLForPlatform(credentialAccount.Platform, baseURL)
	payload := map[string]any{
		"model":        model,
		"input":        modelTraceResponsesInput(prompt),
		"instructions": openai.DefaultInstructions,
		"stream":       true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	applyOpenAICodexProbeHeaders(req.Header)
	req.Header.Set("Authorization", "Bearer "+authToken)
	credentialAccount.ApplyHeaderOverrides(req.Header)

	resp, err := s.doUpstream(req, account, credentialAccount)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, modelTraceCompactErrorBody(respBody))
	}
	return modelTraceParseResponsesStream(resp.Body)
}

// callOpenAIResponsesOAuth 调 ChatGPT 内部 Codex Responses 端点（与账号测试一致的头）。
func (s *ModelTraceService) callOpenAIResponsesOAuth(ctx context.Context, account, credentialAccount *Account, model, prompt string) (string, error) {
	authToken := credentialAccount.GetOpenAIAccessToken()
	if authToken == "" {
		return "", fmt.Errorf("账号没有可用的 access_token")
	}
	payload := map[string]any{
		"model":        normalizeOpenAIModelForUpstream(credentialAccount, model),
		"input":        modelTraceResponsesInput(prompt),
		"instructions": openai.DefaultInstructions,
		"stream":       true,
		"store":        false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Authorization", "Bearer "+authToken)
	canonical := resolveCodexOutboundIdentity("")
	req.Header.Set("Originator", canonical.originator)
	if customUA := strings.TrimSpace(credentialAccount.GetOpenAIUserAgent()); customUA != "" {
		req.Header.Set("User-Agent", customUA)
	} else {
		req.Header.Set("User-Agent", canonical.userAgent)
	}
	setOpenAIChatGPTAccountHeaders(req.Header, credentialAccount)
	enforceCodexIdentityHeadersWithUA(req.Header, credentialAccount.GetOpenAIUserAgent())
	credentialAccount.ApplyHeaderOverrides(req.Header)

	resp, err := s.doOpenAIUpstream(req, account, credentialAccount)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, modelTraceCompactErrorBody(respBody))
	}
	return modelTraceParseResponsesStream(resp.Body)
}

func modelTraceResponsesInput(prompt string) []map[string]any {
	return []map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": prompt},
			},
		},
	}
}

// doUpstream 通过账号代理/TLS 配置发起请求。
func (s *ModelTraceService) doUpstream(req *http.Request, account, credentialAccount *Account) (*http.Response, error) {
	if s.pluginManager != nil && credentialAccount.Platform == PlatformOpenAI {
		if response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(req.Context(), req, modelTraceProxyURL(account), credentialAccount); handled {
			return response, err
		}
	}
	return s.httpUpstream.DoWithTLS(
		req,
		modelTraceProxyURL(account),
		account.ID,
		account.Concurrency,
		s.tlsFPProfileService.ResolveTLSProfile(account),
	)
}

func (s *ModelTraceService) doOpenAIUpstream(req *http.Request, account, credentialAccount *Account) (*http.Response, error) {
	return s.doUpstream(req, account, credentialAccount)
}

func modelTraceProxyURL(account *Account) string {
	if account.ProxyID != nil && account.Proxy != nil {
		return account.Proxy.URL()
	}
	return ""
}

// modelTraceCompactErrorBody 截断上游错误响应，避免把整页 HTML 塞进事件。
func modelTraceCompactErrorBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}

// ---------- SSE 流聚合 ----------

// modelTraceScanSSE 按 SSE 帧迭代 data 行。
func modelTraceScanSSE(body io.Reader, handle func(dataLine string) (stop bool, err error)) error {
	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("读取上游流失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		stop, err := handle(data)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// modelTraceParseClaudeStream 聚合 Claude Messages SSE 的文本增量。
func modelTraceParseClaudeStream(body io.Reader) (string, error) {
	var sb strings.Builder
	stopReason := ""
	err := modelTraceScanSSE(body, func(dataLine string) (bool, error) {
		if dataLine == "[DONE]" {
			return true, nil
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(dataLine), &data); err != nil {
			return false, nil
		}
		switch eventType, _ := data["type"].(string); eventType {
		case "content_block_delta":
			if delta, ok := data["delta"].(map[string]any); ok {
				if text, ok := delta["text"].(string); ok {
					sb.WriteString(text)
				}
			}
		case "message_delta":
			if delta, ok := data["delta"].(map[string]any); ok {
				if reason, ok := delta["stop_reason"].(string); ok {
					stopReason = reason
				}
			}
		case "error":
			message := "未知错误"
			if errData, ok := data["error"].(map[string]any); ok {
				if msg, ok := errData["message"].(string); ok && msg != "" {
					message = msg
				}
			}
			return true, fmt.Errorf("%s", message)
		}
		return false, nil
	})
	if err != nil {
		return "", err
	}
	switch stopReason {
	case "max_tokens":
		return "", fmt.Errorf("回答因 max_tokens 截断，本次回答不计入")
	case "refusal":
		return "", fmt.Errorf("模型拒绝生成，本次回答不计入")
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("上游返回了空回答")
	}
	return sb.String(), nil
}

// modelTraceParseChatCompletionsStream 聚合 Chat Completions SSE 的文本增量。
func modelTraceParseChatCompletionsStream(body io.Reader) (string, error) {
	var sb strings.Builder
	finishReason := ""
	err := modelTraceScanSSE(body, func(dataLine string) (bool, error) {
		if dataLine == "[DONE]" {
			return true, nil
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(dataLine), &data); err != nil {
			return false, nil
		}
		if errData, ok := data["error"].(map[string]any); ok {
			message := "上游错误"
			if msg, ok := errData["message"].(string); ok && msg != "" {
				message = msg
			}
			return true, fmt.Errorf("%s", message)
		}
		choices, _ := data["choices"].([]any)
		if len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					if content, ok := delta["content"].(string); ok {
						sb.WriteString(content)
					}
				}
				if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
					finishReason = reason
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return "", err
	}
	if finishReason == "length" || finishReason == "content_filter" {
		return "", fmt.Errorf("回答未正常完成（%s），本次回答不计入", finishReason)
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("上游返回了空回答")
	}
	return sb.String(), nil
}

// modelTraceParseResponsesStream 聚合 Responses API SSE 的文本增量。
func modelTraceParseResponsesStream(body io.Reader) (string, error) {
	var sb strings.Builder
	completed := false
	err := modelTraceScanSSE(body, func(dataLine string) (bool, error) {
		if dataLine == "[DONE]" {
			return true, nil
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(dataLine), &data); err != nil {
			return false, nil
		}
		switch eventType, _ := data["type"].(string); eventType {
		case "response.output_text.delta":
			if delta, ok := data["delta"].(string); ok {
				sb.WriteString(delta)
			}
		case "response.completed", "response.done":
			completed = true
			return true, nil
		case "response.incomplete":
			if responseData, ok := data["response"].(map[string]any); ok {
				reason, _ := responseData["incomplete_details"].(map[string]any)
				if reason != nil {
					if reasonText, ok := reason["reason"].(string); ok {
						return true, fmt.Errorf("回答未完成（%s），本次回答不计入", reasonText)
					}
				}
			}
			return true, fmt.Errorf("回答未完成，本次回答不计入")
		case "response.failed":
			message := "OpenAI 响应失败"
			if responseData, ok := data["response"].(map[string]any); ok {
				if errData, ok := responseData["error"].(map[string]any); ok {
					if msg, ok := errData["message"].(string); ok && msg != "" {
						message = msg
					}
				}
			}
			return true, fmt.Errorf("%s", message)
		case "error":
			message := "未知错误"
			if errData, ok := data["error"].(map[string]any); ok {
				if msg, ok := errData["message"].(string); ok && msg != "" {
					message = msg
				}
			}
			return true, fmt.Errorf("%s", message)
		}
		return false, nil
	})
	if err != nil {
		return "", err
	}
	if !completed && strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("上游返回了空回答")
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("上游返回了空回答")
	}
	return sb.String(), nil
}
