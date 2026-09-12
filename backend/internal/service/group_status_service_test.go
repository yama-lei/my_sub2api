//go:build unit

package service

import (
	"testing"
	"time"
)

func fptr(v float64) *float64 { return &v }

func groupStatusTestGroup(id int64) *Group {
	return &Group{ID: id, Name: "Pro", Platform: "openai", Description: "demo group"}
}

// 三行聚合数据：fast 两次调用、normal 一次调用，权重/分子分母手工设定，
// 验证 EMA 加权平均与 overall 直接聚合（而非两组均值再平均）的口径。
func groupStatusTestRows() []GroupStatusUsageRow {
	return []GroupStatusUsageRow{
		{GroupID: 2, Tier: GroupStatusTierFast, Requests: 2,
			SpeedNum: 60 * 0.5, SpeedDen: 0.5, // 60 tok/s，权重 0.5
			TTFTNum: 500*0.5 + 700*0.3, TTFTDen: 0.8,
			CacheNum: 800 * 0.5, CacheDen: 1000 * 0.5},
		{GroupID: 2, Tier: GroupStatusTierNormal, Requests: 1,
			SpeedNum: 20 * 0.5, SpeedDen: 0.5, // 20 tok/s，权重 0.5
			TTFTNum: 300 * 0.4, TTFTDen: 0.4,
			CacheNum: 200 * 0.5, CacheDen: 800 * 0.5},
	}
}

func TestBuildGroupStatusEntryMetrics(t *testing.T) {
	entry := BuildGroupStatusEntry(groupStatusTestGroup(2), groupStatusTestRows(), GroupStatusErrorRow{GroupID: 2, Total: 3, Service: 1})

	if entry.Name != "Pro" || entry.Platform != "openai" {
		t.Fatalf("unexpected group meta: %+v", entry)
	}
	if !entry.HasTraffic {
		t.Fatal("entry should have traffic")
	}

	// fast：decode = 60；TTFT = (500*0.5+700*0.3)/0.8 = 575；cache = 800/1000 = 0.8
	if entry.Fast.Requests != 2 {
		t.Fatalf("fast requests = %d, want 2", entry.Fast.Requests)
	}
	if got := *entry.Fast.DecodeSpeedTPS; got != 60 {
		t.Fatalf("fast decode speed = %v, want 60", got)
	}
	if got := *entry.Fast.TTFTMs; got != 575 {
		t.Fatalf("fast ttft = %v, want 575", got)
	}
	if got := *entry.Fast.CacheRate; got != 0.8 {
		t.Fatalf("fast cache rate = %v, want 0.8", got)
	}

	// normal：decode = 20；TTFT = 300；cache = 0.25
	if got := *entry.Normal.DecodeSpeedTPS; got != 20 {
		t.Fatalf("normal decode speed = %v, want 20", got)
	}
	if got := *entry.Normal.TTFTMs; got != 300 {
		t.Fatalf("normal ttft = %v, want 300", got)
	}
	if got := *entry.Normal.CacheRate; got != 0.25 {
		t.Fatalf("normal cache rate = %v, want 0.25", got)
	}

	// overall：分子分母直接跨 tier 合并，decode = (60*0.5+20*0.5)/1 = 40
	if entry.Overall.Requests != 3 {
		t.Fatalf("overall requests = %d, want 3", entry.Overall.Requests)
	}
	if got := *entry.Overall.DecodeSpeedTPS; got != 40 {
		t.Fatalf("overall decode speed = %v, want 40 (direct aggregation, not tier average)", got)
	}
	// (500*0.5+700*0.3 + 300*0.4) / (0.8+0.4) = 580/1.2
	if got, want := *entry.Overall.TTFTMs, 580/1.2; got-want > 1e-9 || want-got > 1e-9 {
		t.Fatalf("overall ttft = %v, want %v", got, want)
	}
	if got, want := *entry.Overall.CacheRate, 500.0/900.0; got-want > 1e-9 || want-got > 1e-9 {
		t.Fatalf("overall cache rate = %v, want %v", got, want)
	}
}

