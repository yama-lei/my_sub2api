package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
)

// Account quality: adulteration (掺水) rate, TTFT and TPS per account.
//
// Adulteration evidence comes from three places:
//   - usage_logs.upstream_model_mismatch: served requests whose upstream
//     response declared a different model than the one sent;
//   - account_quality_events kind=downgrade: attempts the model downgrade guard
//     abandoned before any output (these never reach usage_logs);
//   - account_quality_events kind=probe: scheduled ModelTrace fingerprint checks
//     that identify the model from its output distribution, which catches
//     upstreams that also lie about response.model.
const (
	AccountQualityEventDowngrade = "downgrade"
	AccountQualityEventProbe     = "probe"

	ModelProbeVerdictPass    = "pass"
	ModelProbeVerdictFail    = "fail"
	ModelProbeVerdictUnknown = "unknown" // expected model is not in the fingerprint bank
	ModelProbeVerdictError   = "error"   // no usable output

	modelProbeEnabledExtraKey  = "model_probe_enabled"
	modelProbeIntervalExtraKey = "model_probe_interval_minutes"
	modelProbeModelExtraKey    = "model_probe_model"
	modelProbeExpectedExtraKey = "model_probe_expected_model"
	modelProbeSamplesExtraKey  = "model_probe_samples"

	defaultModelProbeIntervalMinutes = 360
	minModelProbeIntervalMinutes     = 30
	maxModelProbeIntervalMinutes     = 7 * 24 * 60
	defaultModelProbeSamples         = 3
	maxModelProbeSamples             = 5

	modelProbeSampleTimeout   = 4 * time.Minute
	modelProbeMaxWorkers      = 3
	accountQualityEventMaxAge = 30 * 24 * time.Hour
)

// AccountQualityEvent is one row of account_quality_events.
type AccountQualityEvent struct {
	ID             int64           `json:"id"`
	AccountID      int64           `json:"account_id"`
	Kind           string          `json:"kind"`
	RequestedModel string          `json:"requested_model"`
	ObservedModel  string          `json:"observed_model"`
	Verdict        string          `json:"verdict"`
	Score          *float64        `json:"score,omitempty"`
	Detail         json.RawMessage `json:"detail,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// AccountUsageQualityStats aggregates usage_logs for one account.
type AccountUsageQualityStats struct {
	AccountID      int64
	Requests       int64
	Mismatches     int64
	StreamRequests int64
	AvgTTFTMs      *float64
	P50TTFTMs      *float64
	P90TTFTMs      *float64
	GenTokens      int64
	GenMs          int64
	AvgDurationMs  *float64
}

// AccountEventQualityStats aggregates account_quality_events for one account.
type AccountEventQualityStats struct {
	AccountID  int64
	Downgrades int64
	Probes     int64
	ProbeFails int64
}

// AccountQualityRepository persists quality events and reads aggregate stats.
type AccountQualityRepository interface {
	InsertEvent(ctx context.Context, event *AccountQualityEvent) error
	ListEvents(ctx context.Context, accountID int64, kind string, limit int) ([]*AccountQualityEvent, error)
	UsageStats(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]*AccountUsageQualityStats, error)
	EventStats(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]*AccountEventQualityStats, error)
	LatestProbes(ctx context.Context, accountIDs []int64) (map[int64]*AccountQualityEvent, error)
	ListProbeEnabledAccountIDs(ctx context.Context) ([]int64, error)
	DeleteEventsBefore(ctx context.Context, before time.Time) (int64, error)
}

// AccountQualityMetrics is the API view of one account's quality.
type AccountQualityMetrics struct {
	AccountID   int64 `json:"account_id"`
	WindowHours int   `json:"window_hours"`
	Requests    int64 `json:"requests"`
	// Mismatches are served requests whose upstream declared another model.
	Mismatches int64 `json:"mismatches"`
	// Downgrades are attempts abandoned by the downgrade guard.
	Downgrades int64 `json:"downgrades"`
	// AdulterationRate = (mismatches + downgrades) / (requests + downgrades).
	AdulterationRate *float64             `json:"adulteration_rate"`
	Probes           int64                `json:"probes"`
	ProbeFails       int64                `json:"probe_fails"`
	ProbeFailRate    *float64             `json:"probe_fail_rate"`
	LatestProbe      *AccountQualityEvent `json:"latest_probe,omitempty"`
	AvgTTFTMs        *float64             `json:"avg_ttft_ms"`
	P50TTFTMs        *float64             `json:"p50_ttft_ms"`
	P90TTFTMs        *float64             `json:"p90_ttft_ms"`
	AvgDurationMs    *float64             `json:"avg_duration_ms"`
	// TPS is output tokens per second after the first token, over stream requests.
	TPS *float64 `json:"tps"`
}

// ModelProbeConfig is the per-account scheduled ModelTrace check, stored in account.Extra.
type ModelProbeConfig struct {
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"interval_minutes"`
	Model           string `json:"model"`
	ExpectedModel   string `json:"expected_model"`
	Samples         int    `json:"samples"`
}

