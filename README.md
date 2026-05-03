# Telegram Bot - 独角数卡管理工具

Go 语言编写的 Telegram Bot，对接 [dujiao-next](https://github.com/dujiao-next) 数字卡密发售平台，为店铺管理员提供移动端管理能力。

## 功能

| 命令 | 功能 |
|------|------|
| `/start` | 欢迎信息 + 功能概览 |
| `/sales` | 查看销量（今天/昨天/本周/本月） |
| `/orders` | 查看待处理订单 |
| `/cards` | 补充卡密（支持文本和文件上传） |
| `/fulfill` | 批量发货（按商品聚合，FIFO 自动分配卡密） |
| `/pfulfill` | 按母订单发货（严格数量校验，整单发或取消） |
| `/stock` | 查看库存概况 |
| `/cancel` | 取消当前操作 |

### 批量发货流程

1. `/fulfill` → Bot 按商品名聚合所有待发货子订单
2. 选择商品 → 显示需要多少个卡密
3. 发送卡密（每行一个，或上传 txt/csv 文件）
4. Bot 按 FIFO 时间顺序自动分配卡密，逐个调用 dujiao-next API 发货

### 按母订单发货流程

1. `/pfulfill` → Bot 列出所有含待发货子订单的母订单
2. 选择母订单 → 显示子订单列表及所需卡密总数
3. 发送卡密（数量必须精确匹配，否则不发货）
4. Bot 按 FIFO 顺序逐个子订单发货，完成后查询母订单状态并展示详情

### 缺货提醒

Bot 定时轮询库存，低于阈值时主动推送 Telegram 消息给管理员。

## 技术栈

- Go 1.23
- [telebot v3](https://gopkg.in/telebot.v3) — Telegram Bot SDK
- Long Polling 模式
- dujiao-next Admin API + JWT 自动刷新

## 快速开始

### 前置条件

- Go 1.23+
- 运行中的 dujiao-next 实例
- Telegram Bot Token（通过 [@BotFather](https://t.me/BotFather) 创建）

### 配置

复制示例配置并修改：

```bash
cp config.yaml myconfig.yaml
cp .env.example .env
```

编辑 `myconfig.yaml`：

```yaml
telegram:
  bot_token: "your-bot-token"     # 或设置环境变量 BOT_TOKEN
  allowed_users:                  # 允许使用的 Telegram User ID
    - 123456789

dujiao:
  base_url: "http://your-dujiao-instance:8080"
  admin_username: "admin"         # 或设置环境变量 DUJIAO_USERNAME
  admin_password: "your-password" # 或设置环境变量 DUJIAO_PASSWORD
  jwt_refresh_interval: 30m

stock_alert:
  check_interval: 5m
  threshold: 10
```

获取你的 Telegram User ID：向 [@userinfobot](https://t.me/userinfobot) 发送任意消息即可获得。

### 直接运行

```bash
go build -o bot ./cmd/bot
./bot -config myconfig.yaml
```

或使用环境变量覆盖敏感信息：

```bash
export BOT_TOKEN="your-bot-token"
export DUJIAO_USERNAME="admin"
export DUJIAO_PASSWORD="your-password"
./bot -config myconfig.yaml
```

### Docker 运行

```bash
docker build -t telegram-bot-dujiao-next .

docker run -d \
  -e BOT_TOKEN=your-bot-token \
  -e DUJIAO_USERNAME=admin \
  -e DUJIAO_PASSWORD=your-password \
  -v $(pwd)/config.yaml:/etc/bot/config.yaml \
  telegram-bot-dujiao-next
```

### Docker Compose（使用 GHCR 镜像）

创建 `.env` 文件：

```bash
cp .env.example .env
```

编辑 `.env`，填入敏感信息：

```
BOT_TOKEN=your-bot-token
DUJIAO_USERNAME=admin
DUJIAO_PASSWORD=your-password
DUJIAO_BASE_URL=http://your-dujiao-instance:8080
```

创建 `docker-compose.yml`：

```yaml
services:
  bot:
    image: ghcr.io/cvinit/telegram-bot-dujiao-next:latest
    env_file:
      - .env
    volumes:
      - ./config.yaml:/etc/bot/config.yaml
    restart: unless-stopped
```

`config.yaml` 中只需配置非敏感项，敏感信息由 `.env` 覆盖：

```yaml
telegram:
  bot_token: ""
  allowed_users:
    - 123456789

dujiao:
  base_url: ""
  admin_username: ""
  admin_password: ""
  jwt_refresh_interval: 30m

stock_alert:
  check_interval: 5m
  threshold: 10
```

启动：

```bash
docker compose up -d
```

查看日志：

```bash
docker compose logs -f bot
```

> GHCR 镜像支持 linux/amd64 和 linux/arm64，树莓派等 ARM 设备可直接使用。

## CI/CD

推送代码到 GitHub 后自动触发：

- **构建**：为 linux/amd64、linux/arm64、darwin/amd64、darwin/arm64、windows/amd64 编译二进制文件
- **Docker 镜像**：推送到 GHCR（`ghcr.io/<owner>/telegram-bot-dujiao-next`），支持 amd64 + arm64
- **Release**：推送 `v*` tag 时自动创建 GitHub Release 并上传所有二进制文件

发布新版本：

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 项目结构

```
cmd/bot/main.go          — 入口
internal/
  config/config.go       — 配置加载（YAML + 环境变量）
  bot/bot.go             — telebot 初始化、中间件、路由
  handler/handler.go     — 命令处理器、回调处理、缺货提醒
  api/client.go          — dujiao-next Admin API 客户端
  model/model.go         — 数据结构定义
  state/state.go         — 对话状态管理（内存 + TTL）
```

## 许可证

MIT
