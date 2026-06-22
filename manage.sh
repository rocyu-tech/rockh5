#!/bin/bash

# =============================================================================
#  RockGame 服务管理脚本
#  用法: ./manage.sh <命令> [服务名]
#  命令:
#    build <服务名|all>  - 编译服务
#    start <服务名|all>  - 启动服务
#    stop  <服务名|all>  - 停止服务
#    restart <服务名|all>- 重启服务
#    status              - 查看所有服务运行状态
#    logs  <服务名>      - 查看服务日志 (tail -f)
# =============================================================================

set -euo pipefail

# 项目根目录 (脚本所在目录)
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

# 配置文件路径
CONFIG_FILE="etc/dev/config.yaml"

# 二进制输出目录
BIN_DIR="$PROJECT_DIR/bin"

# 日志输出目录
LOG_DIR="$PROJECT_DIR/logs"

# 退出码
EXIT_SUCCESS=0
EXIT_ERR_NOT_FOUND=1
EXIT_ERR_BUILD=2
EXIT_ERR_START=3
EXIT_ERR_STOP=4
EXIT_ERR_ARGS=5

# 所有服务定义: 服务名 -> 编译源码路径
declare -A SERVICE_SRC
SERVICE_SRC[gate]="cmd/gate/main.go"
SERVICE_SRC[mesh-activity]="cmd/mesh/activity/main.go"
SERVICE_SRC[mesh-agent]="cmd/mesh/agent/main.go"
SERVICE_SRC[mesh-item]="cmd/mesh/item/main.go"
SERVICE_SRC[mesh-mail]="cmd/mesh/mail/main.go"
SERVICE_SRC[mesh-rank]="cmd/mesh/rank/main.go"
SERVICE_SRC[mesh-reddot]="cmd/mesh/reddot/main.go"
SERVICE_SRC[mesh-shop]="cmd/mesh/shop/main.go"
SERVICE_SRC[mesh-tag]="cmd/mesh/tag/main.go"
SERVICE_SRC[mesh-task]="cmd/mesh/task/main.go"
SERVICE_SRC[mesh-vip]="cmd/mesh/vip/main.go"
SERVICE_SRC[node-account]="cmd/node/account/main.go"
SERVICE_SRC[node-admin]="cmd/node/admin/main.go"
SERVICE_SRC[node-event]="cmd/node/event/main.go"
SERVICE_SRC[node-game]="cmd/node/game/main.go"
SERVICE_SRC[node-lobby]="cmd/node/lobby/main.go"

# 有序服务名列表 (启动顺序: gate → node → mesh)
SERVICE_ORDER=(
    gate
    node-account node-lobby node-game node-event node-admin
    mesh-activity mesh-agent mesh-shop mesh-vip mesh-task
    mesh-mail mesh-rank mesh-item mesh-tag mesh-reddot
)

# =============================================================================
#  辅助函数
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERR]${NC} $*"; }
log_step()  { echo -e "${CYAN}----${NC} $*"; }

# 校验服务名是否合法
validate_service() {
    local name=$1
    if [[ -z "${SERVICE_SRC[$name]+x}" ]]; then
        log_error "未知服务: '$name'"
        echo ""
        echo "可选服务:"
        for s in "${SERVICE_ORDER[@]}"; do
            echo "  - $s"
        done
        return $EXIT_ERR_NOT_FOUND
    fi
    return 0
}

# 查找服务 PID (精确匹配二进制名 + 配置文件参数)
find_pid() {
    local name=$1
    local bin_name=$(basename "$name")
    # pgrep -f 匹配，过滤掉 grep 自身
    pgrep -f "$bin_name.*$CONFIG_FILE" 2>/dev/null || true
}

# =============================================================================
#  Build
# =============================================================================

build_service() {
    local name=$1
    local src="${SERVICE_SRC[$name]}"
    local bin_path="$BIN_DIR/$name"

    log_step "编译 $name (${src}) → ${bin_path}"
    mkdir -p "$BIN_DIR"

    if go build -o "$bin_path" "$src"; then
        log_info "$name 编译成功: $bin_path"
        return 0
    else
        log_error "$name 编译失败"
        return $EXIT_ERR_BUILD
    fi
}

