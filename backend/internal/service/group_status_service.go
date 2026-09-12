package service

import (
	"context"
	"time"
)

// 分组状态页（面向所有登录用户）的指标口径。
const (
	// GroupStatusWindow 状态统计回看窗口。
	GroupStatusWindow = 24 * time.Hour
	// GroupStatusEmaHalfLife EMA 时间衰减半衰期：6 小时前的样本权重减半，
	// 24h 窗口末端（半衰期的 4 倍）权重约 6.25%，体现"越新越重要"。
	GroupStatusEmaHalfLife = 6 * time.Hour
)

// 分组健康状态带（与前端展示约定一致）。
const (
	GroupHealthHealthy  = "healthy"
	GroupHealthDegraded = "degraded"
	GroupHealthDown     = "down"
	GroupHealthIdle     = "idle"
)

// 分组维度下 fast/normal 两种模式。fast 对应 usage_logs.service_tier 中的
// "priority"/"fast"（OpenAI priority、Anthropic speed=fast），其余（含 NULL、
// "default"/"standard"/"flex" 等）归入 normal。
const (
	GroupStatusTierFast   = "fast"
	GroupStatusTierNormal = "normal"
)

// GroupServiceTierIsFast 判断 usage_logs.service_tier 是否属于 fast 模式。
func GroupServiceTierIsFast(tier string) bool {
	switch tier {
	case "priority", "fast":
		return true
	default:
		return false
	}
}

// GroupStatusTierStats 单个分组在某模式（fast/normal/overall）下的窗口内质量指标。
// 所有均值都按样本时间做 EMA 衰减加权（越新的调用权重越高）。
// 指针为 nil 表示窗口内该指标没有可用样本。
type GroupStatusTierStats struct {
	// Requests 窗口内该模式的调用数（未加权）。
	Requests int64 `json:"requests"`
	// DecodeSpeedTPS 平均 decode 速度（output tokens / 秒，TTFT 之后的纯解码段）。
	DecodeSpeedTPS *float64 `json:"decode_speed_tps"`
	// TTFTMs 平均首 token 延迟（毫秒）。
	TTFTMs *float64 `json:"ttft_ms"`
	// CacheRate 平均缓存命中率（cache_read / (input + cache_creation + cache_read)，
	// 与渠道监控的缓存率口径一致，0-1）。
	CacheRate *float64 `json:"cache_rate"`
}

// GroupStatusUptime 单个分组的成功调用率（uptime）。
type GroupStatusUptime struct {
	// SuccessRequests 窗口内成功（计费）调用数。
	SuccessRequests int64 `json:"success_requests"`
	// ErrorRequests 窗口内错误调用总数（含业务限制）。
	ErrorRequests int64 `json:"error_requests"`
	// ServiceErrors 非业务限制错误数（余额不足/配额用尽等用户侧原因不计入 uptime）。
	ServiceErrors int64 `json:"service_errors"`
	// SuccessRate 成功调用率（0-1）：success / (success + service_errors)。
	// 窗口内无任何调用时为 nil。
	SuccessRate *float64 `json:"success_rate"`
}

