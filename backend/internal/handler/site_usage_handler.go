// Package handler provides HTTP request handlers for the application.
package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// siteUsageCacheTTL 与管理端 dashboard 查询缓存一致（30s），
// 避免多个用户同时打开全站用量页把聚合查询打到数据库。
const siteUsageCacheTTL = 30 * time.Second

// siteUsageCacheEntry 是全站用量查询缓存的条目。
type siteUsageCacheEntry struct {
	payload   any
	expiresAt time.Time
}

// siteUsageCache 是极简的 TTL 缓存（单进程内）。
type siteUsageCache struct {
	mu sync.RWMutex
	m  map[string]siteUsageCacheEntry
}

func newSiteUsageCache() *siteUsageCache {
	return &siteUsageCache{m: make(map[string]siteUsageCacheEntry)}
}

// GetOrLoad 返回缓存命中结果，或在未命中时加载并写入缓存。
func (c *siteUsageCache) GetOrLoad(key string, load func() (any, error)) (any, bool, error) {
	c.mu.RLock()
	entry, ok := c.m[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.payload, true, nil
	}

	payload, err := load()
	if err != nil {
		return nil, false, err
	}
	c.mu.Lock()
	c.m[key] = siteUsageCacheEntry{payload: payload, expiresAt: time.Now().Add(siteUsageCacheTTL)}
	c.mu.Unlock()
	return payload, false, nil
}

// SiteUsageHandler 面向所有登录用户的「全站用量」只读视图。
//
// 数据口径与 Admin Usage 页一致（同一批聚合查询），但对敏感信息做了脱敏：
//   - 用户邮箱 / Key 名称：保留前 2 后 2 个字符，中间以 ** 代替；
//   - 不返回 IP、User-Agent、Session ID、上游账号等管理员字段。
//
// 所有端点仅接受与个人无关的筛选（模型 / 分组 / 请求类型 / 时间范围），
// 不支持按 user_id / api_key_id 过滤，避免被用来定向窥探单个用户。
type SiteUsageHandler struct {
	usageService     *service.UsageService
	dashboardService *service.DashboardService
	cache            *siteUsageCache
}

// NewSiteUsageHandler creates a new site-wide (masked) usage handler.
func NewSiteUsageHandler(usageService *service.UsageService, dashboardService *service.DashboardService) *SiteUsageHandler {
	return &SiteUsageHandler{
		usageService:     usageService,
		dashboardService: dashboardService,
		cache:            newSiteUsageCache(),
	}
}

// ---------- 脱敏 DTO ----------

// siteUsageUser 脱敏后的用户信息（邮箱已打码）。
type siteUsageUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

// siteUsageAPIKey 脱敏后的 API Key 信息（名称已打码）。
type siteUsageAPIKey struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// siteUsageGroup 精简分组信息（不含费率 / 额度配置）。
type siteUsageGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

// siteUsageLog 全站用量记录（脱敏版）。
type siteUsageLog struct {
	ID                  int64            `json:"id"`
	CreatedAt           time.Time        `json:"created_at"`
	UserID              int64            `json:"user_id"`
	User                *siteUsageUser   `json:"user,omitempty"`
	APIKeyID            int64            `json:"api_key_id"`
	APIKey              *siteUsageAPIKey `json:"api_key,omitempty"`
	GroupID             *int64           `json:"group_id,omitempty"`
	Group               *siteUsageGroup  `json:"group,omitempty"`
	RequestID           string           `json:"request_id"`
	Model               string           `json:"model"`
	ServiceTier         *string          `json:"service_tier,omitempty"`
	ReasoningEffort     *string          `json:"reasoning_effort,omitempty"`
	InboundEndpoint     *string          `json:"inbound_endpoint,omitempty"`
	RequestType         string           `json:"request_type"`
	Stream              bool             `json:"stream"`
	BillingType         int8             `json:"billing_type"`
	BillingMode         *string          `json:"billing_mode,omitempty"`
	InputTokens         int64            `json:"input_tokens"`
	OutputTokens        int64            `json:"output_tokens"`
	CacheCreationTokens int64            `json:"cache_creation_tokens"`
	CacheReadTokens     int64            `json:"cache_read_tokens"`
	InputCost           float64          `json:"input_cost"`
	OutputCost          float64          `json:"output_cost"`
	CacheCreationCost   float64          `json:"cache_creation_cost"`
	CacheReadCost       float64          `json:"cache_read_cost"`
	TotalCost           float64          `json:"total_cost"`
	ActualCost          float64          `json:"actual_cost"`
	RateMultiplier      float64          `json:"rate_multiplier"`
	DurationMs          *int             `json:"duration_ms,omitempty"`
	FirstTokenMs        *int             `json:"first_token_ms,omitempty"`
	ImageCount          int              `json:"image_count"`
	VideoCount          int              `json:"video_count"`
}

