# RockGame

**Distributed Gaming Lobby Platform**

RockGame is a high-performance distributed gaming lobby platform built on the [Due framework](https://github.com/due) (Go). It leverages a **Gate-Node-Mesh** architecture to deliver scalable, modular game services with 16 system modules.

## Architecture

```
              ┌──────────┐
              │  Client  │
              └────┬─────┘
                   │
              ┌────▼─────┐
              │   Gate   │  ─── WebSocket / TCP接入层
              └────┬─────┘
                   │
        ┌──────────┼──────────┐
        │                     │
   ┌────▼─────┐        ┌─────▼────┐
   │  Account  │        │   Game   │   ─── 业务节点 (Nodes)
   │   Node    │        │   Node   │
   └────┬─────┘        └─────┬────┘
        │                     │
   ┌────▼─────┐        ┌─────▼────┐
   │ Activity  │        │   Shop   │   ─── 功能网格 (Meshes)
   │   Mesh    │        │   Mesh   │
   └──────────┘        └──────────┘
```

- **Gate** — Entry point handling client connections (WebSocket / TCP), protocol parsing, and request routing.
- **Nodes** — Business logic units (Account, Game, Match, Chat, etc.) that process core game operations.
- **Meshes** — Lightweight service meshes (Activity, Shop, Rank, etc.) providing cross-node feature coordination.

## System Modules (16)

| #   | Module     | Type  | Description                |
|-----|------------|-------|----------------------------|
| 1   | Gate       | Gate  | Client connection gateway   |
| 2   | Account    | Node  | Registration & auth         |
| 3   | Player     | Node  | Player profile management    |
| 4   | Game       | Node  | Game room & session logic    |
| 5   | Match      | Node  | Matchmaking & queuing        |
| 6   | Chat       | Node  | Real-time messaging          |
| 7   | Friend     | Node  | Social & friend list         |
| 8   | Mail       | Node  | In-game mail system          |
| 9   | Activity   | Mesh  | Events & campaigns           |
| 10  | Shop       | Mesh  | Item store & purchasing      |
| 11  | Rank       | Mesh  | Leaderboards & scoring       |
| 12  | Guild      | Mesh  | Clan / guild management      |
| 13  | Bag        | Mesh  | Inventory & items            |
| 14  | Task       | Mesh  | Quests & daily tasks         |
| 15  | Notice     | Mesh  | Announcements & push         |
| 16  | Recharge   | Mesh  | Payment & billing            |

## Tech Stack

| Layer          | Technology                         |
|----------------|-------------------------------------|
| Backend        | Go (Due framework)                  |
| Frontend       | React                               |
| Database       | MySQL (primary), ClickHouse (analytics) |
| Cache          | Redis                               |
| Service Discovery | etcd                               |
| RPC            | gRPC / Protobuf                     |
| Message Queue  | NATS                                |

## Directory Structure

```
rockgame/
├── cmd/                    # Entry points for each component
│   ├── gate/               # Gate server main
│   ├── node/
│   │   ├── account/        # Account node main
│   │   └── game/           # Game node main
│   ├── mesh/
│   │   ├── activity/       # Activity mesh main
│   │   └── shop/           # Shop mesh main
│   └── migrate/            # Database migration tool
├── configs/                # Configuration files
├── internal/               # Private application code
│   ├── account/            # Account module logic
│   ├── game/               # Game module logic
│   ├── activity/           # Activity module logic
│   ├── shop/               # Shop module logic
│   └── pkg/                # Shared internal packages
├── pkg/                    # Public reusable packages
├── proto/                  # Protobuf definitions
│   ├── account/
│   ├── game/
│   ├── activity/
│   └── shop/
├── pb/                     # Generated protobuf Go code
├── api/                    # API definitions & handlers
├── scripts/                # Build, deploy, and utility scripts
├── docker/                 # Dockerfiles for each component
│   ├── Dockerfile.gate
│   ├── Dockerfile.node-account
│   ├── Dockerfile.node-game
│   ├── Dockerfile.mesh-activity
│   └── Dockerfile.mesh-shop
├── deployments/            # Kubernetes / compose manifests
├── docs/                   # Documentation
├── .gitignore
├── .env.example
├── Makefile
├── go.mod
└── go.sum
```

## Quick Start

```bash
# Prerequisites
go version 1.23+
docker & docker-compose
mysql, redis, etcd running

# Build all components
make build

# Run gate server
./bin/gate -conf configs/gate.yaml

# Run a specific node
./bin/node-account -conf configs/node-account.yaml
./bin/node-game -conf configs/node-game.yaml

# Run a specific mesh
./bin/mesh-activity -conf configs/mesh-activity.yaml
./bin/mesh-shop -conf configs/mesh-shop.yaml

# Run database migrations
make migrate
```

## Build Targets

```bash
make build           # Build all components
make gate            # Build gate server
make node-account    # Build account node
make node-game       # Build game node
make mesh-activity   # Build activity mesh
make mesh-shop       # Build shop mesh
make proto           # Generate protobuf code
make lint            # Run golangci-lint
make clean           # Clean build artifacts
make docker          # Build Docker images
make migrate         # Run database migrations
```

## License

MIT