cmd_build() {
    local target=${1:-all}
    if [[ "$target" == "all" ]]; then
        local failed=0
        for svc in "${SERVICE_ORDER[@]}"; do
            build_service "$svc" || failed=$((failed + 1))
        done
        if [[ $failed -gt 0 ]]; then
            log_error "$failed 个服务编译失败"
            return $EXIT_ERR_BUILD
        fi
    else
        validate_service "$target" || return $?
        build_service "$target"
    fi
}

# =============================================================================
#  Stop
# =============================================================================

stop_service() {
    local name=$1
    local pids=$(find_pid "$name")

    if [[ -z "$pids" ]]; then
        echo "    $name 未在运行"
        return 0
    fi

    echo "    停止 $name (PID: $pids) ..."
    for pid in $pids; do
        kill "$pid" 2>/dev/null || true
    done

    # 等待优雅退出 (最多 5 秒)
    local waited=0
    while [[ $waited -lt 5 ]]; do
        local remaining=$(find_pid "$name")
        [[ -z "$remaining" ]] && break
        sleep 1
        waited=$((waited + 1))
    done

    # 强杀残留进程
    local remaining=$(find_pid "$name")
    if [[ -n "$remaining" ]]; then
        for pid in $remaining; do
            kill -9 "$pid" 2>/dev/null || true
        done
        log_warn "$name 强制终止 (PID: $remaining)"
    else
        log_info "$name 已停止"
    fi
}

cmd_stop() {
    local target=${1:-all}
    if [[ "$target" == "all" ]]; then
        log_step "停止所有服务 (逆序)"
        # 逆序停止: mesh → node → gate
        local reversed=()
        for ((i=${#SERVICE_ORDER[@]}-1; i>=0; i--)); do
            reversed+=("${SERVICE_ORDER[$i]}")
        done
        for svc in "${reversed[@]}"; do
            stop_service "$svc"
        done
    else
        validate_service "$target" || return $?
        stop_service "$target"
    fi
}

# =============================================================================
#  Start
# =============================================================================

start_service() {
    local name=$1
    local bin_path="$BIN_DIR/$name"

    # 检查二进制是否存在
    if [[ ! -x "$bin_path" ]]; then
        log_error "$name 二进制不存在或不可执行: $bin_path"
        log_error "请先执行: $0 build $name"
        return $EXIT_ERR_START
    fi

    # 检查是否已在运行
    local existing=$(find_pid "$name")
    if [[ -n "$existing" ]]; then
        log_warn "$name 已在运行 (PID: $existing)，跳过启动"
        return 0
    fi

    # 确保日志目录存在
    mkdir -p "$LOG_DIR"

    # 启动 (日志输出到按天轮转文件，由应用层 logger 管理)
    nohup "$bin_path" -config "$CONFIG_FILE" >> "$LOG_DIR/${name}.stdout.log" 2>&1 &
    local pid=$!

    # 等待进程启动并检查
    sleep 0.5
    if kill -0 "$pid" 2>/dev/null; then
        log_info "$name 已启动 (PID: $pid)"
        return 0
    else
        log_error "$name 启动失败，请检查 $LOG_DIR/${name}.stdout.log"
        return $EXIT_ERR_START
    fi
}

cmd_start() {
    local target=${1:-all}
    if [[ "$target" == "all" ]]; then
        log_step "启动所有服务"
        for svc in "${SERVICE_ORDER[@]}"; do
            start_service "$svc"
        done
    else
        validate_service "$target" || return $?
        start_service "$target"
    fi
}

# =============================================================================
#  Restart
# =============================================================================

cmd_restart() {
    local target=${1:-all}
    if [[ "$target" == "all" ]]; then
        cmd_stop "all"
        echo ""
        cmd_start "all"
    else
        validate_service "$target" || return $?
        stop_service "$target"
        start_service "$target"
    fi
}

# =============================================================================
#  Status
# =============================================================================

cmd_status() {
    echo "======================================================================="
    printf "  %-20s %-8s %s\n" "服务名" "状态" "PID"
    echo "======================================================================="
    for svc in "${SERVICE_ORDER[@]}"; do
        local pid=$(find_pid "$svc")
        local bin_path="$BIN_DIR/$svc"
        local bin_exists="✗"
        [[ -x "$bin_path" ]] && bin_exists="✓"

        if [[ -n "$pid" ]]; then
            printf "  %-20s ${GREEN}%-8s${NC} %s  [bin:%s]\n" "$svc" "RUNNING" "$pid" "$bin_exists"
        else
            printf "  %-20s ${RED}%-8s${NC} %-12s [bin:%s]\n" "$svc" "STOPPED" "-" "$bin_exists"
        fi
    done
    echo "======================================================================="
}

# =============================================================================
#  Logs
# =============================================================================

cmd_logs() {
    local name=$1
    validate_service "$name" || return $?

    # 将 manage.sh 服务名映射到应用层服务名
    # manage.sh: node-lobby -> 应用: lobby-node
    # manage.sh: mesh-activity -> 应用: activity
    local svc_name=""
    case "$name" in
        node-*)    svc_name="${name#node-}-node" ;;   # node-lobby -> lobby-node
        mesh-*)    svc_name="${name#mesh-}" ;;         # mesh-activity -> activity
        gate)      svc_name="gate" ;;
        *)         svc_name="$name" ;;
    esac

    local today=$(date +%Y-%m-%d)
    local latest_log=""

    # 优先查找今天的日志: {LOG_DIR}/{date}/{serviceName}.{nodeID}.log
    # 先找 node=0 (默认单实例)，再找任意 node id
    if [[ -f "$LOG_DIR/$today/${svc_name}.0.log" ]]; then
        latest_log="$LOG_DIR/$today/${svc_name}.0.log"
    else
        # 查找该服务名的任意 node 日志
        latest_log=$(ls -t "$LOG_DIR/$today/${svc_name}".*.log 2>/dev/null | head -1)
    fi

    # 降级: 查找最近日期的日志
    if [[ -z "$latest_log" ]]; then
        latest_log=$(ls -t "$LOG_DIR"/*/"${svc_name}".*.log 2>/dev/null | head -1)
    fi

    if [[ -n "$latest_log" ]]; then
        echo -e "跟踪日志: ${CYAN}$latest_log${NC} (Ctrl+C 退出)"
        tail -f "$latest_log"
    else
        # 最终降级: 查看 stdout 日志
        local stdout_log="$LOG_DIR/${name}.stdout.log"
        if [[ -f "$stdout_log" ]]; then
            echo -e "跟踪 stdout: ${CYAN}$stdout_log${NC} (Ctrl+C 退出)"
            tail -f "$stdout_log"
        else
            log_error "未找到日志文件 ($LOG_DIR/*/${svc_name}.*.log 或 $stdout_log)"
        fi
    fi
}