// MaskSiteName 对用户名 / 邮箱 / Key 名做脱敏：保留前 2 后 2 个字符，中间以 ** 代替。
// ≤2 个字符整体替换为 **；3–5 个字符保留首尾各 1 个字符。
func MaskSiteName(value string) string {
	runes := []rune(strings.TrimSpace(value))
	n := len(runes)
	switch {
	case n == 0:
		return ""
	case n <= 2:
		return "**"
	case n <= 5:
		return string(runes[:1]) + "**" + string(runes[n-1:])
	default:
		return string(runes[:2]) + "**" + string(runes[n-2:])
	}
}

func siteUsageLogFromService(l *service.UsageLog) siteUsageLog {
	requestType := l.EffectiveRequestType()
	stream, _ := service.ApplyLegacyRequestFields(requestType, l.Stream, l.OpenAIWSMode)
	requestedModel := l.RequestedModel
	if requestedModel == "" {
		requestedModel = l.Model
	}

	out := siteUsageLog{
		ID:                  l.ID,
		CreatedAt:           l.CreatedAt,
		UserID:              l.UserID,
		APIKeyID:            l.APIKeyID,
		GroupID:             l.GroupID,
		RequestID:           l.RequestID,
		Model:               requestedModel,
		ServiceTier:         l.ServiceTier,
		ReasoningEffort:     l.ReasoningEffort,
		InboundEndpoint:     l.InboundEndpoint,
		RequestType:         requestType.String(),
		Stream:              stream,
		BillingType:         l.BillingType,
		BillingMode:         l.BillingMode,
		InputTokens:         int64(l.InputTokens),
		OutputTokens:        int64(l.OutputTokens),
		CacheCreationTokens: int64(l.CacheCreationTokens),
		CacheReadTokens:     int64(l.CacheReadTokens),
		InputCost:           l.InputCost,
		OutputCost:          l.OutputCost,
		CacheCreationCost:   l.CacheCreationCost,
		CacheReadCost:       l.CacheReadCost,
		TotalCost:           l.TotalCost,
		ActualCost:          l.ActualCost,
		RateMultiplier:      l.RateMultiplier,
		DurationMs:          l.DurationMs,
		FirstTokenMs:        l.FirstTokenMs,
		ImageCount:          l.ImageCount,
		VideoCount:          l.VideoCount,
	}
	if l.User != nil {
		out.User = &siteUsageUser{ID: l.User.ID, Email: MaskSiteName(l.User.Email)}
	}
	if l.APIKey != nil {
		out.APIKey = &siteUsageAPIKey{ID: l.APIKey.ID, Name: MaskSiteName(l.APIKey.Name)}
	}
	if l.Group != nil {
		out.Group = &siteUsageGroup{ID: l.Group.ID, Name: l.Group.Name, Platform: l.Group.Platform}
	}
	return out
}

// siteUsageRankingItem 脱敏后的用户消耗排行。
type siteUsageRankingItem struct {
	UserID       int64   `json:"user_id"`
	Email        string  `json:"email"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheTokens  int64   `json:"cache_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	Cost         float64 `json:"cost"`
	ActualCost   float64 `json:"actual_cost"`
}

// ---------- 筛选解析（与管理端一致的口径，但不含 user/api_key/account 维度） ----------

type siteUsageFilters struct {
	GroupID            int64
	Model              string
	RequestType        *int16
	Stream             *bool
	NativeCompactionV2 *bool
	BillingType        *int8
	BillingMode        string
	StartTime          *time.Time
	EndTime            *time.Time
}