// GetModelProbeConfig reads the probe settings from Extra with defaults applied.
func (a *Account) GetModelProbeConfig() ModelProbeConfig {
	cfg := ModelProbeConfig{IntervalMinutes: defaultModelProbeIntervalMinutes, Samples: defaultModelProbeSamples}
	if a == nil || a.Extra == nil {
		return cfg
	}
	if v, ok := a.Extra[modelProbeEnabledExtraKey].(bool); ok {
		cfg.Enabled = v
	}
	if raw, ok := a.Extra[modelProbeIntervalExtraKey]; ok && raw != nil {
		if n := parsePoolModeRetryCount(raw); n > 0 {
			cfg.IntervalMinutes = min(max(n, minModelProbeIntervalMinutes), maxModelProbeIntervalMinutes)
		}
	}
	if raw, ok := a.Extra[modelProbeSamplesExtraKey]; ok && raw != nil {
		if n := parsePoolModeRetryCount(raw); n > 0 {
			cfg.Samples = min(n, maxModelProbeSamples)
		}
	}
	if v, ok := a.Extra[modelProbeModelExtraKey].(string); ok {
		cfg.Model = strings.TrimSpace(v)
	}
	if v, ok := a.Extra[modelProbeExpectedExtraKey].(string); ok {
		cfg.ExpectedModel = strings.TrimSpace(v)
	}
	return cfg
}

// resolveModelProbeExpected maps the configured expectation (or, by default, the
// probed model) onto a fingerprint bank model; "" when the bank does not know it.
func resolveModelProbeExpected(cfg ModelProbeConfig) string {
	candidate := cfg.ExpectedModel
	if candidate == "" {
		candidate = cfg.Model
	}
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return ""
	}
	for _, known := range modeltrace.KnownModels() {
		if strings.EqualFold(known, candidate) {
			return known
		}
	}
	return ""
}

