FROM golang:1.24-alpine AS builder

WORKDIR /app

# 依赖缓存层
COPY go.mod go.sum ./
RUN go mod download

# 编译所有服务（注入版本号）
COPY . .
RUN BUILD_TIME=$(date +%Y%m%d%H%M%S) && \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    VERSION_FLAGS="-X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/gate ./cmd/gate && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/node-account ./cmd/node/account && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/node-game ./cmd/node/game && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/node-lobby ./cmd/node/lobby && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/node-event ./cmd/node/event && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-activity ./cmd/mesh/activity && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-shop ./cmd/mesh/shop && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-vip ./cmd/mesh/vip && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-task ./cmd/mesh/task && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-mail ./cmd/mesh/mail && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-rank ./cmd/mesh/rank && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-agent ./cmd/mesh/agent && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-item ./cmd/mesh/item && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-tag ./cmd/mesh/tag && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/mesh-reddot ./cmd/mesh/reddot && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w ${VERSION_FLAGS}" -o bin/node-admin ./cmd/node/admin

# 运行阶段 - 精简镜像
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=UTC

WORKDIR /app
COPY --from=builder /app/bin ./bin
COPY --from=builder /app/etc ./etc

EXPOSE 8080 8001 8101 8201 8301 8501 9001 9101 9201 9301 9401 9501 9601 9701 9801 9901

CMD ["./bin/gate", "-config", "/app/etc/prod/config.yaml"]