func (f *siteUsageFilters) toUsageLogFilters(exactTotal bool) usagestats.UsageLogFilters {
	return usagestats.UsageLogFilters{
		GroupID:            f.GroupID,
		Model:              f.Model,
		ModelFilterSource:  usagestats.ModelSourceRequested,
		RequestType:        f.RequestType,
		Stream:             f.Stream,
		NativeCompactionV2: f.NativeCompactionV2,
		BillingType:        f.BillingType,
		BillingMode:        f.BillingMode,
		StartTime:          f.StartTime,
		EndTime:            f.EndTime,
		ExactTotal:         exactTotal,
	}
}

// toUsageLogFiltersWithRange 显式指定时间范围（半开区间 [start, end)），
// 用于 trend / model-stats / ranking 等始终带时间边界的聚合查询。
func (f *siteUsageFilters) toUsageLogFiltersWithRange(startTime, endTime time.Time) usagestats.UsageLogFilters {
	filters := f.toUsageLogFilters(false)
	filters.StartTime = &startTime
	filters.EndTime = &endTime
	return filters
}

func parseSiteUsageBool(c *gin.Context, name string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// parseSiteUsageFilters 解析通用查询参数。requireDateRange=true 时（stats 端点）
// 未传日期则回退到 period=today/week/month 语义，与管理端一致。
func parseSiteUsageFilters(c *gin.Context, requireDateRange bool) (*siteUsageFilters, error) {
	filters := &siteUsageFilters{
		Model: strings.TrimSpace(c.Query("model")),
	}

	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			return nil, err
		}
		filters.GroupID = id
	}

	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			return nil, err
		}
		value := int16(parsed)
		filters.RequestType = &value
	} else if streamStr := strings.TrimSpace(c.Query("stream")); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			return nil, err
		}
		filters.Stream = &val
	}

	var err error
	if filters.NativeCompactionV2, err = parseSiteUsageBool(c, "native_compaction_v2"); err != nil {
		return nil, err
	}

	if billingTypeStr := strings.TrimSpace(c.Query("billing_type")); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			return nil, err
		}
		bt := int8(val)
		filters.BillingType = &bt
	}

	filters.BillingMode = strings.TrimSpace(c.Query("billing_mode"))

	userTZ := c.Query("timezone")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	switch {
	case startDateStr != "" || endDateStr != "":
		// 与管理端一致：start/end 各自独立生效。
		if startDateStr != "" {
			startTime, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
			if err != nil {
				return nil, err
			}
			filters.StartTime = &startTime
		}
		if endDateStr != "" {
			endTime, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
			if err != nil {
				return nil, err
			}
			// 半开区间 [start, end)：end 取次日 00:00（DST 安全），与管理端一致。
			endTime = endTime.AddDate(0, 0, 1)
			filters.EndTime = &endTime
		}
	case requireDateRange:
		now := timezone.NowInUserLocation(userTZ)
		var startTime time.Time
		switch c.DefaultQuery("period", "today") {
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		default:
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		}
		filters.StartTime = &startTime
		filters.EndTime = &now
	}

	return filters, nil
}

// ---------- 端点 ----------

