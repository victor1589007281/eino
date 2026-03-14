.PHONY: all build run test clean docker-up docker-down lint help \
       build-go build-ts test-go test-ts install-plugin e2e

GO_SERVICE_DIR := go-collab-service
TS_PLUGIN_DIR  := ts-plugins/collab
OPENCLAW_DIR   ?= $(HOME)/.openclaw
PLUGIN_DEST    := $(OPENCLAW_DIR)/plugins/@team/collab

# ==============================================================================
# Build
# ==============================================================================

all: build

build: build-go build-ts
	@echo "✅ All builds complete."

build-go:
	@echo ">>> Building Go service..."
	cd $(GO_SERVICE_DIR) && go build -o bin/collab-service ./cmd/server
	@echo "    Binary: $(GO_SERVICE_DIR)/bin/collab-service"

build-ts:
	@echo ">>> Building TS plugin..."
	cd $(TS_PLUGIN_DIR) && npm install && npm run build
	@echo "    Output: $(TS_PLUGIN_DIR)/dist/"

# ==============================================================================
# Run
# ==============================================================================

run-go:
	@echo ">>> Starting Go service (localhost:8090)..."
	cd $(GO_SERVICE_DIR) && go run ./cmd/server

run-go-bg:
	@echo ">>> Starting Go service in background..."
	cd $(GO_SERVICE_DIR) && nohup go run ./cmd/server > /tmp/collab-service.log 2>&1 &
	@echo "    PID: $$! | Log: /tmp/collab-service.log"

stop-go:
	@-lsof -ti:8090 | xargs kill 2>/dev/null && echo ">>> Go service stopped." || echo ">>> No Go service running."

# ==============================================================================
# Test
# ==============================================================================

test: test-go test-ts
	@echo "✅ All tests passed."

test-go:
	@echo ">>> Running Go unit tests..."
	cd $(GO_SERVICE_DIR) && go test -v -cover -count=1 ./...

test-ts:
	@echo ">>> Running TS unit tests..."
	cd $(TS_PLUGIN_DIR) && npx jest --no-cache --coverage

e2e: build run-go-bg
	@echo ">>> Waiting for Go service..."
	@sleep 3
	@echo "=== E2E: Health Check ==="
	curl -sf http://localhost:8090/health | python3 -m json.tool
	@echo "\n=== E2E: Register Agent ==="
	curl -sf -X POST http://localhost:8090/api/v1/agents/register \
	  -H 'Content-Type: application/json' \
	  -d '{"id":"e2e-arch","display_name":"架构师","emoji":"🏗️","role":"architect","skills":["design"]}' | python3 -m json.tool
	@echo "\n=== E2E: Create Project ==="
	curl -sf -X POST http://localhost:8090/api/v1/projects \
	  -H 'Content-Type: application/json' \
	  -d '{"name":"E2E测试项目","group_id":"oc_e2e_'$$RANDOM'","team_agents":["e2e-arch"],"tech_stack":["Go"]}' | python3 -m json.tool
	@echo "\n=== E2E: Create Task ==="
	curl -sf -X POST http://localhost:8090/api/v1/tasks \
	  -H 'Content-Type: application/json' \
	  -d '{"project_id":"oc_e2e","title":"E2E任务","assignee":"e2e-arch","priority":"P1"}' | python3 -m json.tool
	@echo "\n=== E2E: Dashboard ==="
	curl -sf http://localhost:8090/api/v1/dashboard/overview | python3 -m json.tool
	@echo "\n✅ E2E tests complete."
	@$(MAKE) stop-go

# ==============================================================================
# Docker
# ==============================================================================

docker-up:
	docker compose up -d --build 2>/dev/null || docker-compose up -d --build
	@echo "✅ Services up. Go: http://localhost:8090"

docker-down:
	docker compose down 2>/dev/null || docker-compose down

docker-logs:
	docker compose logs -f 2>/dev/null || docker-compose logs -f

docker-restart:
	$(MAKE) docker-down && $(MAKE) docker-up

docker-build-images:
	@echo ">>> Building Docker images..."
	docker build -t collab-go-service:latest ./$(GO_SERVICE_DIR)
	docker build -t collab-ts-plugin:latest ./$(TS_PLUGIN_DIR)
	@echo "✅ Images built."

# ==============================================================================
# Database
# ==============================================================================

db-shell:
	docker compose exec postgres psql -U collab -d collab 2>/dev/null || \
	docker exec collab-postgres psql -U collab -d collab 2>/dev/null || \
	psql "host=localhost user=collab password=collab dbname=collab"

db-migrate:
	@echo ">>> Running SQL migration..."
	docker compose exec -T postgres psql -U collab -d collab < $(GO_SERVICE_DIR)/migrations/000001_init.up.sql 2>/dev/null || \
	psql "host=localhost user=collab password=collab dbname=collab" < $(GO_SERVICE_DIR)/migrations/000001_init.up.sql
	@echo "✅ Migration complete."

