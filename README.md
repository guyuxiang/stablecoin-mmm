# GLUSD/USDT 做市机器人

Uniswap V3 自动做市机器人，稳定币交易对价格保持在 1.0 附近。

## 功能特性

- **三层流动性头寸**: Core(核心深度) / Mid(缓冲层) / Tail(尾部防御)
- **自动再平衡**: 根据价格偏离和定时触发
- **风控熔断**: 价格偏离超过阈值自动暂停
- **实时监控**: 价格、手续费、Gas成本、净收益
- **API控制**: 启动/停止/查询状态
- **Web管理界面**: 可视化控制面板

## 配置

修改 `config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

uniswap:
  rpc_url: "https://unichain-sepolia-rpc.publicnode.com"
  chain_id: 1301
  pool_address: "0x..."          # 创建池子后填写
  factory_address: "0x1F98431c8aD98523631AE4a59f267346ea31F984"
  position_manager: "0xC36442b4a4522E871399CD717aBDD847Ab11FE88"
  swap_router: "0xE592427A0AEce92De3Edee1F18E0157C05861564"
  fee_tier: 500
  token0_address: "0x948e15b38f096d3a664fdeef44c13709732b2110"  # GLUSD
  token1_address: "0x2d7efff683b0a21e0989729e0249c42cdf9ee442"  # USDT

bot:
  private_key: "your_private_key"
  core_ratio: 0.6
  mid_ratio: 0.3
  tail_ratio: 0.1
  core_range_bps: 5
  mid_range_bps: 50
  tail_range_bps: 200
  rebalance_threshold: 0.002
  rebalance_interval_seconds: 60

risk:
  circuit_breaker_deviation_bps: 30
  max_daily_loss_bps: 50

oracle:
  ref_price: 1.0
```

## 快速开始

### 1. 编译

```bash
go build -o uniswap-bot .
```

### 2. 创建池子

```bash
./uniswap-bot create-pool config.yaml
```

输出:
```
Pool Address: 0x...
```

将池地址填入 `config.yaml` 的 `pool_address`。

### 3. 添加流动性

```bash
./uniswap-bot add-liquidity config.yaml
```

### 4. 启动机器人

```bash
./uniswap-bot start config.yaml
```

### 5. Web管理界面

启动后访问: http://localhost:8080

### 6. API控制

```bash
# 查看状态
curl http://localhost:8080/api/v1/status

# 启动机器人
curl -X POST http://localhost:8080/api/v1/start

# 停止机器人
curl -X POST http://localhost:8080/api/v1/stop

# 查看指标
curl http://localhost:8080/api/v1/metrics

# 查看告警
curl http://localhost:8080/api/v1/alerts
```

## 头寸结构

```
价格区间配置 (参考价 1.0):

Tail (尾部防御)  ──────────  [0.98 - 1.02]  占比 10%
Mid (缓冲层)     ───────────  [0.995 - 1.005]  占比 30%
Core (核心深度)  ──────────  [0.9995 - 1.0005] 占比 60%
                        ↑
                    价格 1.0
```

## 风控机制

- **熔断**: 价格偏离 > 30bps 触发熔断，暂停所有操作
- **单日最大损失**: 50bps
- **单笔最大交易**: 10bps
- **熔断持续时间**: 15分钟

## API 接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/` | GET | Web管理界面 |
| `/api/v1/status` | GET | 运行状态、熔断状态 |
| `/api/v1/metrics` | GET | 价格、手续费、净收益等 |
| `/api/v1/positions` | GET | 头寸信息 |
| `/api/v1/risk` | GET | 风控状态 |
| `/api/v1/balance` | GET | 账户余额 |
| `/api/v1/start` | POST | 启动机器人 |
| `/api/v1/stop` | POST | 停止机器人 |
| `/api/v1/alerts` | GET | 告警信息 |
| `/api/v1/rebalance` | POST | 触发再平衡 |
| `/api/v1/create-pool` | POST | 创建交易池 |
| `/api/v1/add-liquidity` | POST | 添加流动性 |
| `/api/v1/swap` | POST | 执行Swap交易 |

### 创建交易对 API

```bash
curl -X POST http://localhost:8080/api/v1/create-pool \
  -H "Content-Type: application/json" \
  -d '{
    "token0": "0x948e15b38f096d3a664fdeef44c13709732b2110",
    "token1": "0x2d7efff683b0a21e0989729e0249c42cdf9ee442",
    "fee": 500
  }'
```

### 添加流动性 API

```bash
curl -X POST http://localhost:8080/api/v1/add-liquidity \
  -H "Content-Type: application/json" \
  -d '{
    "token0": "0x948e15b38f096d3a664fdeef44c13709732b2110",
    "token1": "0x2d7efff683b0a21e0989729e0249c42cdf9ee442",
    "amount0": "1000000000000000000",
    "amount1": "1000000000000000000"
  }'
```

### Swap API

```bash
curl -X POST http://localhost:8080/api/v1/swap \
  -H "Content-Type: application/json" \
  -d '{
    "token_in": "0x948e15b38f096d3a664fdeef44c13709732b2110",
    "token_out": "0x2d7efff683b0a21e0989729e0249c42cdf9ee442",
    "amount_in": "1000000000000000000"
  }'
```

## 技术栈

- Go 1.21+
- Gin Web Framework
- go-ethereum
- Uniswap V3 SDK

## 目录结构

```
uniswap-bot/
├── main.go              # 主程序入口
├── config.yaml          # 配置文件
├── web/
│   └── index.html      # Web管理界面
├── config/              # 配置加载
├── pkg/
│   ├── api/             # HTTP API
│   ├── executor/        # 交易执行
│   ├── monitor/         # 监控告警
│   ├── oracle/          # 价格预言机
│   ├── position/        # 头寸管理
│   ├── rebalancer/      # 再平衡逻辑
│   ├── risk/            # 风控引擎
│   └── uniswap/         # Uniswap 交互
└── 做市.md              # 设计文档
```

## 许可证

MIT