// List returns a masked, site-wide usage record list.
// GET /api/v1/usage/site
func (h *SiteUsageHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)

	exactTotal := false
	if raw := strings.TrimSpace(c.Query("exact_total")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			response.BadRequest(c, "Invalid exact_total value, use true or false")
			return
		}
		exactTotal = parsed
	}

	filters, err := parseSiteUsageFilters(c, false)
	if err != nil {
		response.BadRequest(c, "Invalid filters: "+err.Error())
		return
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters.toUsageLogFilters(exactTotal))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]siteUsageLog, 0, len(records))
	for i := range records {
		out = append(out, siteUsageLogFromService(&records[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// Stats returns masked site-wide aggregate statistics.
// GET /api/v1/usage/site/stats
func (h *SiteUsageHandler) Stats(c *gin.Context) {
	filters, err := parseSiteUsageFilters(c, true)
	if err != nil {
		response.BadRequest(c, "Invalid filters: "+err.Error())
		return
	}

	cacheKey := "stats:" + mustMarshalSiteCacheKey(filters)
	nocache := strings.TrimSpace(c.Query("nocache")) == "1"

	var stats *usagestats.UsageStats
	if nocache {
		stats, err = h.usageService.GetStatsWithFilters(c.Request.Context(), filters.toUsageLogFilters(false))
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		c.Header("X-Usage-Stats-Cache", "bypass")
	} else {
		var hit bool
		var payload any
		payload, hit, err = h.cache.GetOrLoad(cacheKey, func() (any, error) {
			return h.usageService.GetStatsWithFilters(c.Request.Context(), filters.toUsageLogFilters(false))
		})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		stats, _ = payload.(*usagestats.UsageStats)
		c.Header("X-Usage-Stats-Cache", siteCacheStatusValue(hit))
	}
	if stats == nil {
		response.Error(c, 500, "failed to load usage stats")
		return
	}

	// 全站视图只保留入站端点聚合；上游端点 / 端点路径属管理端内部信息。
	stats.UpstreamEndpoints = nil
	stats.EndpointPaths = nil
	response.Success(c, stats)
}

// SnapshotV2 returns site-wide trend + group distribution for charts.
// GET /api/v1/usage/site/dashboard/snapshot-v2
func (h *SiteUsageHandler) SnapshotV2(c *gin.Context) {
	startTime, endTime := parseSiteTimeRange(c)
	granularity := strings.TrimSpace(c.DefaultQuery("granularity", "day"))
	if granularity != "hour" {
		granularity = "day"
	}

	filters, err := parseSiteUsageFilters(c, false)
	if err != nil {
		response.BadRequest(c, "Invalid filters: "+err.Error())
		return
	}

	cacheKey := "snapshot:" + granularity + ":" + startTime.UTC().Format(time.RFC3339) + ":" + endTime.UTC().Format(time.RFC3339) + ":" + mustMarshalSiteCacheKey(filters)
	payload, _, err := h.cache.GetOrLoad(cacheKey, func() (any, error) {
		return h.buildSiteSnapshot(c.Request.Context(), startTime, endTime, granularity, filters)
	})
	if err != nil {
		response.Error(c, 500, "failed to load site usage snapshot")
		return
	}
	response.Success(c, payload)
}

func (h *SiteUsageHandler) buildSiteSnapshot(ctx context.Context, startTime, endTime time.Time, granularity string, filters *siteUsageFilters) (gin.H, error) {
	logFilters := filters.toUsageLogFiltersWithRange(startTime, endTime)

	trend, err := h.dashboardService.GetUsageTrendWithUsageFilters(ctx, startTime, endTime, granularity, logFilters)
	if err != nil {
		return nil, err
	}
	groups, err := h.dashboardService.GetGroupStatsWithUsageFilters(ctx, startTime, endTime, logFilters)
	if err != nil {
		return nil, err
	}

	return gin.H{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"start_date":   startTime.Format("2006-01-02"),
		"end_date":     endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity":  granularity,
		"trend":        trend,
		"groups":       groups,
	}, nil
}

// ModelStats returns site-wide per-model statistics (requested model names only).
// GET /api/v1/usage/site/dashboard/model-stats
func (h *SiteUsageHandler) ModelStats(c *gin.Context) {
	startTime, endTime := parseSiteTimeRange(c)

	filters, err := parseSiteUsageFilters(c, false)
	if err != nil {
		response.BadRequest(c, "Invalid filters: "+err.Error())
		return
	}

	cacheKey := "models:" + startTime.UTC().Format(time.RFC3339) + ":" + endTime.UTC().Format(time.RFC3339) + ":" + mustMarshalSiteCacheKey(filters)
	payload, _, err := h.cache.GetOrLoad(cacheKey, func() (any, error) {
		return h.dashboardService.GetModelStatsWithUsageFiltersBySource(
			c.Request.Context(), startTime, endTime, filters.toUsageLogFiltersWithRange(startTime, endTime), usagestats.ModelSourceRequested)
	})
	if err != nil {
		response.Error(c, 500, "failed to load model statistics")
		return
	}
	models, _ := payload.([]usagestats.ModelStat)
	if models == nil {
		models = []usagestats.ModelStat{}
	}
	response.Success(c, gin.H{"models": models})
}

// Ranking returns the masked site-wide per-user token/cost ranking.
// GET /api/v1/usage/site/ranking
func (h *SiteUsageHandler) Ranking(c *gin.Context) {
	startTime, endTime := parseSiteTimeRange(c)

	filters, err := parseSiteUsageFilters(c, false)
	if err != nil {
		response.BadRequest(c, "Invalid filters: "+err.Error())
		return
	}

	limit := 50
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	sortBy := strings.TrimSpace(c.Query("sort_by"))

	dim := usagestats.UserBreakdownDimension{
		GroupID:            filters.GroupID,
		Model:              filters.Model,
		ModelType:          usagestats.ModelSourceRequested,
		RequestType:        filters.RequestType,
		Stream:             filters.Stream,
		NativeCompactionV2: filters.NativeCompactionV2,
		BillingType:        filters.BillingType,
		SortBy:             sortBy,
	}

	cacheKey := "ranking:" + startTime.UTC().Format(time.RFC3339) + ":" + endTime.UTC().Format(time.RFC3339) + ":" + strconv.Itoa(limit) + ":" + sortBy + ":" + mustMarshalSiteCacheKey(filters)
	payload, _, err := h.cache.GetOrLoad(cacheKey, func() (any, error) {
		items, err := h.dashboardService.GetUserBreakdownStats(c.Request.Context(), startTime, endTime, dim, limit)
		if err != nil {
			return nil, err
		}
		out := make([]siteUsageRankingItem, 0, len(items))
		for _, item := range items {
			out = append(out, siteUsageRankingItem{
				UserID:       item.UserID,
				Email:        MaskSiteName(item.Email),
				Requests:     item.Requests,
				InputTokens:  item.InputTokens,
				OutputTokens: item.OutputTokens,
				CacheTokens:  item.CacheTokens,
				TotalTokens:  item.TotalTokens,
				Cost:         item.Cost,
				ActualCost:   item.ActualCost,
			})
		}
		return out, nil
	})
	if err != nil {
		response.Error(c, 500, "failed to load ranking")
		return
	}
	response.Success(c, payload)
}

// parseSiteTimeRange 与管理端 parseTimeRange 同语义：默认最近 7 天。
func parseSiteTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)

	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	if startDate := c.Query("start_date"); startDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = t
		}
	}

	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	if endDate := c.Query("end_date"); endDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = t.AddDate(0, 0, 1)
		}
	}

	return startTime, endTime
}

