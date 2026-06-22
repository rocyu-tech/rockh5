# RockGame 环境搭建指南

本文档涵盖从零开始搭建 RockGame 完整编译环境、运行环境及 Docker 部署环境的所有步骤。

---

## 目录

- [1. 系统要求](#1-系统要求)
- [2. 编译环境搭建](#2-编译环境搭建)
  - [2.1 Go 语言环境](#21-go-语言环境)
  - [2.2 Protobuf 编译器](#22-protobuf-编译器)
  - [2.3 代码质量工具](#23-代码质量工具)
  - [2.4 前端环境（调试客户端）](#24-前端环境调试客户端)
- [3. 运行环境搭建](#3-运行环境搭建)
  - [3.1 MySQL](#31-mysql)
  - [3.2 Redis](#32-redis)
  - [3.3 etcd](#33-etcd)
  - [3.4 Nginx（生产环境负载均衡）](#34-nginx生产环境负载均衡)
- [4. Docker 部署环境](#4-docker-部署环境)
  - [4.1 Docker 安装](#41-docker-安装)
  - [4.2 Docker Compose 说明](#42-docker-compose-说明)
  - [4.3 一键启动全部服务](#43-一键启动全部服务)
  - [4.4 常用 Docker 运维命令](#44-常用-docker-运维命令)
- [5. 项目编译与启动](#5-项目编译与启动)
  - [5.1 克隆代码](#51-克隆代码)
  - [5.2 本地编译](#52-本地编译)
  - [5.3 数据库初始化](#53-数据库初始化)
  - [5.4 etcd 路由初始化](#54-etcd-路由初始化)
  - [5.5 本地开发模式启动](#55-本地开发模式启动)
  - [5.6 本地多实例启动（生产模拟）](#56-本地多实例启动生产模拟)
- [6. 环境变量说明](#6-环境变量说明)
- [7. 端口分配表](#7-端口分配表)
- [8. 常见问题排查](#8-常见问题排查)

---

## 1. 系统要求

| 项目 | 最低配置 | 推荐配置 |
|------|----------|----------|
| 操作系统 | Ubuntu 20.04 / CentOS 8 / macOS 12 | Ubuntu 22.04 LTS |
| CPU | 4 核 | 8 核 |
| 内存 | 8 GB | 16 GB |
| 磁盘 | 50 GB | 100 GB SSD |
| 网络 | 需访问 GitHub / Docker Hub | 海外 VPS 推荐 |

> 说明：RockGame 包含 15 个 Go 微服务 + MySQL + Redis + etcd，完整部署需要较多资源。开发环境可仅启动必要服务。

---

## 2. 编译环境搭建

### 2.1 Go 语言环境

RockGame 使用 Go 1.24，必须先安装 Go 编译器。

**Linux (Ubuntu/Debian):**

```bash
# 下载 Go 1.24
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz

# 解压到 /usr/local（需要 sudo）
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.4.linux-amd64.tar.gz

# 配置环境变量（追加到 ~/.bashrc 或 ~/.zshrc）
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
# 输出: go version go1.24.4 linux/amd64

# 配置代理（国内网络必需）
go env -w GOPROXY=https://goproxy.cn,direct
```

**macOS:**

```bash
# 使用 Homebrew 安装
brew install go@1.24
brew link --overwrite go@1.24

# 验证
go version
```

> 注意：如果系统上已有旧版本 Go，务必先卸载（`sudo apt remove golang*` 或 `brew remove go`），避免版本冲突。

### 2.2 Protobuf 编译器

RockGame 使用 gRPC/Protobuf 进行服务间通信，需要安装 protoc 及 Go 插件。

```bash
# 安装 protoc 编译器（Linux）
PB_VERSION="29.3"
wget https://github.com/protocolbuffers/protobuf/releases/download/v${PB_VERSION}/protoc-${PB_VERSION}-linux-x86_64.zip
unzip protoc-${PB_VERSION}-linux-x86_64.zip -d /usr/local
rm protoc-${PB_VERSION}-linux-x86_64.zip

# 验证
protoc --version
# 输出: libprotoc 29.3

# 安装 Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 验证插件安装（应输出安装路径）
which protoc-gen-go
which protoc-gen-go-grpc
```

### 2.3 代码质量工具

```bash
# 安装 golangci-lint（Go 代码静态检查）
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# 验证
golangci-lint --version
```

### 2.4 前端环境（调试客户端）

RockH5 是 React 调试客户端，仅开发调试时需要。

```bash
# 安装 Node.js 18+ (推荐使用 nvm 管理)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
source ~/.bashrc
nvm install 18
nvm use 18

# 验证
node --version
npm --version

# 克隆前端仓库并安装依赖
git clone https://github.com/rocyu-tech/rockh5.git
cd rockh5
npm install
```

---

## 3. 运行环境搭建

RockGame 依赖三个核心基础设施：MySQL、Redis、etcd。以下是本地安装方式（Docker 方式见第 4 节）。

### 3.1 MySQL

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install mysql-server -y

# 启动服务
sudo systemctl start mysql
sudo systemctl enable mysql

# 初始安全配置
sudo mysql_secure_installation

# 创建数据库和用户
sudo mysql -u root -p
```

```sql
CREATE DATABASE rockgame CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'rockgame'@'%' IDENTIFIED BY '${DB_PASSWORD}';
GRANT ALL PRIVILEGES ON rockgame.* TO 'rockgame'@'%';
FLUSH PRIVILEGES;
```

**修改 MySQL 配置**（`/etc/mysql/mysql.conf.d/mysqld.cnf`）：

```ini
[mysqld]
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
max_connections = 500
innodb_buffer_pool_size = 1G
```

```bash
sudo systemctl restart mysql
```

### 3.2 Redis

```bash
# Ubuntu/Debian
sudo apt install redis-server -y

# 修改配置（/etc/redis/redis.conf）
sudo sed -i 's/^# requirepass.*/requirepass ${REDIS_PASSWORD}/' /etc/redis/redis.conf
sudo sed -i 's/^maxmemory.*/maxmemory 512mb/' /etc/redis/redis.conf
sudo sed -i 's/^# maxmemory-policy.*/maxmemory-policy allkeys-lru/' /etc/redis/redis.conf

# 重启
sudo systemctl restart redis
sudo systemctl enable redis

# 验证
redis-cli -a ${REDIS_PASSWORD} ping
# 输出: PONG
```

### 3.3 etcd

**二进制安装（推荐，最简单）：**

```bash
# 下载 etcd（Linux amd64）
ETCD_VER="v3.5.17"
wget https://github.com/etcd-io/etcd/releases/download/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz
tar xzf etcd-${ETCD_VER}-linux-amd64.tar.gz
sudo mv etcd-${ETCD_VER}-linux-amd64/etcd /usr/local/bin/
sudo mv etcd-${ETCD_VER}-linux-amd64/etcdctl /usr/local/bin/
rm -rf etcd-${ETCD_VER}-linux-amd64*

# 验证
etcd --version
etcdctl version
```

**systemd 服务配置：**

```bash
sudo tee /etc/systemd/system/etcd.service > /dev/null << 'EOF'
[Unit]
Description=etcd service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/etcd \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:2379 \
  --listen-peer-urls http://127.0.0.1:2380
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl start etcd
sudo systemctl enable etcd

# 验证
etcdctl endpoint health
# 输出: 127.0.0.1:2379 is healthy
```

### 3.4 Nginx（生产环境负载均衡）

```bash
# Ubuntu/Debian
sudo apt install nginx -y

# RockGame 使用 Nginx 在 Gate 前做负载均衡
# 配置文件位于 deploy/docker/nginx.conf（Docker部署时自动挂载）
# 本地部署时复制到 Nginx 配置目录

sudo cp deploy/docker/nginx.conf /etc/nginx/conf.d/rockgame.conf

# 修改 upstream 地址为本地地址
# upstream rockgame_gate {
#     server 127.0.0.1:8080 weight=1;
#     server 127.0.0.1:8081 weight=1;
# }

sudo nginx -t
sudo systemctl restart nginx
```

---

## 4. Docker 部署环境

Docker 方式是最推荐的部署方式，一条命令启动全部基础设施和应用服务。

### 4.1 Docker 安装

**Linux (Ubuntu/Debian):**

```bash
# 卸载旧版本（如有）
sudo apt remove docker docker-engine docker.io containerd runc

# 安装依赖
sudo apt update
sudo apt install -y ca-certificates curl gnupg

# 添加 Docker 官方 GPG 密钥
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# 添加 Docker 仓库
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo $VERSION_CODENAME) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装 Docker Engine
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin

# 将当前用户加入 docker 组（免 sudo）
sudo usermod -aG docker $USER
newgrp docker

# 验证
docker --version
docker compose version
```

**macOS:**

```bash
# 下载安装 Docker Desktop
# https://www.docker.com/products/docker-desktop/
# 安装后 Docker Compose V2 插件自动包含
```

**CentOS/RHEL:**

```bash
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker $USER
```

### 4.2 Docker Compose 说明

> **重要提示：`docker-compose` vs `docker compose`**
>
> Docker Compose 有两个版本：
>
> | 版本 | 命令 | 说明 |
> |------|------|------|
> | V1 (已废弃) | `docker-compose` | 独立 Python 程序，需要单独安装，**已停止维护** |
> | V2 (当前) | `docker compose` | Docker 内置插件，安装 Docker Engine 时自动包含 |
>
> 如果你遇到 `docker-compose: command not found`，说明你使用的是新版 Docker，应使用 `docker compose`（中间是**空格**，不是**连字符**）。
>
> ```bash
> # 错误 (V1，已废弃)
> docker-compose up -d
>
> # 正确 (V2，当前版本)
> docker compose up -d
> ```
>
> 如果确实需要 V1 兼容（不推荐），可以手动创建软链接：
> ```bash
> DOCKER_CONFIG="${DOCKER_CONFIG:-$HOME/.docker}"
> mkdir -p $DOCKER_CONFIG/cli-plugins
> curl -SL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 \
>   -o $DOCKER_CONFIG/cli-plugins/docker-compose
> chmod +x $DOCKER_CONFIG/cli-plugins/docker-compose
> ```

### 4.3 一键启动全部服务

```bash
# 进入项目目录
cd rockgame/deploy/docker

# 一键启动所有服务（基础设施 + 15 个微服务 + Nginx）
docker compose up -d

# 查看服务状态
docker compose ps

# 查看日志（所有服务）
docker compose logs -f

# 查看某个服务日志
docker compose logs -f gate-0
docker compose logs -f etcd

# 停止所有服务
docker compose down

# 停止并清除数据卷（重新初始化数据库）
docker compose down -v
```

**仅启动基础设施（开发调试）：**

```bash
# 只启动 MySQL + Redis + etcd，不启动应用服务
docker compose up -d mysql redis etcd

# 然后用本地编译的二进制启动应用
./bin/gate -config etc/dev/config.yaml
```

**分步启动（推荐生产环境使用）：**

```bash
# 第一步：基础设施
docker compose up -d mysql redis etcd

# 等待基础设施就绪（healthcheck 通过）
docker compose ps mysql redis etcd

# 第二步：Node 层
docker compose up -d node-account-0 node-account-1 \
  node-game-0 node-game-1 \
  node-lobby-0 node-lobby-1 \
  node-event-0 node-event-1

# 第三步：Mesh 层
docker compose up -d mesh-activity-0 mesh-activity-1 \
  mesh-shop-0 mesh-shop-1 \
  mesh-vip-0 mesh-vip-1 \
  mesh-task-0 mesh-task-1 \
  mesh-mail-0 mesh-mail-1 \
  mesh-rank-0 mesh-rank-1 \
  mesh-agent-0 mesh-agent-1 \
  mesh-item-0 mesh-item-1 \
  mesh-tag-0 mesh-tag-1 \
  mesh-reddot-0 mesh-reddot-1

# 第四步：Gate + Nginx
docker compose up -d gate-0 gate-1 nginx
```

### 4.4 常用 Docker 运维命令

```bash
# ---- 服务管理 ----

# 重启单个服务
docker compose restart gate-0

# 重建镜像并重启（代码更新后）
docker compose up -d --build gate-0

# 扩容（新增实例，注意端口冲突需修改配置）
docker compose up -d --scale node-account-0=3

# ---- 故障排查 ----

# 查看容器日志（最近100行）
docker compose logs --tail=100 gate-0

# 进入容器终端
docker compose exec gate-0 sh

# 查看容器资源占用
docker stats

# 查看 etcd 注册的所有服务
docker compose exec etcd etcdctl get /rockgame/services/ --prefix

# ---- 数据管理 ----

# 备份 MySQL
docker compose exec mysql mysqldump -u root -proot123 rockgame > backup.sql

# 恢复 MySQL
docker compose exec -T mysql mysql -u root -proot123 rockgame < backup.sql

# 查看 Redis 状态
docker compose exec redis redis-cli -a "" info stats

# ---- 清理 ----

# 清理无用镜像
docker image prune -f

# 清理所有未使用资源
docker system prune -f
```

---

## 5. 项目编译与启动

### 5.1 克隆代码

```bash
git clone https://github.com/rocyu-tech/rockgame.git
cd rockgame
```

### 5.2 本地编译

```bash
# 拉取依赖
go mod download

# 编译全部服务
make build

# 或单独编译某个服务
make gate
make node-account
make node-game
make mesh-activity
make mesh-shop

# 生成的二进制文件在 bin/ 目录下
ls bin/
# gate  node-account  node-game  mesh-activity  mesh-shop  ...
```

> Makefile 默认只编译 5 个核心服务，完整编译 15 个服务需要手动补充 Makefile target，或直接用 go build：
>
> ```bash
> CGO_ENABLED=0 go build -o bin/node-lobby ./cmd/node/lobby
> CGO_ENABLED=0 go build -o bin/node-event ./cmd/node/event
> CGO_ENABLED=0 go build -o bin/mesh-vip ./cmd/mesh/vip
> CGO_ENABLED=0 go build -o bin/mesh-task ./cmd/mesh/task
> CGO_ENABLED=0 go build -o bin/mesh-mail ./cmd/mesh/mail
> CGO_ENABLED=0 go build -o bin/mesh-rank ./cmd/mesh/rank
> CGO_ENABLED=0 go build -o bin/mesh-agent ./cmd/mesh/agent
> CGO_ENABLED=0 go build -o bin/mesh-item ./cmd/mesh/item
> CGO_ENABLED=0 go build -o bin/mesh-tag ./cmd/mesh/tag
> CGO_ENABLED=0 go build -o bin/mesh-reddot ./cmd/mesh/reddot
> ```

### 5.3 数据库初始化

```bash
# 执行数据库迁移
make migrate

# 或直接运行
go run ./cmd/migrate
```

### 5.4 etcd 路由初始化

> **重要：首次部署必须执行此步骤！** Gate 通过 etcd 获取路由表来决定如何转发请求。如果 etcd 中没有路由数据，Gate 会降级到 YAML 静态路由（仅 dev 配置有效，prod 配置路由为空将导致所有请求 404）。

**路由表内容：** 定义了 URL 前缀与后端服务的映射关系，例如 `/api/v1/account` → `account-node`（需 JWT 认证）、`/api/v1/auth` → `account-node`（无需认证）。完整 16 条路由见 `cmd/bootstrap/main.go` 中的 `defaultRoutes`。

**etcd 数据结构：**

```
key:   /rockgame/routes/api/v1/account
value: {"prefix":"/api/v1/account","backend":"account-node","auth":true}
```

**使用 bootstrap 工具初始化（推荐）：**

```bash
# 查看当前 etcd 中的路由
make show-routes

# 初始化路由到 etcd（交互式确认）
make bootstrap-routes

# 强制写入，不弹确认提示（CI/CD 自动化部署用）
make bootstrap-routes-force

# 清空 etcd 中的所有路由
make clean-routes
```

**直接用 Go 命令：**

```bash
# 查看路由
go run ./cmd/bootstrap -config etc/dev/config.yaml -action show

# 写入路由
go run ./cmd/bootstrap -config etc/prod/config.yaml -action seed

# 强制写入（CI/CD）
go run ./cmd/bootstrap -config etc/prod/config.yaml -action seed -force

# 清空路由
go run ./cmd/bootstrap -config etc/dev/config.yaml -action clean
```

**用 etcdctl 手动查询（排查用）：**

```bash
# 查看所有路由
etcdctl --endpoints=http://127.0.0.1:2379 get /rockgame/routes/ --prefix

# 查看所有已注册的服务实例
etcdctl --endpoints=http://127.0.0.1:2379 get /rockgame/services/ --prefix

# Docker 环境中
docker exec rockgame-etcd etcdctl get /rockgame/routes/ --prefix
```

**写入后的验证：**

```bash
# 检查 etcd 路由数量
etcdctl get /rockgame/routes/ --prefix --keys-only
# 应输出 16 行 key

# 检查 Gate 是否加载（自动热加载，无需重启）
curl http://127.0.0.1:8080/health | jq .
# source 应为 "etcd"，routes 应为 16
```

> **注意：** 开发环境（`etc/dev/config.yaml`）中 `gate.routes` 有完整的 16 条静态路由作为 fallback，即使不初始化 etcd 也能工作。但生产环境（`etc/prod/config.yaml`）中 `gate.routes: []` 为空，**必须通过 bootstrap 初始化 etcd 路由**，否则所有 API 请求返回 404。

### 5.5 本地开发模式启动

开发环境只需启动必要的最小服务集合：

```bash
# 终端 1: 启动 Gate（接入层）
./bin/gate -config etc/dev/config.yaml

# 终端 2: 启动 Account Node（账号服务）
ROCKGAME_APP_NAME=rockgame-account-node ./bin/node-account -config etc/dev/config.yaml -node 0

# 终端 3: 启动 Game Node（游戏服务）
ROCKGAME_APP_NAME=rockgame-game-node ./bin/node-game -config etc/dev/config.yaml -node 0

# 终端 4: 启动 Activity Mesh（活动服务）
ROCKGAME_APP_NAME=rockgame-activity-mesh ./bin/mesh-activity -config etc/dev/config.yaml -node 0
```

### 5.6 本地多实例启动（生产模拟）

模拟多实例部署，验证负载均衡和服务发现：

```bash
# Gate 2 实例（不同端口: 8080, 8081）
./bin/gate -config etc/dev/config.yaml -node 0 -nodes 2 &
./bin/gate -config etc/dev/config.yaml -node 1 -nodes 2 &

# Account Node 3 实例（不同端口: 8001, 8002, 8003）
ROCKGAME_APP_NAME=rockgame-account-node ./bin/node-account -config etc/dev/config.yaml -node 0 &
ROCKGAME_APP_NAME=rockgame-account-node ./bin/node-account -config etc/dev/config.yaml -node 1 &
ROCKGAME_APP_NAME=rockgame-account-node ./bin/node-account -config etc/dev/config.yaml -node 2 &

# Mesh 实例（一致性哈希路由）
ROCKGAME_APP_NAME=rockgame-activity-mesh ./bin/mesh-activity -config etc/dev/config.yaml -node 0 &
ROCKGAME_APP_NAME=rockgame-activity-mesh ./bin/mesh-activity -config etc/dev/config.yaml -node 1 &
```

---

## 6. 环境变量说明

| 变量名 | 说明 | 默认值 | 示例 |
|--------|------|--------|------|
| `ROCKGAME_APP_NAME` | 服务标识（用于日志和 etcd 注册） | 从 config 读取 | `rockgame-gate` |
| `ROCKGAME_APP_ENV` | 环境标识 | `dev` | `prod` |
| `DB_PASSWORD` | 数据库密码 | `root123` | 从环境变量覆盖 config |
| `REDIS_PASSWORD` | Redis 密码 | 空 | 从环境变量覆盖 config |
| `ETCD_ENDPOINTS` | etcd 地址列表 | `127.0.0.1:2379` | `10.0.1.100:2379,10.0.1.101:2379` |
| `JWT_SECRET` | JWT 签名密钥 | `dev-jwt-secret-change-in-production` | 生产环境务必修改 |

> 环境变量覆盖优先级：环境变量 > config.yaml > 默认值

---

## 7. 端口分配表

| 服务 | basePort | 实例0 | 实例1 | 实例2 | 说明 |
|------|----------|-------|-------|-------|------|
| Nginx | 80 | 80 | - | - | 入口负载均衡 |
| Gate | 8080 | 8080 | 8081 | 8082 | 无状态，LB轮询 |
| Account Node | 8001 | 8001 | 8002 | 8003 | 无状态 |
| Game Node | 8101 | 8101 | 8102 | 8103 | 无状态 |
| Lobby Node | 8201 | 8201 | 8202 | 8203 | 无状态 |
| Event Node | 8301 | 8301 | 8302 | 8303 | 无状态 |
| Activity Mesh | 9001 | 9001 | 9002 | 9003 | 有状态，一致性哈希 |
| Shop Mesh | 9101 | 9101 | 9102 | 9103 | 有状态 |
| VIP Mesh | 9201 | 9201 | 9202 | 9203 | 有状态 |
| Task Mesh | 9301 | 9301 | 9302 | 9303 | 有状态 |
| Mail Mesh | 9401 | 9401 | 9402 | 9403 | 有状态 |
| Rank Mesh | 9501 | 9501 | 9502 | 9503 | 有状态 |
| Agent Mesh | 9601 | 9601 | 9602 | 9603 | 有状态 |
| Item Mesh | 9701 | 9701 | 9702 | 9703 | 有状态 |
| Tag Mesh | 9801 | 9801 | 9802 | 9803 | 有状态 |
| RedDot Mesh | 9901 | 9901 | 9902 | 9903 | 有状态 |

---

## 8. 常见问题排查

### Q: `docker-compose: command not found`

新版 Docker 使用 `docker compose`（空格），不是 `docker-compose`（连字符）。

```bash
# 检查 Docker Compose 版本
docker compose version

# 如果仍提示找不到，安装 Compose V2 插件
mkdir -p ~/.docker/cli-plugins
curl -SL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 \
  -o ~/.docker/cli-plugins/docker-compose
chmod +x ~/.docker/cli-plugins/docker-compose
```

### Q: `go: module xxx requires go >= 1.24`

```bash
# 检查当前版本
go version

# 升级 Go（参考 2.1 节）
```

### Q: `protoc-gen-go: program not found`

```bash
# 确认 GOPATH/bin 在 PATH 中
echo $PATH | grep GOPATH

# 如果没有，手动添加
export PATH=$PATH:$(go env GOPATH)/bin

# 重新安装插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Q: etcd 启动失败 `bind: address already in use`

```bash
# 检查端口占用
sudo lsof -i :2379
sudo lsof -i :2380

# 杀掉占用进程或修改 etcd 监听端口
```

### Q: etcd 中没有路由，请求全部返回 404

```bash
# 症状：curl /health 返回 source: "yaml" 或 routes: 0
# 原因：首次部署未初始化 etcd 路由

# 解决：执行 bootstrap
make bootstrap-routes

# 验证
curl http://127.0.0.1:8080/health | jq .data.source
# 应输出 "etcd"
```

### Q: Gate 健康检查显示 `source: "yaml"`

说明 etcd 中没有路由数据，Gate 自动降级到了 `etc/dev/config.yaml` 中的静态路由。开发环境正常，生产环境需要执行 `make bootstrap-routes`。

### Q: 如何确认路由写入成功

```bash
# 方法1：bootstrap 工具查看
make show-routes

# 方法2：etcdctl 直接查
etcdctl get /rockgame/routes/ --prefix --keys-only | wc -l
# 应输出 16

# 方法3：Gate /health 接口
curl -s http://127.0.0.1:8080/health | jq '{routes: .data.routes, source: .data.source}'
```

### Q: Docker 容器启动失败 `healthcheck failed`

```bash
# 查看容器日志
docker compose logs mysql
docker compose logs etcd
docker compose logs redis

# 常见原因：
# 1. MySQL 启动慢，等待其 healthcheck 通过即可
# 2. 端口冲突，检查是否有进程占用了 3306/6379/2379
# 3. 数据卷损坏，执行 docker compose down -v 清除后重建
```

### Q: Gate 启动后无法连接后端服务

```bash
# 1. 检查 etcd 是否有服务注册
etcdctl get /rockgame/services/ --prefix

# 2. 检查后端服务是否正常启动
docker compose ps | grep node

# 3. 检查 etcd 连接配置
# 确认 config.yaml 中 etcd.addrs 地址正确
```
