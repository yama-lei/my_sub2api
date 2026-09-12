package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// groupStatusRepository 分组状态页的只读聚合查询实现。
//
// 数据源：
//   - usage_logs：成功（计费）调用，提供 decode 速度 / TTFT / 缓存率的 EMA 加权聚合；
//   - ops_error_logs：错误调用计数，用于计算 uptime（成功调用率）。
type groupStatusRepository struct {
	db *sql.DB
}

// NewGroupStatusRepository 构造分组状态仓储。
func NewGroupStatusRepository(db *sql.DB) service.GroupStatusRepository {
	return &groupStatusRepository{db: db}
}

const groupStatusQueryTimeout = 3 * time.Second

// groupTierStatsSQL 按 (group, tier) 聚合 24h 窗口内的 EMA 加权指标。
//
// EMA 权重：w = exp(-ln2 · age / halfLife)，即按半衰期做时间指数衰减，
// 等价于对窗口内样本按时间做流式 EMA 的加权平均形态。
// decode 速度样本需同时满足：有 first_token_ms、总时长大于 TTFT、有输出 token。
const groupTierStatsSQL = `
WITH window_rows AS (
	SELECT
		group_id,
		CASE WHEN service_tier IN ('priority','fast') THEN 'fast' ELSE 'normal' END AS tier,
		input_tokens,
		output_tokens,
		cache_creation_tokens,
		cache_read_tokens,
		first_token_ms,
		GREATEST(COALESCE(duration_ms, 0) - COALESCE(first_token_ms, 0), 0) AS decode_ms,
		exp(-ln(2) * GREATEST(EXTRACT(EPOCH FROM ($3::timestamptz - created_at)), 0) / $4::float8) AS w
	FROM usage_logs
	WHERE created_at >= $1::timestamptz AND created_at < $2::timestamptz AND group_id IS NOT NULL
)
SELECT
	group_id,
	tier,
	COUNT(*) AS requests,
	COALESCE(SUM(CASE WHEN decode_ms > 0 AND output_tokens > 0
		THEN output_tokens * w / (decode_ms / 1000.0) ELSE 0 END), 0) AS speed_num,
	COALESCE(SUM(CASE WHEN decode_ms > 0 AND output_tokens > 0 THEN w ELSE 0 END), 0) AS speed_den,
	COALESCE(SUM(CASE WHEN first_token_ms IS NOT NULL THEN first_token_ms * w ELSE 0 END), 0) AS ttft_num,
	COALESCE(SUM(CASE WHEN first_token_ms IS NOT NULL THEN w ELSE 0 END), 0) AS ttft_den,
	COALESCE(SUM(cache_read_tokens * w), 0) AS cache_num,
	COALESCE(SUM((input_tokens + cache_creation_tokens + cache_read_tokens) * w), 0) AS cache_den
FROM window_rows
GROUP BY group_id, tier
`

// groupErrorStatsSQL 按 group 聚合窗口内错误数；业务限制（余额/配额等用户侧原因）
// 单列，不计入 uptime 的失败分母。
const groupErrorStatsSQL = `
SELECT
	group_id,
	COUNT(*) AS total_errors,
	COUNT(*) FILTER (WHERE NOT COALESCE(is_business_limited, false)) AS service_errors
FROM ops_error_logs
WHERE created_at >= $1::timestamptz AND group_id IS NOT NULL
GROUP BY group_id
`

// GetGroupTierStats 实现 service.GroupStatusRepository。
func (r *groupStatusRepository) GetGroupTierStats(ctx context.Context, since, until time.Time, halfLife time.Duration) ([]service.GroupStatusUsageRow, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil group status repository")
	}
	halfLifeSeconds := halfLife.Seconds()
	if halfLifeSeconds <= 0 {
		halfLifeSeconds = float64((time.Duration(6) * time.Hour).Seconds())
	}
	queryCtx, cancel := context.WithTimeout(ctx, groupStatusQueryTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(queryCtx, groupTierStatsSQL, since.UTC(), until.UTC(), until.UTC(), halfLifeSeconds)
	if err != nil {
		return nil, fmt.Errorf("query group tier stats: %w", err)
	}
	defer rows.Close()

	out := make([]service.GroupStatusUsageRow, 0, 16)
	for rows.Next() {
		var row service.GroupStatusUsageRow
		if err := rows.Scan(&row.GroupID, &row.Tier, &row.Requests,
			&row.SpeedNum, &row.SpeedDen, &row.TTFTNum, &row.TTFTDen, &row.CacheNum, &row.CacheDen); err != nil {
			return nil, fmt.Errorf("scan group tier stats: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetGroupErrorStats 实现 service.GroupStatusRepository。
func (r *groupStatusRepository) GetGroupErrorStats(ctx context.Context, since time.Time) ([]service.GroupStatusErrorRow, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil group status repository")
	}
	queryCtx, cancel := context.WithTimeout(ctx, groupStatusQueryTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(queryCtx, groupErrorStatsSQL, since.UTC())
	if err != nil {
		// ops_error_logs 缺表等情况下不阻塞状态页主指标，仅降级为无错误数据。
		if !tableMissing(queryCtx, r.db, "ops_error_logs") {
			return nil, fmt.Errorf("query group error stats: %w", err)
		}
		return []service.GroupStatusErrorRow{}, nil
	}
	defer rows.Close()

	out := make([]service.GroupStatusErrorRow, 0, 16)
	for rows.Next() {
		var row service.GroupStatusErrorRow
		if err := rows.Scan(&row.GroupID, &row.Total, &row.Service); err != nil {
			return nil, fmt.Errorf("scan group error stats: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// tableMissing 判断错误是否因目标表不存在（fresh 部署可能尚未产生 ops 表）。
func tableMissing(ctx context.Context, db *sql.DB, table string) bool {
	const checkSQL = `SELECT to_regclass($1::regclass) IS NOT NULL`
	var exists bool
	if err := db.QueryRowContext(ctx, checkSQL, "public."+table).Scan(&exists); err != nil {
		return false
	}
	return !exists
}
