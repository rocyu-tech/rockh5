# RockGame Multi-Process Deployment Guide
#
# ========================================
# 核心原则
# ========================================
#
# 1. 无状态服务 (Gate / 4 Nodes):
#    - 多实例完全等价，任意实例可处理任意请求
#    - 扩容方式：直接增加实例，前面 Nginx/Kong 轮询
#    - 端口分配：Node 使用 basePort + nodeID，每个实例必须用不同 nodeID
#
# 2. 有状态 Mesh 服务 (10 个 Mesh):
#    - 同一用户的请求必须路由到同一实例（用户数据亲和）
#    - 扩容方式：通过 etcd 注册新节点，Gate Watch 感知后自动更新一致性哈希环
#    - 扩容时仅约 1/N 用户迁移，无需全量数据迁移
#    - 端口分配：同 Node，basePort + nodeID
#
# 3. 服务发现 (etcd):
#    - 所有 Node/Mesh 服务启动时向 etcd 注册（带 Lease TTL 10s 自动续约心跳）
#    - Gate 通过 etcd Watch 监听后端节点上下线，实时更新路由表
#    - 服务注册路径: /rockgame/services/{serviceName}/{nodeID} → {addr}:{port}
#    - Mesh 注册名带 -mesh 后缀 (如 activity-mesh) 用于 Gate 区分 Node/Mesh 路由策略
#
# ========================================
# 端口分配规则
# ========================================
#
# 服务              basePort    实例0    实例1    实例2    ...
# ────────────────────────────────────────────────────────
# Gate              8080        8080    8081    8082    (需Nginx映射)
# Account Node      8001        8001    8002    8003
# Game Node         8101        8101    8102    8103
# Lobby Node        8201        8201    8202    8203
# Event Node        8301        8301    8302    8303
# Activity Mesh     9001        9001    9002    9003
# Shop Mesh         9101        9101    9102    9103
# VIP Mesh          9201        9201    9202    9203
# Task Mesh         9301        9301    9302    9303
# Mail Mesh         9401        9401    9402    9403
# Rank Mesh         9501        9501    9502    9503
# Agent Mesh        9601        9601    9602    9603
# Item Mesh         9701        9701    9702    9703
# Tag Mesh          9801        9801    9802    9803
# RedDot Mesh       9901        9901    9902    9903
#
# ========================================
# 启动命令示例
# ========================================
#
# 单实例 (开发环境):
#   ./bin/gate -config etc/dev/config.yaml
#   ./bin/node-account -config etc/dev/config.yaml -node 0
#   ./bin/mesh-activity -config etc/dev/config.yaml -node 0
#
# 多实例 (生产环境):
#   # Gate 2 实例 (需要不同端口，修改 config 或用 Nginx)
#   ./bin/gate -config etc/prod/config.yaml    # :8080
#   ./bin/gate -config etc/prod/config.yaml    # 另一台机器或容器
#
#   # Node 3 实例
#   ./bin/node-account -config etc/prod/config.yaml -node 0  # :8001
#   ./bin/node-account -config etc/prod/config.yaml -node 1  # :8002
#   ./bin/node-account -config etc/prod/config.yaml -node 2  # :8003
#
#   # Mesh 2 实例 (一致性哈希自动路由)
#   ./bin/mesh-activity -config etc/prod/config.yaml -node 0  # :9001
#   ./bin/mesh-activity -config etc/prod/config.yaml -node 1  # :9002
#
# ========================================
# 配置文件说明
# ========================================
#
# 所有服务共享同一份 config.yaml，通过环境变量 ROCKGAME_APP_NAME 区分服务。
# 端口通过 -node 参数动态计算，不需要为每个实例维护独立配置文件。
#
# 环境变量覆盖优先级: 环境变量 > config.yaml > 默认值
#   ROCKGAME_APP_NAME  - 服务标识（用于日志和注册中心）
#   ROCKGAME_APP_ENV   - 环境标识
#   DB_PASSWORD        - 数据库密码
#   REDIS_PASSWORD     - Redis密码
#   ETCD_ENDPOINTS     - etcd地址 (默认 http://127.0.0.1:2379)
#   JWT_SECRET         - JWT密钥