// AccountQualityService records quality events, serves metrics and runs the
// scheduled ModelTrace probes.
type AccountQualityService struct {
	repo           AccountQualityRepository
	accountRepo    AccountRepository
	accountTestSvc *AccountTestService

	mu        sync.Mutex
	lastProbe map[int64]time.Time
	running   map[int64]bool
	loaded    bool

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

func NewAccountQualityService(repo AccountQualityRepository, accountRepo AccountRepository, accountTestSvc *AccountTestService) *AccountQualityService {
	return &AccountQualityService{
		repo:           repo,
		accountRepo:    accountRepo,
		accountTestSvc: accountTestSvc,
		lastProbe:      make(map[int64]time.Time),
		running:        make(map[int64]bool),
		stopCh:         make(chan struct{}),
	}
}

// RecordDowngrade stores a guard-abandoned attempt. It never blocks the gateway.
func (s *AccountQualityService) RecordDowngrade(accountID int64, requestedModel, observedModel, mode string, generation int) {
	if s == nil || s.repo == nil || accountID <= 0 {
		return
	}
	detail, _ := json.Marshal(map[string]any{"mode": mode, "reroll_generation": generation})
	event := &AccountQualityEvent{
		AccountID:      accountID,
		Kind:           AccountQualityEventDowngrade,
		RequestedModel: truncateQualityModel(requestedModel),
		ObservedModel:  truncateQualityModel(observedModel),
		Detail:         detail,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.InsertEvent(ctx, event); err != nil {
			logger.L().Warn("account_quality.record_downgrade_failed", zap.Int64("account_id", accountID), zap.Error(err))
		}
	}()
}

func truncateQualityModel(model string) string {
	model = strings.TrimSpace(model)
	if len(model) > 255 {
		return model[:255]
	}
	return model
}

// GetMetrics returns quality metrics for the given accounts over the last windowHours.
func (s *AccountQualityService) GetMetrics(ctx context.Context, accountIDs []int64, windowHours int) (map[int64]*AccountQualityMetrics, error) {
	if windowHours <= 0 {
		windowHours = 24
	}
	windowHours = min(windowHours, 30*24)
	out := make(map[int64]*AccountQualityMetrics, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	since := time.Now().Add(-time.Duration(windowHours) * time.Hour)
	usage, err := s.repo.UsageStats(ctx, accountIDs, since)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.EventStats(ctx, accountIDs, since)
	if err != nil {
		return nil, err
	}
	latest, err := s.repo.LatestProbes(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range accountIDs {
		out[id] = buildAccountQualityMetrics(id, windowHours, usage[id], events[id], latest[id])
	}
	return out, nil
}

func buildAccountQualityMetrics(accountID int64, windowHours int, usage *AccountUsageQualityStats, events *AccountEventQualityStats, latest *AccountQualityEvent) *AccountQualityMetrics {
	m := &AccountQualityMetrics{AccountID: accountID, WindowHours: windowHours, LatestProbe: latest}
	if usage != nil {
		m.Requests = usage.Requests
		m.Mismatches = usage.Mismatches
		m.AvgTTFTMs = usage.AvgTTFTMs
		m.P50TTFTMs = usage.P50TTFTMs
		m.P90TTFTMs = usage.P90TTFTMs
		m.AvgDurationMs = usage.AvgDurationMs
		if usage.GenMs > 0 && usage.GenTokens > 0 {
			tps := float64(usage.GenTokens) / (float64(usage.GenMs) / 1000)
			m.TPS = &tps
		}
	}
	if events != nil {
		m.Downgrades = events.Downgrades
		m.Probes = events.Probes
		m.ProbeFails = events.ProbeFails
	}
	if attempts := m.Requests + m.Downgrades; attempts > 0 {
		rate := float64(m.Mismatches+m.Downgrades) / float64(attempts)
		m.AdulterationRate = &rate
	}
	if m.Probes > 0 {
		rate := float64(m.ProbeFails) / float64(m.Probes)
		m.ProbeFailRate = &rate
	}
	return m
}

// ListEvents returns the newest events of an account (kind "" = all kinds).
func (s *AccountQualityService) ListEvents(ctx context.Context, accountID int64, kind string, limit int) ([]*AccountQualityEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListEvents(ctx, accountID, kind, limit)
}

// Start launches the probe scheduler (ticks every minute).
func (s *AccountQualityService) Start() {
	if s == nil || s.repo == nil || s.accountTestSvc == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go s.loop()
	})
}

// Stop halts the scheduler and waits for in-flight probes to notice.
func (s *AccountQualityService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		done := make(chan struct{})
		go func() { s.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	})
}

func (s *AccountQualityService) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	lastCleanup := time.Time{}
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
		}
		s.runDueProbes()
		if time.Since(lastCleanup) > 6*time.Hour {
			lastCleanup = time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			if _, err := s.repo.DeleteEventsBefore(ctx, time.Now().Add(-accountQualityEventMaxAge)); err != nil {
				logger.L().Warn("account_quality.cleanup_failed", zap.Error(err))
			}
			cancel()
		}
	}
}