func siteCacheStatusValue(hit bool) string {
	if hit {
		return "hit"
	}
	return "miss"
}

// mustMarshalSiteCacheKey 把筛选条件序列化成稳定的缓存 key 片段。
func mustMarshalSiteCacheKey(filters *siteUsageFilters) string {
	var startTime, endTime *string
	if filters.StartTime != nil {
		v := filters.StartTime.UTC().Format(time.RFC3339)
		startTime = &v
	}
	if filters.EndTime != nil {
		v := filters.EndTime.UTC().Format(time.RFC3339)
		endTime = &v
	}
	raw, err := json.Marshal(struct {
		GroupID            int64   `json:"group_id"`
		Model              string  `json:"model"`
		RequestType        *int16  `json:"request_type"`
		Stream             *bool   `json:"stream"`
		NativeCompactionV2 *bool   `json:"native_compaction_v2"`
		BillingType        *int8   `json:"billing_type"`
		BillingMode        string  `json:"billing_mode"`
		StartTime          *string `json:"start_time"`
		EndTime            *string `json:"end_time"`
	}{
		GroupID:            filters.GroupID,
		Model:              filters.Model,
		RequestType:        filters.RequestType,
		Stream:             filters.Stream,
		NativeCompactionV2: filters.NativeCompactionV2,
		BillingType:        filters.BillingType,
		BillingMode:        filters.BillingMode,
		StartTime:          startTime,
		EndTime:            endTime,
	})
	if err != nil {
		return ""
	}
	return string(raw)
}
