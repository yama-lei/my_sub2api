package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAccountQualityMetrics(t *testing.T) {
	avg := 1200.0
	m := buildAccountQualityMetrics(24, 24,
		&AccountUsageQualityStats{Requests: 90, Mismatches: 5, AvgTTFTMs: &avg, GenTokens: 5000, GenMs: 100000},
		&AccountEventQualityStats{Downgrades: 10, Probes: 4, ProbeFails: 1},
		nil,
	)
	require.NotNil(t, m.AdulterationRate)
	require.InDelta(t, 15.0/100.0, *m.AdulterationRate, 1e-9)
	require.NotNil(t, m.TPS)
	require.InDelta(t, 50.0, *m.TPS, 1e-9)
	require.NotNil(t, m.ProbeFailRate)
	require.InDelta(t, 0.25, *m.ProbeFailRate, 1e-9)
	require.Equal(t, &avg, m.AvgTTFTMs)

	empty := buildAccountQualityMetrics(1, 24, nil, nil, nil)
	require.Nil(t, empty.AdulterationRate)
	require.Nil(t, empty.TPS)
	require.Nil(t, empty.ProbeFailRate)
}

func TestModelProbeConfig(t *testing.T) {
	cfg := (&Account{}).GetModelProbeConfig()
	require.False(t, cfg.Enabled)
	require.Equal(t, defaultModelProbeIntervalMinutes, cfg.IntervalMinutes)
	require.Equal(t, defaultModelProbeSamples, cfg.Samples)

	cfg = (&Account{Extra: map[string]any{
		modelProbeEnabledExtraKey:  true,
		modelProbeIntervalExtraKey: float64(5),
		modelProbeSamplesExtraKey:  float64(9),
		modelProbeModelExtraKey:    " gpt-6-sol ",
	}}).GetModelProbeConfig()
	require.True(t, cfg.Enabled)
	require.Equal(t, minModelProbeIntervalMinutes, cfg.IntervalMinutes)
	require.Equal(t, maxModelProbeSamples, cfg.Samples)
	require.Equal(t, "gpt-6-sol", cfg.Model)

	require.Equal(t, "gpt-6-sol", resolveModelProbeExpected(cfg))
	cfg.Model = "gpt-6.1-sol"
	require.Equal(t, "", resolveModelProbeExpected(cfg), "models outside the bank have no expectation")
	cfg.ExpectedModel = "GPT-6-SOL"
	require.Equal(t, "gpt-6-sol", resolveModelProbeExpected(cfg))
}