// GroupStatusEntry 单个分组的完整状态。
type GroupStatusEntry struct {
	GroupID     int64  `json:"group_id"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Description string `json:"description"`
	// HasTraffic 窗口内是否有任何调用（成功或错误）。
	HasTraffic bool `json:"has_traffic"`
	// Status 健康状态带：healthy / degraded / down / idle。
	Status string               `json:"status"`
	Fast   GroupStatusTierStats `json:"fast"`
	Normal GroupStatusTierStats `json:"normal"`
	// Overall 不区分模式的整体指标（= Fast + Normal 合并）。
	Overall GroupStatusTierStats `json:"overall"`
	Uptime  GroupStatusUptime    `json:"uptime"`
}

// GroupStatusReport 分组状态接口的完整响应。
type GroupStatusReport struct {
	GeneratedAt      time.Time          `json:"generated_at"`
	WindowHours      int                `json:"window_hours"`
	EmaHalfLifeHours int                `json:"ema_half_life_hours"`
	Groups           []GroupStatusEntry `json:"groups"`
}

// GroupStatusUsageRow usage_logs 按 (group, tier) 聚合后的 EMA 加权中间结果，
// 由仓储层产出、服务层组装。
type GroupStatusUsageRow struct {
	GroupID  int64
	Tier     string
	Requests int64
	// SpeedNum Σ(tokens·w / decode_seconds)；SpeedDen Σw（仅 decode 有效的样本）。
	SpeedNum float64
	SpeedDen float64
	// TTFTNum Σ(ttft_ms·w)；TTFTDen Σw（仅有 first_token_ms 的样本）。
	TTFTNum float64
	TTFTDen float64
	// CacheNum Σ(cache_read·w)；CacheDen Σ((input+cache_creation+cache_read)·w)。
	CacheNum float64
	CacheDen float64
}

// GroupStatusErrorRow ops_error_logs 按 group 聚合的错误计数。
type GroupStatusErrorRow struct {
	GroupID int64
	Total   int64
	// Service 非业务限制（is_business_limited=false）的错误数。
	Service int64
}

// GroupStatusRepository 分组状态页的只读聚合查询。
type GroupStatusRepository interface {
	// GetGroupTierStats 返回 (since, until) 窗口内按 (group, tier) 的 EMA 加权聚合，
	// halfLife 为权重半衰期。
	GetGroupTierStats(ctx context.Context, since, until time.Time, halfLife time.Duration) ([]GroupStatusUsageRow, error)
	// GetGroupErrorStats 返回 since 起按 group 聚合的错误计数。
	GetGroupErrorStats(ctx context.Context, since time.Time) ([]GroupStatusErrorRow, error)
}

// GroupStatusService 面向所有登录用户的分组状态页服务。
type GroupStatusService struct {
	apiKeyService *APIKeyService
	repo          GroupStatusRepository
	window        time.Duration
	halfLife      time.Duration
}

// NewGroupStatusService 构造分组状态服务。
func NewGroupStatusService(apiKeyService *APIKeyService, repo GroupStatusRepository) *GroupStatusService {
	return &GroupStatusService{
		apiKeyService: apiKeyService,
		repo:          repo,
		window:        GroupStatusWindow,
		halfLife:      GroupStatusEmaHalfLife,
	}
}

// GetReport 返回当前用户可见分组的状态报告。
// 分组可见性与 API Key 绑定处使用同一套逻辑（GetAvailableGroups）。
func (s *GroupStatusService) GetReport(ctx context.Context, userID int64) (*GroupStatusReport, error) {
	groups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	until := time.Now()
	since := until.Add(-s.window)

	usageRows, err := s.repo.GetGroupTierStats(ctx, since, until, s.halfLife)
	if err != nil {
		return nil, err
	}
	errorRows, err := s.repo.GetGroupErrorStats(ctx, since)
	if err != nil {
		return nil, err
	}

	usageByGroup := make(map[int64][]GroupStatusUsageRow, len(usageRows))
	for _, row := range usageRows {
		usageByGroup[row.GroupID] = append(usageByGroup[row.GroupID], row)
	}
	errorsByGroup := make(map[int64]GroupStatusErrorRow, len(errorRows))
	for _, row := range errorRows {
		errorsByGroup[row.GroupID] = row
	}

	report := &GroupStatusReport{
		GeneratedAt:      until.UTC(),
		WindowHours:      int(s.window / time.Hour),
		EmaHalfLifeHours: int(s.halfLife / time.Hour),
		Groups:           make([]GroupStatusEntry, 0, len(groups)),
	}
	for i := range groups {
		group := &groups[i]
		entry := BuildGroupStatusEntry(group, usageByGroup[group.ID], errorsByGroup[group.ID])
		report.Groups = append(report.Groups, entry)
	}
	return report, nil
}

// BuildGroupStatusEntry 由聚合行组装单个分组的状态（纯函数，便于单测）。
func BuildGroupStatusEntry(group *Group, usage []GroupStatusUsageRow, errs GroupStatusErrorRow) GroupStatusEntry {
	fast, normal, overall := groupStatusTierStats(usage)

	success := overall.Requests
	serviceErrs := errs.Service
	entry := GroupStatusEntry{
		GroupID:     group.ID,
		Name:        group.Name,
		Platform:    group.Platform,
		Description: group.Description,
		HasTraffic:  success > 0 || errs.Total > 0,
		Fast:        fast,
		Normal:      normal,
		Overall:     overall,
		Uptime: GroupStatusUptime{
			SuccessRequests: success,
			ErrorRequests:   errs.Total,
			ServiceErrors:   serviceErrs,
		},
	}
	entry.Uptime.SuccessRate = groupStatusSuccessRate(success, serviceErrs)
	entry.Status = GroupHealthStatus(entry.HasTraffic, entry.Uptime.SuccessRate)
	return entry
}

// GroupHealthStatus 根据是否有流量与成功调用率给出健康状态带。
// 无流量 → idle；成功率 ≥99% → healthy；≥90% → degraded；否则 down。
func GroupHealthStatus(hasTraffic bool, successRate *float64) string {
	if !hasTraffic || successRate == nil {
		return GroupHealthIdle
	}
	switch {
	case *successRate >= 0.99:
		return GroupHealthHealthy
	case *successRate >= 0.90:
		return GroupHealthDegraded
	default:
		return GroupHealthDown
	}
}

// groupStatusSuccessRate 成功调用率：success / (success + serviceErrors)。
// 无任何调用（分母为 0）时返回 nil。
func groupStatusSuccessRate(success, serviceErrors int64) *float64 {
	total := success + serviceErrors
	if total <= 0 {
		return nil
	}
	rate := float64(success) / float64(total)
	return &rate
}

// groupStatusTierStats 把 (group, tier) 聚合行拆成 fast / normal / overall 三组指标。
// overall 直接对全部行聚合（而非两组均值再平均），与 EMA 口径完全一致。
func groupStatusTierStats(rows []GroupStatusUsageRow) (fast, normal, overall GroupStatusTierStats) {
	var fastRows, normalRows []GroupStatusUsageRow
	for i := range rows {
		if rows[i].Tier == GroupStatusTierFast {
			fastRows = append(fastRows, rows[i])
		} else {
			normalRows = append(normalRows, rows[i])
		}
	}
	fast = groupStatusStatsFromRows(fastRows)
	normal = groupStatusStatsFromRows(normalRows)
	overall = groupStatusStatsFromRows(rows)
	return fast, normal, overall
}

// groupStatusStatsFromRows 把同一模式的聚合行合并为 EMA 加权指标。
// 各指标独立计算：decode 速度 = Σnum/Σden（只有 decode 有效的样本参与）。
func groupStatusStatsFromRows(rows []GroupStatusUsageRow) GroupStatusTierStats {
	stats := GroupStatusTierStats{}
	for _, row := range rows {
		stats.Requests += row.Requests
	}
	if stats.Requests == 0 {
		return stats
	}
	stats.DecodeSpeedTPS = emaRatio(emaSum(rows, rowSpeedNum), emaSum(rows, rowSpeedDen))
	stats.TTFTMs = emaRatio(emaSum(rows, rowTTFTNum), emaSum(rows, rowTTFTDen))
	stats.CacheRate = emaRatio(emaSum(rows, rowCacheNum), emaSum(rows, rowCacheDen))
	return stats
}

func emaSum(rows []GroupStatusUsageRow, pick func(*GroupStatusUsageRow) float64) float64 {
	var sum float64
	for i := range rows {
		sum += pick(&rows[i])
	}
	return sum
}

// emaRatio 加权平均：Σnum/Σden；den<=0（无有效样本）时返回 nil。
func emaRatio(num, den float64) *float64 {
	if den <= 0 {
		return nil
	}
	v := num / den
	return &v
}

func rowSpeedNum(r *GroupStatusUsageRow) float64  { return r.SpeedNum }
func rowSpeedDen(r *GroupStatusUsageRow) float64  { return r.SpeedDen }
func rowTTFTNum(r *GroupStatusUsageRow) float64   { return r.TTFTNum }
func rowTTFTDen(r *GroupStatusUsageRow) float64   { return r.TTFTDen }
func rowCacheNum(r *GroupStatusUsageRow) float64  { return r.CacheNum }
func rowCacheDen(r *GroupStatusUsageRow) float64  { return r.CacheDen }
