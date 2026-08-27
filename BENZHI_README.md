# TSDB 时序数据库存储引擎

一个用 Go 编写的单机时序数据库存储引擎，支持高频时序数据的写入、查询、
降采样聚合、分片管理、数据保留清理与崩溃恢复。

## 功能

- **写入**：单点写入与批量写入，写入前做完整校验，批量写入具备原子性。
- **存储**：内存表 + 压缩块（snappy）的分层存储，预写日志（WAL）保证崩溃可恢复。
- **查询**：按时间范围查询、按标签过滤查询、跨分片合并与去重。
- **聚合**：按固定时间窗口降采样，支持 avg / min / max / sum / count。
- **运维**：分片密封、块合并（compaction）、数据保留清理与运行状态统计。
- **HTTP 服务**：`cmd/tsdb-server` 提供完整的 HTTP API。

## 构建与运行

### 本地运行

```bash
go build ./...
go run ./cmd/tsdb-server -addr :8080 -data-dir ./data
```

服务启动后可通过以下端点操作：

- `POST /write`：写入单个数据点
- `POST /write-batch`：批量写入
- `GET /query`、`GET /query-all`、`GET /query-label`：查询
- `GET /downsample`、`GET /downsample-series`：降采样聚合
- `POST /retention`：执行保留清理
- `POST /compact`：合并块
- `GET /health`：健康检查

### Docker 构建

项目使用离线 vendored 依赖构建镜像，支持 `linux/amd64` 与 `linux/arm64`：

```bash
./build_benzhi_docker.sh
```

或直接：

```bash
docker build -f benzhi.Dockerfile -t tsdb-server .
docker run --rm -p 8080:8080 tsdb-server
```

启动后可访问 `http://localhost:8080/health` 进行健康检查。

## 目录结构

- `cmd/tsdb-server`：HTTP 服务入口
- `internal/store`：引擎核心与路由
- `internal/shard`：分片状态机与读写
- `internal/wal`：预写日志
- `internal/storage`：压缩块与校验
- `internal/memtable`：内存表
- `internal/index`：序列与标签索引
- `internal/downsample`：降采样聚合
- `internal/retention`：保留清理策略
