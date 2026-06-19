# RockGame Project Information

## Repositories

| Name | Repository | Description |
|------|-----------|-------------|
| rockgame (Backend) | `git@github.com:rocyu-tech/rockgame.git` | Gate-Node-Mesh 架构，15 个 Go 服务 |
| rockadmin (Admin Panel) | `git@github.com:rocyu-tech/rockadmin.git` | React 19 + antd v5 + TypeScript + Vite |
| rockh5 (H5 Player Client) | `git@github.com:rocyu-tech/rockh5.git` | H5 玩家端前端，接口测试用 |

## Server Addresses

| Service | Address | Notes |
|---------|---------|-------|
| Gate (API Gateway) | `http://47.108.78.147:8880` | 后端统一入口，所有 API 通过此网关转发 |
| RockAdmin (Admin Panel) | `http://47.108.78.147:8899` | 后台管理系统 |
| RockH5 (H5 Player) | `http://47.108.78.147:8890` | H5 玩家端前端/接口测试 |

## Tech Stack

- **Backend**: Go 1.24 + Fiber v2 + GORM + MySQL 8.0 + Redis 7 + etcd 3.5
- **Admin Frontend**: React 19 + antd v5 + TypeScript + Vite + Zustand
- **H5 Frontend**: React + TypeScript + Vite + axios
- **Auth**: JWT (HS256), admin TTL 480min, player TTL 15min, refresh 7 days

## Architecture

- Gate (port 8880) → Node services (account/admin/game/lobby/event) → MySQL/Redis
- Mesh services (activity/shop/vip/task/mail/rank/agent/item/tag/reddot) for stateful routing
- etcd: service discovery + dynamic route management
- Nginx: reverse proxy / load balancing