# =============================================================================
#  入口
# =============================================================================

if [[ $# -eq 0 ]]; then
    echo ""
    echo "RockGame 服务管理脚本"
    echo ""
    echo "用法: $0 <命令> [服务名|all]"
    echo ""
    echo "命令:"
    echo "  build   <服务名|all>   编译服务 (默认 all)"
    echo "  start   <服务名|all>   启动服务 (默认 all)"
    echo "  stop    <服务名|all>   停止服务 (默认 all)"
    echo "  restart <服务名|all>   重启服务 (默认 all)"
    echo "  status                 查看所有服务运行状态"
    echo "  logs    <服务名>        实时跟踪服务日志"
    echo ""
    echo "示例:"
    echo "  $0 build all           # 编译所有服务"
    echo "  $0 restart mesh-agent  # 重启单个服务"
    echo "  $0 status              # 查看状态"
    echo "  $0 logs gate           # 查看 gate 日志"
    echo ""
    echo "服务列表:"
    for s in "${SERVICE_ORDER[@]}"; do
        echo "  - $s"
    done
    exit $EXIT_ERR_ARGS
fi

COMMAND=$1
shift

case "$COMMAND" in
    build)   cmd_build "$@" ;;
    start)   cmd_start "$@" ;;
    stop)    cmd_stop "$@" ;;
    restart) cmd_restart "$@" ;;
    status)  cmd_status ;;
    logs)    cmd_logs "$@" ;;
    *)
        log_error "未知命令: '$COMMAND'"
        echo "运行 '$0' 查看使用说明"
        exit $EXIT_ERR_ARGS
        ;;
esac
