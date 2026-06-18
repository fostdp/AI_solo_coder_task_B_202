# 古代编钟调音磨锉声学仿真与音高修正系统

> Bianzhong (古代编钟声学仿真系统：传感器数据采集、有限元磨锉仿真、增广拉格朗日音高优化、告警推送、三维振型可视化
> 全套工程化部署：Docker多阶段构建、docker-compose 一键编排、Prometheus + Grafana 监控、TimescaleDB 降采样与保留策略、Gzip/Brotli 前端压缩、pprof 性能剖析、可配置 Python 模拟器

---

## 目录

- [架构总览](#架构总览)
  - [系统架构图](#系统架构图)
  - [模块说明](#模块说明)
- [部署步骤](#部署步骤)
  - [环境要求](#环境要求)
  - [快速启动](#快速启动)
  - [配置环境变量](#配置环境变量)
  - [常见问题](#常见问题)
- [模拟器用法](#模拟器用法)
  - [快速试用](#快速试用)
  - [音高目标](#音高目标)
  - [磨锉位置](#磨锉位置)
  - [全部环境变量](#全部环境变量)
- [可观测性](#可观测性)
  - [Prometheus 指标](#prometheus-指标)
  - [pprof 性能剖析](#pprof-性能剖析)
  - [Grafana 仪表板](#grafana-仪表板)
  - [TimescaleDB 降采样策略](#timescaledb-降采样策略)
- [API 文档](#api-文档)
- [目录结构](#目录结构)

---

## 架构总览

### 系统架构图
```
                              ┌─────────────────────────────────────────────────────────────┐
                              │                   前端 (Browser)                        │
                              │  ┌─────────────────────────────────────────┐      │
                              │  │  index.html  |  Three.js 模态振型动画     │      │
                              │  │  Chart.js  音高/偏差/磨锉趋势       │      │
                              │  │  WebSocket 实时告警推送            │      │
                              │  └──────────────┬──────────────────────┘      │
                              └─────────────────┼─────────────────────────────────────┘
                                                │  HTTP(S) + WebSocket
┌───────────────────────────────┐                    ┌──▼──────────────────────────────────────────────┐
│  nginx (可选 profile)│── 静态资源缓存     │  Go Backend (bianzhong-server)        │
│ gzip/brotli/缓存   │◄──────────────►  │  ┌───────────────────────────────┐ │
└───────────────────────┘  反代 API/WSS        │  │  • handlers   HTTP/handlers.go  │ │
                                         │  │    (REST + WebSocket 广播        │ │
                                         │  ├───────────────────────────────┤ │
                                         │  │  • metrics    Prometheus + pprof│ │
                                         │  │  • middleware Gzip          │ │
                                         │  │  • config    JSON 配置加载   │ │
                                         │  └──────────────┬────────────────┘ │
                                         ┌───────────┬───────────┬─────────┤
         ├───────────┤           │           │         │
         ▼           ▼            ▼          │         │
  ┌─────────┐  ┌──────────┐  ┌──────────┐ │         │
  │simulation│  │simulation│  │  mqtt   │ │         │
  │  FEM 3D  │  │gradient  │  │ alert.go│ │         │
  │ 磨锉仿真  │  │ 下降优化  │  │MQTT推送 │ │         │
  └────┬─────┘  └────┬─────┘  └────┬─────┘ │         │
       │              │              │       │         │
       └──────────────┼──────────────┘       │         │
                      │                      │         │
         ┌────────────┴────────────┐         │         │
         ▼            ▼            ▼            ▼         ▼
  ┌────────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐
  │TimescaleDB │ │  MQTT    │ │Prometheus│ │ Grafana   │ │bell-simulator│
  │  PostgreSQL│ │Mosquitto │ │ /metrics │ │ dashboard│ │  Python     │
  │  保留策略 │ │ Topic:   │ │ 采集指标  │ │ 可视化 │ │  虚拟编钟    │
  │  连续聚合 │ │bianzhong/│ │          │ │         │ │  配置音高/    │
  │  压缩     │ │  alerts  │ │          │ │         │ │  磨锉位置    │
  └─────┬─────┘ └────┬─────┘ └──────────┘ └────┬────┘ └──────┬───────┘
        │             │                       │              │
        └─────────────┴───────────────────────┴──────────────┘
                      Docker Compose 网络: bianzhong-net (172.28.0.0/16)
```

### 模块说明

| 模块 | 语言/镜像 | 说明 |
|------|----------|------|
| **bianzhong-server** | Go 1.22 | 业务后端。静态编译，CGO=0。REST API + WebSocket 广播 + FEM + 梯度下降 |
| **timescaledb** | timescale/timescaledb:2.14.2-pg16 | PostgreSQL 16 + TimescaleDB。自动建表、连续聚合降采样、压缩、保留策略 |
| **mqtt-broker** | eclipse-mosquitto:2.0.18 | 告警推送通道。1883 TCP + 9001 WebSocket |
| **prometheus** | prom/prometheus:v2.49.1 | 指标采集（HTTP 请求、仿真耗时、告警、网格重建等） |
| **grafana** | grafana/grafana:10.2.3 | 监控与业务仪表板。预配 TimescaleDB + Prometheus 数据源 |
| **bell-simulator** | Python 3.11 | 可配置音高目标、磨锉位置、磨锉深度的虚拟编钟。闭环：测量→告警→磨锉→请求后端优化建议 |
| **nginx** (可选) | nginx:1.25-alpine | 静态资源 gzip/brotli + 反代 API |

---

## 部署步骤

### 环境要求

| 依赖 | 最小版本 | 说明 |
|------|---------|------|
| Docker | ≥ 20.10+ | Compose v2（`docker compose` 子命令） |
| Docker Compose | v2 | 已集成在 Docker Desktop / Docker CE |
| CPU | 2 核 | 4 核推荐 |
| 内存 | 4 GB | 8 GB 推荐（TimescaleDB 256MB shared_buffers + Prometheus + Grafana） |
| 磁盘 | 10 GB | TimescaleDB 数据卷随时间增长 |

### 快速启动

```bash
# 1. 克隆并进入项目根目录
cd AI_solo_coder_task_A_202

# 2. （可选）复制环境变量模板并修改默认密码
cp .env.example .env
#    # 编辑 .env 中的 DB_PASSWORD、GRAFANA_PASSWORD

# 3. 构建并启动核心服务（5 个容器）
docker compose up -d --build

# 4. 等待健康检查全部通过（首次启动需 30-90 秒）
docker compose ps
docker compose logs -f bianzhong-server

# 5. 访问验证
curl http://localhost:8080/api/healthz
# -> {"status":"ok","version":"...","build_time":"..."}

# 6. 打开 Web UI
#    浏览器访问:  http://localhost:8080/
#    Prometheus:   http://localhost:9090/
#    Grafana:    http://localhost:3000/  (admin / admin123)
#    pprof:      http://localhost:6060/debug/pprof/

# 7. （可选）启动编钟模拟器
#    自动对音高目标 E4，3 个磨锉位置 8 角 × 3 层
docker compose --profile simulator up -d bell-simulator
docker compose logs -f bell-simulator

# 8. （可选）启动 nginx（前端代理
docker compose --profile nginx up -d nginx-proxy
#    访问: http://localhost/

# 9. 停止与销毁
docker compose down
docker compose down -v      # 保留数据卷不要用
```

### 配置环境变量

所有变量见根目录 `.env`（复制自 `.env.example`）：

| 变量 | 默认值 | 说明 |
|--------|---------|------|
| `SERVER_PORT | 8080 | Go 后端对外端口 |
| `DB_PORT` | 5432 | TimescaleDB 端口 |
| `DB_PASSWORD | bianzhong_secure_pw_CHANGEME | 数据库密码（生产必改 |
| `MQTT_PORT` | 1883 | MQTT TCP |
| `MQTT_WS_PORT` | 9001 | MQTT WebSocket |
| `PROM_PORT` | 9090 | Prometheus |
| `GRAFANA_PORT` | 3000 | Grafana |
| `GRAFANA_PASSWORD` | admin123 | Grafana 管理员密码（生产必改 |
| `PPROF_PORT` | 6060 | pprof 端口（生产环境建议绑定到 127.0.0.1 或通过防火墙限制 |
| `NGINX_PORT` | 80 | nginx （需 profile=nginx）
| **模拟器相关** | — | 见下节 [模拟器用法](#模拟器用法) |

### 常见问题

**Q: 首次启动后端报 `connection refused:5432`？**
A: TimescaleDB 初始化需要时间，`depends_on: condition: service_healthy` 已经包含健康检查，但首次如果仍超时，用 `docker compose restart bianzhong-server`。

**Q: MQTT 连不上？**
A: 检查 `mosquitto.conf` 已设 `allow_anonymous true`（测试用；生产环境加密码文件）。用客户端测试：
```bash
docker run --rm --network bianzhong-net eclipse-mosquitto:2.0.18 mosquitto_pub -h mqtt-broker -t "bianzhong/alerts/test" -m 'hello'
```

**Q: 模拟器没反应？**
A: `bell-simulator` 是 `restart: "no"` profile=simulator` 启动后才会创建。检查 `docker compose ps` 看容器是否存在。

---

## 模拟器用法

`bell-simulator` 是一个高保真虚拟编钟模拟器，用经验公式模拟真实的**厚度-频率关系、磨锉位置效率、测量噪声，通过 HTTP API 与后端闭环交互。

### 快速试用

```bash
# 默认: 目标 E4 (329.63 Hz，8 角度 × 3 高度磨锉
docker compose --profile simulator up -d

# 自定义: 目标中央 C (C4 = 261.63 Hz)，4 个磨锉角
SIM_TARGET_NOTE=C4 \
SIM_GRIND_POSITIONS="0,90,180,270" \
SIM_GRIND_HEIGHTS="0.5" \
docker compose --profile simulator up -d bell-simulator

# 只发测量不磨锉
SIM_GRIND_ENABLED=0 SIM_SESSION_SECONDS=300 \
docker compose --profile simulator up --force-recreate bell-simulator
```

### 音高目标

两种方式指定目标频率（**TARGET_NOTE 优先）：

```bash
# 方式一: 音名 (C0 ~ B8)
SIM_TARGET_NOTE=G4
# 识别 C C# Db D D# Eb E F F# Gb G G# Ab A A# Bb B

# 方式二: 频率 (Hz)
SIM_TARGET_FREQ=261.63

# 容差窗口
SIM_TOLERANCE=10   # ±10 cents 内认为已调音完成
SIM_INITIAL_OFFSET=30   # 起始偏调 +30 cents（模拟未打磨毛培铸造偏差

# 测量噪声
SIM_NOISE=2   # ±2 cents 高斯噪声
SIM_MEASURE_INTERVAL=5   # 5 秒/次
```

### 磨锉位置

磨锉位置用**角度**（0~359° 逗号分隔）× **高度层**（0=底部，1=顶部，归一化）：

```bash
# 8 个均布角度 × 3 个高度
SIM_GRIND_POSITIONS="0,45,90,135,180,225,270,315"
SIM_GRIND_HEIGHTS="0.3,0.5,0.7"

# 只在 "钟声发音点" 4 个主音区磨锉
SIM_GRIND_POSITIONS="0,90,180,270"
SIM_GRIND_HEIGHTS="0.4,0.6"

# 单次磨锉深度
SIM_GRIND_DEPTH=0.15      # 平均 0.15 mm
SIM_GRIND_RANDOM=0.3        # ±30% 随机抖动

# 多久磨一次
SIM_GRIND_INTERVAL=3           # 每 3 次测量磨一次

# 后端推荐磨锉计划
SIM_AUTO_TUNE=1            # 向 /api/bells/{id}/correction 请求优化建议
```

**模拟器内部的磨锉效率按位置计算：
- **轴向效率**：中部 50% 处最高（距上下两端越低）
- **周向效率**：sin² 形，4 个波峰/波谷交替
- **总效率** = 轴向 × 周向 × 0.95（迟滞系数）

### 全部环境变量

见 `simulator/bell_simulator.py` 文件头。

---

## 可观测性

### Prometheus 指标

访问 `http://localhost:9090/graph` 可查询以下指标：

| 指标名 | 类型 | 标签 | 含义 |
|--------|------|------|------|
| `bianzhong_http_requests_total` | Counter | method, route, status_code | HTTP 请求计数 |
| `bianzhong_http_request_duration_seconds` | Histogram | method, route | HTTP 耗时分布 |
| `bianzhong_measurements_received_total` | Counter | — | 声学测量接收数 |
| `bianzhong_pitch_deviation_alerts_total` | Counter | severity | 音高偏差告警数 |
| `bianzhong_grinding_operations_total` | Counter | — | 磨锉操作数 |
| `bianzhong_simulation_duration_seconds` | Histogram | — | FEM 仿真耗时 |
| `bianzhong_correction_iterations` | Histogram | status | 优化迭代次数 |
| `bianzhong_grid_rebuilds_total` | Counter | reason, success | 网格重建计数 |
| `bianzhong_websocket_clients_active` | Gauge | — | WebSocket 在线客户端数 |
| `bianzhong_mqtt_deliveries_total` | Counter | status | MQTT 推送结果 |

示例 PromQL:
```promql
# HTTP 95 分位响应时间
histogram_quantile(0.95, sum(rate(bianzhong_http_request_duration_seconds_bucket[5m]))

# 每秒测量数
rate(bianzhong_measurements_received_total[1m])

# 仿真耗时中位数
histogram_quantile(0.5, sum(rate(bianzhong_simulation_duration_seconds_bucket[5m]))
```

### pprof 性能剖析

生产环境可用 `PPROF_ADDR` 环境变量控制，默认 `:6060`。

```bash
# CPU 剖析 30 秒
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 内存堆
go tool pprof http://localhost:6060/debug/pprof/heap

# 火焰图 (需安装 graphviz)
go tool pprof -http=:8081 http://localhost:6060/debug/pprof/profile

# goroutine 泄漏
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Grafana 仪表板

`http://localhost:3000` 登录：admin / admin123

Grafana 已自动连接两个数据源：
- **Prometheus** (默认)：Go 服务实时指标
- **TimescaleDB**：历史时序查询

手工在仪表板 JSON 放入 `monitoring/grafana/dashboards/` 下可自动加载。

### TimescaleDB 降采样策略

`sql/timescale_policies.sql` 在 DB 初始化时自动执行，配置 4 级策略：

**保留策略 (drop_chunks)：**
| 表 | 保留期 |
|-----|--------|
| acoustic_measurements | 90 天 |
| grinding_operations | 365 天 |
| alert_events | 180 天 |
| acoustic_measurements_1m (1 分钟聚合) | 14 天 |
| acoustic_measurements_1h (1 小时聚合) | 1 年 |
| grinding_daily_summary | 3 年 |
| alert_hourly_summary | 1 年 |

**连续聚合 (Continuous Aggregates)：**
- `acoustic_measurements_1m` — 按 bell_id × mode_order，每分钟重新聚合一次
- `acoustic_measurements_1h` — 从 1m 视图上卷
- `grinding_daily_summary` — 每日操作汇总
- `alert_hourly_summary` — 每小时告警分布

**压缩策略：** 7 天前的声学测量、30 天前的磨锉/告警启用列存压缩，segmentby=bell_id,mode_order 压缩率 8~15 倍。

**自动刷新：**
```sql
-- 手工查询历史 1 小时聚合示例：
SELECT bucket, avg_frequency, avg_deviation
FROM acoustic_measurements_1h
WHERE bell_id = 'bell-01' AND bucket > now() - interval '7 days';
```

---

## API 文档

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/healthz` | 健康检查 |
| GET | `/api/version` | 版本信息 |
| GET | `/api/bells` | 编钟列表 |
| GET | `/api/bells/{id}` | 单个编钟详情 |
| POST | `/api/measurements` | 上报声学测量 |
| GET | `/api/bells/{id}/measurements` | 查询历史测量 |
| POST | `/api/grinding` | 上报磨锉操作 |
| GET | `/api/bells/{id}/grinding` | 查询磨锉历史 |
| POST | `/api/bells/{id}/simulation` | 运行 FEM 仿真 |
| GET | `/api/bells/{id}/correction` | 获取音高修正建议 |
| GET | `/api/alerts` | 查询告警事件 |
| GET | `/api/dashboard/stats` | 仪表盘统计 |
| GET | `/api/ws` | WebSocket 实时推送 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/debug/pprof/*` | pprof 调试（通过 pprof 端口访问） |

---

## 目录结构

```
AI_solo_coder_task_A_202/
├── backend/
│   ├── Dockerfile                 # Go 多阶段构建
│   ├── main.go                  # 服务入口（集成 pprof + metrics + gzip）
│   ├── go.mod / go.sum
│   ├── config/
│   │   ├── acoustic_params.json  # 声学参数配置
│   │   ├── constraint_params.json# 优化约束配置
│   │   └── loader.go            # JSON 配置加载
│   ├── database/                 # TimescaleDB 连接层
│   ├── handlers/                 # HTTP + WebSocket handlers
│   ├── metrics/                  # Prometheus 指标 + pprof 中间件
│   ├── middleware/               # Gzip 压缩中间件
│   ├── models/                  # 数据模型
│   ├── mqtt/                    # MQTT 告警推送
│   └── simulation/              # FEM + 梯度下降优化
├── frontend/
│   ├── index.html
│   ├── css/style.css
│   └── js/
│       ├── api.js
│       ├── app.js
│       └── bell3d.js           # Three.js GPU 着色器模态动画
├── simulator/
│   ├── Dockerfile
│   └── bell_simulator.py       # 可配置调音模拟器
├── mqtt/
│   └── mosquitto.conf          # MQTT Broker 配置
├── sql/
│   ├── init.sql              # 初始化 DDL + 种子数据
│   └── timescale_policies.sql  # 降采样保留策略
├── monitoring/
│   ├── prometheus.yml            # Prometheus 采集配置
│   └── grafana/
│       └── provisioning/datasources/  # 自动数据源
├── nginx/
│   └── nginx.conf              # nginx gzip/brotli + 缓存
├── docker-compose.yml         # 一键编排
├── .env.example           # 环境变量模板
└── README.md
```
