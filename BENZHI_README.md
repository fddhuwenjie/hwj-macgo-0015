# 本质评测环境说明

## 项目

- 项目编号：`hwj-macgo-0015`
- 项目名称：授权证据生命周期核心
- 项目说明：管理主体、资源范围、用途条件、复核轮次、决策凭据、委托链与到期事件的本地可恢复技术证据服务。

## 固定环境

- Go toolchain：`go1.26.5`
- go.mod language version：`go 1.21`
- GOTOOLCHAIN：`local`
- 支持平台：`linux/amd64`、`linux/arm64`
- Docker 基础镜像：`golang:1.26.5-bookworm`
- Docker manifest：`golang@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd`

## 构建

```bash
./build_benzhi_docker.sh hwj-macgo-0015:benzhi-amd64 linux/amd64
./build_benzhi_docker.sh hwj-macgo-0015:benzhi-arm64 linux/arm64
```

## 运行

```bash
docker run --rm -it --network none hwj-macgo-0015:benzhi-amd64 bash
```

## 容器内验证

```bash
go version
go env GOTOOLCHAIN GOPROXY GOMODCACHE GOCACHE
go test ./...
go vet ./...
go build ./...
```

---

# 项目 README 同步内容

# 授权证据生命周期核心

本项目实现一个授权证据生命周期管理核心，围绕主体、资源范围、用途条件、授权申请、复核轮次、条件版本、决策凭据、临时暂停、委托链和到期事件等核心领域对象，提供申请、复核、启用、暂停、恢复、到期与撤回等完整状态流转。系统采用本地文件持久化，包含写前日志、原子快照、恢复机制、后台到期任务与派生查询，支持幂等决策、乐观并发、时区截止、审计链与风险排序。

## 快速开始

```bash
go build ./...
go test ./...
go run ./cmd/evidence --self-check
```

## 目录结构

- `cmd/evidence`：可执行入口
- `internal/domain`：领域对象、状态机、错误与仓库接口
- `internal/application`：应用服务与事务协调
- `internal/repository`：文件系统持久化实现
- `internal/journal`：写前日志
- `internal/recovery`：快照与恢复
- `internal/scheduler`：后台到期任务
- `internal/query`：派生查询与风险排序
- `internal/audit`：审计链

## 设计要点

- 授权范围只允许收窄，不能由复核步骤静默扩大。
- 条件版本随决策冻结，暂停期间迟到操作不改变有效性。
- 委托链必须无环且不得扩大上游范围。
- 所有持久化写入带版本、长度与校验，支持安全尾部截断、快照轮换与崩溃恢复。
- 所有公开状态修改均为确定性事务，使用互斥锁与乐观版本号保护共享状态。
