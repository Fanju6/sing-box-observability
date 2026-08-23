# 架构与接口

## 产品边界

项目只读取 sing-box 专用 `/observability/v1` API，适合设备本地或小型私有部署。它负责展示当前状态、趋势、连接历史、排行和只读运行信息。

明确不支持：

- 修改 sing-box 配置
- 选择代理、切换模式、修改路由或规则
- 主动关闭连接
- Clash 兼容管理接口
- DNS、证书、日志管理或其他无关 sing-box API
- 任何写回 sing-box 的数据路径

## 数据流

```text
浏览器（React SPA）
  │ 同源 REST + SSE + HttpOnly 会话
  ▼
Go 服务
  ├─ API/BFF 与认证
  ├─ 当前快照缓存
  ├─ 上游 SSE 订阅与浏览器广播
  ├─ Prometheus 指标抓取和速率计算
  ├─ 活动连接校准
  ├─ SQLite 历史和保留策略
  └─ 可选的嵌入式前端
  │ Authorization: Bearer <上游密钥>
  ▼
sing-box /observability/v1
```

Go BFF 确保上游令牌不会进入浏览器，让所有浏览器共享一条上游 SSE，并把 Prometheus 文本、JSON 和 SSE 统一成稳定的同源 API。SQLite 用于保存轻量的短期趋势，无需部署 Prometheus 或 Grafana。

## 采集与历史

- 启动采集前必须先验证 `/capabilities`。
- 默认每 2 秒抓取指标，每 15 秒持久化，每 3 秒完整校准活动连接。
- SSE 的 open/close 事件用于降低显示延迟，完整校准结果才是活动连接真值。
- SSE 重连或事件 ID 缺口会触发全量校准和下游 `resync`。
- 速率按相邻有效计数器差值除以真实时间计算；计数器下降视为重置。
- 超过三个抓取周期的缺口不会被跨越计算。
- 自动降采样保证每条序列最多返回 720 个点。
- SQLite 默认保留 7 天并使用 WAL；数据库异常只降级历史功能，不中断采集。

## 上游兼容要求

上游必须报告 `apiVersion: 1`、`cursorPagination: true`，并提供以下端点：

| 端点 | 格式 | 用途 |
|---|---|---|
| `/capabilities` | JSON | 能力、限制、端点和排行维度 |
| `/metrics` | Prometheus text | 运行时、流量、连接、URLTest 和 API 健康指标 |
| `/status` | JSON | 当前运行状态与容量 |
| `/connections/active` | JSON | cursor 分页的活动连接 |
| `/connections/recent` | JSON | 时间窗口与 cursor 分页的近期连接 |
| `/top` | JSON | 指定维度、窗口和数量的排行 |
| `/events` | SSE | 带单调事件 ID 的 open/close 事件 |

不会从旧 `/status` 推导 capabilities，不会发送 offset 上游分页，也不会模拟缺失端点。上游返回畸形数据、401、超时或不兼容协议时，服务只暴露安全错误码，不会记录或返回令牌及上游响应正文。

典型的实验配置如下；字段仍可能随上游分支变化，应以实际源码为准：

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "replace-with-a-secret"
    },
    "observability": {
      "enabled": true,
      "recent_connections": 2000,
      "recent_ttl": "1h",
      "top_k_size": 100,
      "expose_sensitive": false
    }
  }
}
```

## 控制台 API

`contracts/openapi.yaml` 是前后端唯一契约，控制台 API 不是上游 API 的直接镜像。

| 路由 | 用途 |
|---|---|
| `GET /healthz` | 进程存活，不泄露上游信息 |
| `GET /readyz` | API 与 SQLite 就绪状态 |
| `GET/POST/DELETE /api/v1/session` | 可选控制台会话 |
| `GET /api/v1/meta` | 来源状态、capabilities 和采集器信息 |
| `GET /api/v1/overview` | 当前值、区间汇总、趋势和简要排行 |
| `GET /api/v1/rankings` | 指定维度的历史排行 |
| `GET /api/v1/connections/active` | 已校准的活动连接 |
| `GET /api/v1/connections/recent` | SQLite 中保留的近期连接 |
| `GET /api/v1/events` | 来源和连接生命周期 SSE |

时间预设为 `15m`、`1h`、`6h`、`24h` 和 `7d`。自定义 `from`/`to` 使用 RFC3339，不能与 `range` 同时使用。缺失、重置、采样不足或敏感字段关闭时使用 `null` 或省略字段，不伪造零值。

浏览器 SSE 事件包括 `hello`、`source.state`、`connection.open`、`connection.close` 和 `resync`。SSE 只负责失效通知和降低延迟，REST 响应仍是真值来源。

## 安全边界

- 默认同源部署，不开启宽泛 CORS。
- 空控制台令牌只允许回环监听。
- 登录成功后使用 HttpOnly、SameSite=Strict Cookie。
- 未认证的回环模式会拒绝非回环 Host，降低 DNS rebinding 风险。
- 健康检查不返回令牌、上游 URL、连接数据或详细版本。
- 上游关闭 `expose_sensitive` 后，历史敏感字段也会在搜索和响应阶段过滤。
