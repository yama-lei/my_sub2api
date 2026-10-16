-- Qoder CN OAuth uses the existing account quota monitor. Extend only the
-- monitor provider constraints; account/group platforms use application validation.
ALTER TABLE channel_monitors
    DROP CONSTRAINT IF EXISTS channel_monitors_provider_check;
ALTER TABLE channel_monitors
    ADD CONSTRAINT channel_monitors_provider_check
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'grok', 'qoder',
                        'antigravity', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'));

ALTER TABLE channel_monitor_request_templates
    DROP CONSTRAINT IF EXISTS channel_monitor_request_templates_provider_check;
ALTER TABLE channel_monitor_request_templates
    ADD CONSTRAINT channel_monitor_request_templates_provider_check
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'grok', 'qoder',
                        'antigravity', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'));
