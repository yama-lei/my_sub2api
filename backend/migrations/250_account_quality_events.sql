-- Per-account quality signals that are not visible in usage_logs:
--   downgrade: an upstream attempt that declared a different model than the one
--              sent and was abandoned by the model downgrade guard (the retried
--              attempt is what lands in usage_logs).
--   probe:     a scheduled ModelTrace fingerprint check of the account.
CREATE TABLE IF NOT EXISTS account_quality_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('downgrade', 'probe')),
    requested_model VARCHAR(255) NOT NULL DEFAULT '',
    observed_model VARCHAR(255) NOT NULL DEFAULT '',
    verdict VARCHAR(16) NOT NULL DEFAULT '',
    score DOUBLE PRECISION,
    detail JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_account_quality_events_account_kind_created
    ON account_quality_events(account_id, kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_quality_events_created
    ON account_quality_events(created_at);
