# ============================================================================
# RockGame - Distributed Gaming Lobby Platform
# Build Automation Makefile
# ============================================================================

.PHONY: build gate \
	node-account node-game node-admin node-lobby node-event \
	mesh-activity mesh-shop mesh-vip mesh-task mesh-mail mesh-rank mesh-agent mesh-item mesh-tag mesh-reddot \
	proto lint clean docker migrate bootstrap-routes help

# Variables
BINARY_DIR := ./bin
GO := go
GOFLAGS := -v
LDFLAGS := -s -w
BUILD_TIME := $(shell date +%Y%m%d%H%M%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_FLAGS := -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

# Ensure binary directory exists
$(BINARY_DIR):
	@mkdir -p $(BINARY_DIR)

# ============================================================================
# Build Targets
# ============================================================================

## build: Build all components
build: gate node-account node-game node-admin node-lobby node-event \
	mesh-activity mesh-shop mesh-vip mesh-task mesh-mail mesh-rank mesh-agent mesh-item mesh-tag mesh-reddot
	@echo "=== All components built successfully ==="

## gate: Build gate server
gate: $(BINARY_DIR)
	@echo "=== Building gate server ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/gate ./cmd/gate

## node-account: Build account node
node-account: $(BINARY_DIR)
	@echo "=== Building account node ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/node-account ./cmd/node/account

## node-game: Build game node
node-game: $(BINARY_DIR)
	@echo "=== Building game node ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/node-game ./cmd/node/game

## node-admin: Build admin node
node-admin: $(BINARY_DIR)
	@echo "=== Building admin node ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/node-admin ./cmd/node/admin

## node-lobby: Build lobby node
node-lobby: $(BINARY_DIR)
	@echo "=== Building lobby node ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/node-lobby ./cmd/node/lobby

## node-event: Build event node
node-event: $(BINARY_DIR)
	@echo "=== Building event node ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/node-event ./cmd/node/event

## mesh-activity: Build activity mesh
mesh-activity: $(BINARY_DIR)
	@echo "=== Building activity mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-activity ./cmd/mesh/activity

## mesh-shop: Build shop mesh
mesh-shop: $(BINARY_DIR)
	@echo "=== Building shop mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-shop ./cmd/mesh/shop

## mesh-vip: Build vip mesh
mesh-vip: $(BINARY_DIR)
	@echo "=== Building vip mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-vip ./cmd/mesh/vip

## mesh-task: Build task mesh
mesh-task: $(BINARY_DIR)
	@echo "=== Building task mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-task ./cmd/mesh/task

## mesh-mail: Build mail mesh
mesh-mail: $(BINARY_DIR)
	@echo "=== Building mail mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-mail ./cmd/mesh/mail

## mesh-rank: Build rank mesh
mesh-rank: $(BINARY_DIR)
	@echo "=== Building rank mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-rank ./cmd/mesh/rank

## mesh-agent: Build agent mesh
mesh-agent: $(BINARY_DIR)
	@echo "=== Building agent mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-agent ./cmd/mesh/agent

## mesh-item: Build item mesh
mesh-item: $(BINARY_DIR)
	@echo "=== Building item mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-item ./cmd/mesh/item

## mesh-tag: Build tag mesh
mesh-tag: $(BINARY_DIR)
	@echo "=== Building tag mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-tag ./cmd/mesh/tag

## mesh-reddot: Build reddot mesh
mesh-reddot: $(BINARY_DIR)
	@echo "=== Building reddot mesh ==="
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" \
		-o $(BINARY_DIR)/mesh-reddot ./cmd/mesh/reddot

# ============================================================================
# Code Generation
# ============================================================================

## proto: Generate protobuf code
proto:
	@echo "=== Generating protobuf code ==="
	@which protoc >/dev/null 2>&1 || (echo "ERROR: protoc not found. Install from https://github.com/protocolbuffers/protobuf"; exit 1)
	@which protoc-gen-go >/dev/null 2>&1 || (echo "ERROR: protoc-gen-go not found. Run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1)
	@which protoc-gen-go-grpc >/dev/null 2>&1 || (echo "ERROR: protoc-gen-go-grpc not found. Run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1)
	@for f in $$(find proto -name '*.proto'); do \
		protoc --go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			-I proto $$f; \
	done
	@echo "=== Protobuf code generation complete ==="

# ============================================================================
# Quality
# ============================================================================

## lint: Run golangci-lint
lint:
	@echo "=== Running golangci-lint ==="
	@which golangci-lint >/dev/null 2>&1 || (echo "ERROR: golangci-lint not found. Install from https://golangci-lint.run"; exit 1)
	golangci-lint run ./...

# ============================================================================
# Database
# ============================================================================

## migrate: Run database migrations
migrate:
	@echo "=== Running database migrations ==="
	$(GO) run $(GOFLAGS) ./cmd/migrate

## bootstrap-routes: Seed etcd with default route table (idempotent, ask for confirmation)
bootstrap-routes:
	@echo "=== Bootstrapping etcd routes ==="
	$(GO) run $(GOFLAGS) ./cmd/bootstrap -action seed

## bootstrap-routes-force: Seed etcd routes without confirmation (for CI/CD)
bootstrap-routes-force:
	@echo "=== Bootstrapping etcd routes (force) ==="
	$(GO) run $(GOFLAGS) ./cmd/bootstrap -action seed -force

## show-routes: Display routes currently in etcd
show-routes:
	@echo "=== Showing etcd routes ==="
	$(GO) run $(GOFLAGS) ./cmd/bootstrap -action show

## clean-routes: Delete all routes from etcd
clean-routes:
	@echo "=== Cleaning etcd routes ==="
	$(GO) run $(GOFLAGS) ./cmd/bootstrap -action clean

# ============================================================================
# Docker
# ============================================================================

## docker: Build docker images
docker:
	@echo "=== Building Docker images ==="
	@docker build -t rockgame-gate:latest     -f docker/Dockerfile.gate     .
	@docker build -t rockgame-node-account:latest -f docker/Dockerfile.node-account .
	@docker build -t rockgame-node-game:latest -f docker/Dockerfile.node-game .
	@docker build -t rockgame-mesh-activity:latest -f docker/Dockerfile.mesh-activity .
	@docker build -t rockgame-mesh-shop:latest -f docker/Dockerfile.mesh-shop .
	@echo "=== Docker images built successfully ==="

# ============================================================================
# Cleanup
# ============================================================================

## clean: Clean build artifacts
clean:
	@echo "=== Cleaning build artifacts ==="
	rm -rf $(BINARY_DIR)
	@echo "=== Clean complete ==="

# ============================================================================
# Help
# ============================================================================

## help: Show this help message
help:
	@echo "RockGame Build Targets:"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