func (s *AccountQualityService) runDueProbes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ids, err := s.repo.ListProbeEnabledAccountIDs(ctx)
	if err != nil {
		logger.L().Warn("account_quality.list_probe_accounts_failed", zap.Error(err))
		return
	}
	if len(ids) == 0 {
		return
	}
	if err := s.loadLastProbes(ctx, ids); err != nil {
		logger.L().Warn("account_quality.load_last_probes_failed", zap.Error(err))
		return
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		logger.L().Warn("account_quality.load_probe_accounts_failed", zap.Error(err))
		return
	}

	now := time.Now()
	sem := make(chan struct{}, modelProbeMaxWorkers)
	for _, account := range accounts {
		if account == nil || !account.IsActive() {
			continue
		}
		cfg := account.GetModelProbeConfig()
		if !cfg.Enabled || cfg.Model == "" {
			continue
		}
		if !s.claimProbe(account.ID, now, time.Duration(cfg.IntervalMinutes)*time.Minute) {
			continue
		}
		select {
		case <-s.stopCh:
			s.releaseProbe(account.ID)
			return
		case sem <- struct{}{}:
		}
		s.wg.Add(1)
		go func(acc *Account, cfg ModelProbeConfig) {
			defer s.wg.Done()
			defer func() { <-sem }()
			defer s.releaseProbe(acc.ID)
			if _, err := s.RunProbe(context.Background(), acc.ID, cfg); err != nil {
				logger.L().Warn("account_quality.probe_failed", zap.Int64("account_id", acc.ID), zap.Error(err))
			}
		}(account, cfg)
	}
}

// loadLastProbes seeds the in-memory schedule from the DB once, so restarts do
// not re-probe every account immediately.
func (s *AccountQualityService) loadLastProbes(ctx context.Context, ids []int64) error {
	s.mu.Lock()
	loaded := s.loaded
	s.mu.Unlock()
	if loaded {
		return nil
	}
	latest, err := s.repo.LatestProbes(ctx, ids)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ev := range latest {
		if ev != nil && ev.CreatedAt.After(s.lastProbe[id]) {
			s.lastProbe[id] = ev.CreatedAt
		}
	}
	s.loaded = true
	return nil
}

func (s *AccountQualityService) claimProbe(accountID int64, now time.Time, interval time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[accountID] {
		return false
	}
	if last, ok := s.lastProbe[accountID]; ok && now.Sub(last) < interval {
		return false
	}
	s.running[accountID] = true
	s.lastProbe[accountID] = now
	return true
}

func (s *AccountQualityService) releaseProbe(accountID int64) {
	s.mu.Lock()
	delete(s.running, accountID)
	s.mu.Unlock()
}

// ErrModelProbeBusy is returned when a probe for the account is already running.
var ErrModelProbeBusy = errors.New("a model probe for this account is already running")

// RunProbeNow runs a probe immediately with the account's saved config
// (Model may be overridden), regardless of whether scheduling is enabled.
func (s *AccountQualityService) RunProbeNow(ctx context.Context, accountID int64, model, expectedModel string) (*AccountQualityEvent, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	cfg := account.GetModelProbeConfig()
	if m := strings.TrimSpace(model); m != "" {
		cfg.Model = m
	}
	if m := strings.TrimSpace(expectedModel); m != "" {
		cfg.ExpectedModel = m
	}
	if cfg.Model == "" {
		return nil, errors.New("probe model is not configured")
	}
	if !s.claimProbe(accountID, time.Now(), 0) {
		return nil, ErrModelProbeBusy
	}
	defer s.releaseProbe(accountID)
	return s.RunProbe(ctx, accountID, cfg)
}

type modelProbeSampleDetail struct {
	ExpectedCount int    `json:"expected_count"`
	Parsed        int    `json:"parsed_numbers"`
	Accepted      bool   `json:"accepted"`
	ObservedModel string `json:"observed_model,omitempty"`
	LatencyMs     int64  `json:"latency_ms"`
	Error         string `json:"error,omitempty"`
}

type modelProbeCandidate struct {
	Model       string  `json:"model"`
	Probability float64 `json:"probability"`
}