func TestBuildGroupStatusEntryUptimeAndHealth(t *testing.T) {
	cases := []struct {
		name       string
		usage      []GroupStatusUsageRow
		errs       GroupStatusErrorRow
		wantStatus string
		wantRate   *float64
	}{
		{"no traffic", nil, GroupStatusErrorRow{}, GroupHealthIdle, nil},
		{"all healthy", groupStatusTestRows(), GroupStatusErrorRow{Total: 0}, GroupHealthHealthy, fptr(1)},
		{"degraded", []GroupStatusUsageRow{{GroupID: 2, Tier: GroupStatusTierFast, Requests: 95}}, GroupStatusErrorRow{Total: 5, Service: 5}, GroupHealthDegraded, fptr(0.95)},
		{"business-limited errors ignored", groupStatusTestRows(), GroupStatusErrorRow{Total: 50, Service: 0}, GroupHealthHealthy, fptr(1)},
		{"down", groupStatusTestRows(), GroupStatusErrorRow{Total: 27, Service: 27}, GroupHealthDown, fptr(0.1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := BuildGroupStatusEntry(groupStatusTestGroup(2), tc.usage, tc.errs)
			if entry.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", entry.Status, tc.wantStatus)
			}
			if tc.wantRate == nil {
				if entry.Uptime.SuccessRate != nil {
					t.Fatalf("success rate = %v, want nil", *entry.Uptime.SuccessRate)
				}
				return
			}
			if diff := *entry.Uptime.SuccessRate - *tc.wantRate; diff > 1e-9 || diff < -1e-9 {
				t.Fatalf("success rate = %v, want %v", *entry.Uptime.SuccessRate, *tc.wantRate)
			}
		})
	}
}

func TestGroupServiceTierIsFast(t *testing.T) {
	for tier, want := range map[string]bool{
		"priority": true, "fast": true,
		"": false, "default": false, "standard": false, "flex": false, "auto": false,
	} {
		if got := GroupServiceTierIsFast(tier); got != want {
			t.Fatalf("GroupServiceTierIsFast(%q) = %v, want %v", tier, got, want)
		}
	}
}

func TestGroupStatusTierStatsEmpty(t *testing.T) {
	entry := BuildGroupStatusEntry(groupStatusTestGroup(1), []GroupStatusUsageRow{
		{GroupID: 1, Tier: GroupStatusTierFast, Requests: 0},
	}, GroupStatusErrorRow{})
	if entry.HasTraffic {
		t.Fatal("zero-request rows must not count as traffic")
	}
	if entry.Fast.DecodeSpeedTPS != nil || entry.Fast.TTFTMs != nil || entry.Fast.CacheRate != nil {
		t.Fatalf("empty tier stats must be nil pointers: %+v", entry.Fast)
	}
	if entry.Overall.Requests != 0 {
		t.Fatalf("overall requests = %d, want 0", entry.Overall.Requests)
	}
}

func TestGroupHealthStatusBands(t *testing.T) {
	if got := GroupHealthStatus(true, fptr(0.999)); got != GroupHealthHealthy {
		t.Fatalf("0.999 → %q, want healthy", got)
	}
	if got := GroupHealthStatus(true, fptr(0.95)); got != GroupHealthDegraded {
		t.Fatalf("0.95 → %q, want degraded", got)
	}
	if got := GroupHealthStatus(true, fptr(0.5)); got != GroupHealthDown {
		t.Fatalf("0.5 → %q, want down", got)
	}
	if got := GroupHealthStatus(false, nil); got != GroupHealthIdle {
		t.Fatalf("no traffic → %q, want idle", got)
	}
}

// 窗口与半衰期常量兜底检查，防止误改成 0 导致权重公式除零。
func TestGroupStatusWindowConstants(t *testing.T) {
	if GroupStatusWindow != 24*time.Hour {
		t.Fatalf("window = %v, want 24h", GroupStatusWindow)
	}
	if GroupStatusEmaHalfLife <= 0 || GroupStatusEmaHalfLife > GroupStatusWindow {
		t.Fatalf("half life = %v, must be in (0, 24h]", GroupStatusEmaHalfLife)
	}
}
