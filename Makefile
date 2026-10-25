.PHONY: build build-backend build-frontend run generate docker-build docker-dev image deploy test test-backend test-unit test-integration test-e2e-local test-frontend test-frontend-critical

DOCKER_DEV := docker compose --env-file deploy/.env -p sub2api-dev -f deploy/docker-compose.dev.yml

VERSION ?= $(shell sh backend/scripts/resolve-version.sh)
LDFLAGS ?= -s -w -X main.Version=$(VERSION)

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/api/__tests__/qoderUsage.spec.ts \
	src/api/__tests__/relayAccountWorkspace.spec.ts \
	src/api/__tests__/relayQuotaReset.spec.ts \
	src/composables/__tests__/useModelWhitelist.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts \
	src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts \
	src/constants/__tests__/platforms.spec.ts \
	src/components/account/__tests__/credentialsBuilder.platformCatalog.spec.ts \
	src/components/account/__tests__/CreateAccountModal.spec.ts \
	src/components/account/__tests__/EditAccountModal.spec.ts \
	src/components/account/__tests__/credentialsBuilder.spec.ts \
	src/components/account/__tests__/QoderOAuthFlow.spec.ts \
	src/components/account/__tests__/QoderUsageCell.spec.ts \
	src/components/account/__tests__/AccountUsageCell.spec.ts \
	src/components/account/__tests__/ClaudeResetCreditsCell.spec.ts \
	src/components/account/__tests__/OpenCodeGoProtocolRulesEditor.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/views/user/__tests__/RelayView.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/views/admin/__tests__/groupsMessagesDispatch.spec.ts \
	src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts \
	src/components/admin/channel/__tests__/PricingEntryCard.modelDefaultPrice.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（不嵌入前端）
build-backend:
	@cd backend && CGO_ENABLED=0 go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -trimpath -o bin/server ./cmd/server

# 重新生成 Ent 与 Wire 代码
generate:
	@cd backend && go generate ./ent && go generate ./cmd/server

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 先编译前端，再嵌入后端二进制并启动。
run: build-frontend
	@$(MAKE) build-backend GOFLAGS="$(GOFLAGS) -tags=embed"
	@cd backend && exec ./bin/server

docker-build:
	@$(DOCKER_DEV) build sub2api

docker-dev:
	@$(DOCKER_DEV) up --build

image:
	@docker build -t sub2api:latest -f Dockerfile .

deploy:
	@bash deploy/deploy.sh

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@cd backend && go test ./... && golangci-lint run ./...

test-unit:
	@cd backend && go test -tags=unit ./...

test-integration:
	@cd backend && go test -tags=integration ./...

test-e2e-local:
	@cd backend && go test -tags=e2e -v -timeout=300s ./internal/integration/...

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)