type modelProbeDetail struct {
	ExpectedModel       string                   `json:"expected_model,omitempty"`
	ExpectedProbability *float64                 `json:"expected_probability,omitempty"`
	Prediction          string                   `json:"prediction,omitempty"`
	PredictionProb      float64                  `json:"prediction_probability,omitempty"`
	FamilyPrediction    string                   `json:"family_prediction,omitempty"`
	UsedOutputs         int                      `json:"used_outputs"`
	Top                 []modelProbeCandidate    `json:"top,omitempty"`
	Samples             []modelProbeSampleDetail `json:"samples"`
	Error               string                   `json:"error,omitempty"`
}

// RunProbe sends cfg.Samples ModelTrace challenges to the account, scores the
// answers against the fingerprint bank and stores the verdict as a probe event.
func (s *AccountQualityService) RunProbe(ctx context.Context, accountID int64, cfg ModelProbeConfig) (*AccountQualityEvent, error) {
	samples := cfg.Samples
	if samples <= 0 {
		samples = defaultModelProbeSamples
	}
	challenges := modeltrace.GenerateChallenges(samples)
	detail := modelProbeDetail{ExpectedModel: resolveModelProbeExpected(cfg)}
	outputs := make([]modeltrace.Output, 0, len(challenges))
	observed := ""
	for _, ch := range challenges {
		sampleCtx, cancel := context.WithTimeout(ctx, modelProbeSampleTimeout)
		run := s.accountTestSvc.RunProbeBackground(sampleCtx, accountID, cfg.Model, ch.Prompt)
		cancel()
		sd := modelProbeSampleDetail{
			ExpectedCount: ch.ExpectedCount,
			Parsed:        len(modeltrace.ParseNumbers(run.Text)),
			ObservedModel: run.ObservedModel,
			LatencyMs:     run.LatencyMs,
			Error:         truncateProbeError(run.ErrorMessage),
		}
		sd.Accepted = sd.Parsed >= modeltrace.MinimumNumbersFor(ch.ExpectedCount)
		if observed == "" {
			observed = run.ObservedModel
		}
		detail.Samples = append(detail.Samples, sd)
		outputs = append(outputs, modeltrace.Output{Text: run.Text, ExpectedCount: ch.ExpectedCount})
		if ctx.Err() != nil {
			break
		}
	}

	event := &AccountQualityEvent{
		AccountID:      accountID,
		Kind:           AccountQualityEventProbe,
		RequestedModel: truncateQualityModel(cfg.Model),
		ObservedModel:  truncateQualityModel(observed),
	}
	analysis, err := modeltrace.Analyze(outputs)
	if err != nil {
		event.Verdict = ModelProbeVerdictError
		detail.Error = err.Error()
	} else {
		detail.Prediction = analysis.Prediction
		detail.PredictionProb = analysis.Probability
		detail.FamilyPrediction = analysis.FamilyPrediction
		detail.UsedOutputs = analysis.UsedOutputs
		for i, r := range analysis.Results {
			if i >= 3 {
				break
			}
			detail.Top = append(detail.Top, modelProbeCandidate{Model: r.Model, Probability: r.Probability})
		}
		event.Verdict = ModelProbeVerdictUnknown
		if detail.ExpectedModel != "" {
			p, _ := modeltrace.ProbabilityOf(analysis, detail.ExpectedModel)
			detail.ExpectedProbability = &p
			event.Score = &p
			if analysis.Prediction == detail.ExpectedModel {
				event.Verdict = ModelProbeVerdictPass
			} else {
				event.Verdict = ModelProbeVerdictFail
			}
		}
	}
	event.Detail, _ = json.Marshal(detail)
	event.CreatedAt = time.Now()

	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.InsertEvent(saveCtx, event); err != nil {
		return event, fmt.Errorf("save probe result: %w", err)
	}
	logger.L().Info("account_quality.probe_done",
		zap.Int64("account_id", accountID),
		zap.String("model", cfg.Model),
		zap.String("verdict", event.Verdict),
		zap.String("prediction", detail.Prediction),
		zap.String("expected", detail.ExpectedModel),
	)
	return event, nil
}

func truncateProbeError(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}