db-reset:
	@echo "⚠️  This will destroy all data!"
	docker compose exec -T postgres psql -U collab -d collab < $(GO_SERVICE_DIR)/migrations/000001_init.down.sql 2>/dev/null || true
	docker compose exec -T postgres psql -U collab -d collab < $(GO_SERVICE_DIR)/migrations/000001_init.up.sql 2>/dev/null || true
	@echo "✅ Database reset."

# ==============================================================================
# OpenClaw Plugin Install
# ==============================================================================

install-plugin: build-ts
	@echo ">>> Installing @team/collab to OpenClaw..."
	@mkdir -p $(PLUGIN_DEST)
	cp -r $(TS_PLUGIN_DIR)/dist $(PLUGIN_DEST)/
	cp $(TS_PLUGIN_DIR)/package.json $(PLUGIN_DEST)/
	cp $(TS_PLUGIN_DIR)/openclaw.plugin.json $(PLUGIN_DEST)/
	@echo "✅ Plugin installed to $(PLUGIN_DEST)"
	@echo "   Restart OpenClaw to load the plugin."

uninstall-plugin:
	rm -rf $(PLUGIN_DEST)
	@echo "✅ Plugin removed. Restart OpenClaw."

# ==============================================================================
# K8s Deploy
# ==============================================================================

k8s-deploy:
	./deploy/scripts/k8s-deploy.sh

k8s-deploy-registry:
	@test -n "$(REGISTRY)" || (echo "Usage: make k8s-deploy-registry REGISTRY=xxx"; exit 1)
	./deploy/scripts/k8s-deploy.sh $(REGISTRY)

k8s-status:
	kubectl get pods,svc -n collab

k8s-logs:
	kubectl logs -n collab -l app=go-service -f --tail=50

k8s-destroy:
	kubectl delete namespace collab

smoke-test:
	./deploy/scripts/smoke-test.sh $(or $(GO_SERVICE_URL),http://localhost:8090)

install-test-agent:
	./deploy/scripts/install-test-agent.sh $(OPENCLAW_DIR)

# ==============================================================================
# Lint
# ==============================================================================

lint: lint-go lint-ts

lint-go:
	cd $(GO_SERVICE_DIR) && go vet ./...

lint-ts:
	cd $(TS_PLUGIN_DIR) && npx eslint src/ --ext .ts 2>/dev/null || true

# ==============================================================================
# Clean
# ==============================================================================

clean:
	rm -rf $(GO_SERVICE_DIR)/bin
	rm -rf $(TS_PLUGIN_DIR)/dist
	@echo "✅ Cleaned."

clean-all: clean
	rm -rf $(TS_PLUGIN_DIR)/node_modules
	@echo "✅ All cleaned (including node_modules)."

# ==============================================================================
# Help
# ==============================================================================

help:
	@echo ""
	@echo "=== OpenClaw Multi-Agent Collaboration System ==="
	@echo ""
	@echo "  Build & Run:"
	@echo "    make build            Build Go service + TS plugin"
	@echo "    make build-go         Build Go binary"
	@echo "    make build-ts         Build TS plugin"
	@echo "    make run-go           Run Go service locally"
	@echo "    make run-go-bg        Run Go service in background"
	@echo "    make stop-go          Stop background Go service"
	@echo ""
	@echo "  Test:"
	@echo "    make test             Run all tests (Go + TS)"
	@echo "    make test-go          Go unit tests with coverage"
	@echo "    make test-ts          TS unit tests with coverage"
	@echo "    make e2e              Build + start + E2E curl tests"
	@echo ""
	@echo "  Docker:"
	@echo "    make docker-up        Start PG + Go via Docker Compose"
	@echo "    make docker-down      Stop all containers"
	@echo "    make docker-logs      Tail container logs"
	@echo "    make docker-restart   Rebuild and restart"
	@echo "    make docker-build-images  Build Docker images only"
	@echo ""
	@echo "  Database:"
	@echo "    make db-shell         Open psql shell"
	@echo "    make db-migrate       Run SQL migrations"
	@echo "    make db-reset         Reset database (⚠️ destroys data)"
	@echo ""
	@echo "  OpenClaw Integration:"
	@echo "    make install-plugin   Install TS plugin to OpenClaw"
	@echo "    make uninstall-plugin Remove TS plugin from OpenClaw"
	@echo ""
	@echo "  K8s:"
	@echo "    make k8s-deploy       Deploy to K8s (local images)"
	@echo "    make k8s-deploy-registry REGISTRY=xxx  Deploy with registry"
	@echo "    make k8s-status       Show K8s pods and services"
	@echo "    make k8s-logs         Tail Go service logs in K8s"
	@echo "    make k8s-destroy      Delete K8s namespace"
	@echo "    make smoke-test       Run 10-point API smoke test"
	@echo "    make install-test-agent Install test agent to OpenClaw"
	@echo ""
	@echo "  Misc:"
	@echo "    make lint             Run linters"
	@echo "    make clean            Clean build artifacts"
	@echo "    make clean-all        Clean all (including node_modules)"
	@echo ""
