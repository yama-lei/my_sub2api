package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountQualityRepository struct {
	db *sql.DB
}

func NewAccountQualityRepository(db *sql.DB) service.AccountQualityRepository {
	return &accountQualityRepository{db: db}
}

func (r *accountQualityRepository) InsertEvent(ctx context.Context, event *service.AccountQualityEvent) error {
	var detail any
	if len(event.Detail) > 0 {
		detail = []byte(event.Detail)
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return r.db.QueryRowContext(ctx, `
		INSERT INTO account_quality_events (account_id, kind, requested_model, observed_model, verdict, score, detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, event.AccountID, event.Kind, event.RequestedModel, event.ObservedModel, event.Verdict, event.Score, detail, createdAt,
	).Scan(&event.ID, &event.CreatedAt)
}

const accountQualityEventColumns = `id, account_id, kind, requested_model, observed_model, verdict, score, detail, created_at`

func scanAccountQualityEvent(scanner interface{ Scan(...any) error }) (*service.AccountQualityEvent, error) {
	ev := &service.AccountQualityEvent{}
	var score sql.NullFloat64
	var detail []byte
	if err := scanner.Scan(&ev.ID, &ev.AccountID, &ev.Kind, &ev.RequestedModel, &ev.ObservedModel, &ev.Verdict, &score, &detail, &ev.CreatedAt); err != nil {
		return nil, err
	}
	if score.Valid {
		v := score.Float64
		ev.Score = &v
	}
	if len(detail) > 0 {
		ev.Detail = detail
	}
	return ev, nil
}

func (r *accountQualityRepository) ListEvents(ctx context.Context, accountID int64, kind string, limit int) ([]*service.AccountQualityEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+accountQualityEventColumns+`
		FROM account_quality_events
		WHERE account_id = $1 AND ($2 = '' OR kind = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, accountID, kind, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.AccountQualityEvent, 0, limit)
	for rows.Next() {
		ev, err := scanAccountQualityEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *accountQualityRepository) UsageStats(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]*service.AccountUsageQualityStats, error) {
	// TTFT and TPS only count streamed requests that produced a first token;
	// generation time is the span after the first token.
	rows, err := r.db.QueryContext(ctx, `
		SELECT account_id,
			COUNT(*),
			COUNT(*) FILTER (WHERE upstream_model_mismatch IS TRUE),
			COUNT(*) FILTER (WHERE stream),
			AVG(first_token_ms) FILTER (WHERE stream AND first_token_ms > 0),
			percentile_cont(0.5) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE stream AND first_token_ms > 0),
			percentile_cont(0.9) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE stream AND first_token_ms > 0),
			COALESCE(SUM(output_tokens) FILTER (WHERE stream AND first_token_ms > 0 AND duration_ms > first_token_ms AND output_tokens > 0), 0),
			COALESCE(SUM(duration_ms - first_token_ms) FILTER (WHERE stream AND first_token_ms > 0 AND duration_ms > first_token_ms AND output_tokens > 0), 0),
			AVG(duration_ms) FILTER (WHERE duration_ms > 0)
		FROM usage_logs
		WHERE account_id = ANY($1) AND created_at >= $2
		GROUP BY account_id
	`, pq.Array(accountIDs), since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]*service.AccountUsageQualityStats, len(accountIDs))
	for rows.Next() {
		st := &service.AccountUsageQualityStats{}
		var avgTTFT, p50, p90, avgDuration sql.NullFloat64
		if err := rows.Scan(&st.AccountID, &st.Requests, &st.Mismatches, &st.StreamRequests, &avgTTFT, &p50, &p90, &st.GenTokens, &st.GenMs, &avgDuration); err != nil {
			return nil, err
		}
		st.AvgTTFTMs = nullFloatPtr(avgTTFT)
		st.P50TTFTMs = nullFloatPtr(p50)
		st.P90TTFTMs = nullFloatPtr(p90)
		st.AvgDurationMs = nullFloatPtr(avgDuration)
		out[st.AccountID] = st
	}
	return out, rows.Err()
}

func nullFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func (r *accountQualityRepository) EventStats(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]*service.AccountEventQualityStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT account_id,
			COUNT(*) FILTER (WHERE kind = 'downgrade'),
			COUNT(*) FILTER (WHERE kind = 'probe' AND verdict IN ('pass', 'fail')),
			COUNT(*) FILTER (WHERE kind = 'probe' AND verdict = 'fail')
		FROM account_quality_events
		WHERE account_id = ANY($1) AND created_at >= $2
		GROUP BY account_id
	`, pq.Array(accountIDs), since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]*service.AccountEventQualityStats, len(accountIDs))
	for rows.Next() {
		st := &service.AccountEventQualityStats{}
		if err := rows.Scan(&st.AccountID, &st.Downgrades, &st.Probes, &st.ProbeFails); err != nil {
			return nil, err
		}
		out[st.AccountID] = st
	}
	return out, rows.Err()
}

func (r *accountQualityRepository) LatestProbes(ctx context.Context, accountIDs []int64) (map[int64]*service.AccountQualityEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (account_id) `+accountQualityEventColumns+`
		FROM account_quality_events
		WHERE account_id = ANY($1) AND kind = 'probe'
		ORDER BY account_id, created_at DESC
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]*service.AccountQualityEvent, len(accountIDs))
	for rows.Next() {
		ev, err := scanAccountQualityEvent(rows)
		if err != nil {
			return nil, err
		}
		out[ev.AccountID] = ev
	}
	return out, rows.Err()
}

func (r *accountQualityRepository) ListProbeEnabledAccountIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id FROM accounts
		WHERE deleted_at IS NULL AND extra->>'model_probe_enabled' = 'true'
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *accountQualityRepository) DeleteEventsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM account_quality_events WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
