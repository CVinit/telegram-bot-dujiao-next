# Research: Local dujiao-next integration surface

- Query: 盘点本地项目所有调用上游 dujiao-next 的 HTTP endpoint、请求/响应 DTO、业务命令与测试，形成受影响矩阵。
- Scope: internal / mixed
- Date: 2026-09-07

## Findings

### 总结

本项目是 Go Telegram 管理 Bot。所有 dujiao-next HTTP 调用集中在 internal/api/client.go，业务层通过 api.Client 间接调用，没有在 handler 中直接拼接 dujiao-next 请求。当前集成包含登录、订单列表、订单详情、订单状态更新、发货、卡密批量补充、商品列表、销量概览、库存告警等 endpoint。

### Endpoint / DTO / 调用方受影响矩阵

| Endpoint | 方法 / 鉴权 | 请求 DTO / 参数 | 响应 DTO | 本地 API 方法 | 业务调用方 / 命令 | 相关测试 |
|---|---|---|---|---|---|---|
| /api/v1/admin/login | POST；无 Bearer，管理员凭据 | model.LoginRequest：username、password | model.Response + model.LoginResponseData：requires_totp、token、expires_at、user | 私有 login，client.go:67-128 | 启动时 EnsureToken：cmd/bot/main.go:35-37；EnsureToken 和刷新协程也会触发 | 没有登录 HTTP 测试；model_test.go:187-199 测试通用 envelope |
| /api/v1/admin/orders?page=...&page_size=...&status=... | GET；Bearer JWT；分页 | page、page_size、可选 status；可加 sort_by、sort_order | model.PageResponse + []model.Order + model.Pagination | ListOrders / listOrders，client.go:223-250 | ListFulfillingOrders 给 /orders、/fulfill、/pfulfill；ListRecentOrders 给支付告警 | client_test.go:15-73 验证 updated_at desc 和分页；model_test.go:201-242 测试分页/订单 JSON |
| /api/v1/admin/orders/:id | GET；Bearer JWT | orderID 路径参数，无 body | model.Response + model.Order | GetOrder，client.go:316-326 | 发货后查询子订单及母订单状态：handler.go:735-740、848-866 | 无专门 HTTP mock；model_test.go:215-242 测试 Order DTO |
| /api/v1/admin/orders/:id | PATCH；Bearer JWT | 内联 map[string]string：status=completed | model.Response；data 不消费 | UpdateOrderStatus，client.go:309-314 | /fulfill 和 /pfulfill 每次发货成功后调用：handler.go:717-721、827-831 | 无状态更新 HTTP mock；docs/DUJIAO_NEXT_ORDER_RECOVERY.md 记录上游状态边界 |
| /api/v1/admin/fulfillments | POST；Bearer JWT | model.CreateFulfillmentRequest：order_id、payload、可选 delivery_data | model.Response + model.FulfillmentResponse | CreateFulfillment，client.go:297-307 | /fulfill 的 processFulfillSecrets：handler.go:671-776；/pfulfill 的 processParentFulfillSecrets：handler.go:778-870 | 无专门 HTTP mock；handler 测试只覆盖本地分组/解析/展示 |
| /api/v1/admin/card-secrets/batch | POST；Bearer JWT | model.CreateCardSecretBatchRequest：product_id、sku_id、secrets、可选 batch_no、note | model.Response + model.CreateCardSecretBatchResponse：created、batch_id、batch_no | CreateCardSecretBatch，client.go:328-338 | /cards：ListProducts 后选择商品/SKU，提交文本或文件后 processCardSecrets：handler.go:112-145、387-454、627-668 | 无 API mock；没有 cards 到请求 JSON 的完整链路测试 |
| /api/v1/admin/products?page=...&page_size=... | GET；Bearer JWT；分页 | page、page_size | model.PageResponse + []model.Product + model.Pagination；Product 可含 []model.SKU | ListProducts，client.go:340-351 | loadAllProducts：handler.go:939-953；/cards 过滤 auto；/stock 展示库存 | 无产品 API mock；model_test.go:99-116 和 handler_test.go:192-258 测试 SKU/库存展示 |
| /api/v1/admin/dashboard/overview?range=today/7d/30d | GET；Bearer JWT | handleSalesCallback 固定传 range：today、7d、30d | model.Response + model.DashboardOverview，含 KPI、Funnel、Alerts | GetDashboardOverview，client.go:353-367 | /sales 展示选择器，回调在 handler.go:355-385 | model_test.go:244-278 测试 Dashboard DTO；无 HTTP query mock |
| /api/v1/admin/dashboard/inventory-alerts | GET；Bearer JWT | 无 body / 查询参数 | model.Response + []model.InventoryAlert：商品、SKU、sku_spec_values、库存 | GetInventoryAlerts，client.go:369-378 | StockAlertChecker.check：handler.go:1084-1157；/stock 不直接使用此 endpoint | model_test.go:118-185、handler_test.go:380-397 测试告警 DTO/展示；无 HTTP mock |

### 认证、传输和 envelope

- api.Client 的 baseURL、管理员凭据、JWT 和过期时间位于 internal/api/client.go:18-27，token/过期时间由 sync.RWMutex 保护。
- doRequest 和 doPageRequest 会先 EnsureToken，设置 Authorization: Bearer token 和 Content-Type: application/json：client.go:131-218。
- 非分页响应使用 model.Response，分页响应使用 model.PageResponse；只有 status_code == 0 才继续：client.go:94-101、170-178、210-218。
- EnsureToken 在 token 为空或距离过期不足 5 分钟时重新登录：client.go:57-65。启动主动登录，运行后由 StartRefreshLoop 定时刷新：cmd/bot/main.go:35-38、internal/bot/bot.go:96-104。
- requires_totp 为 true 或 token 缺失时登录失败：client.go:108-113。
- 配置来自 config.yaml / 环境变量，关键字段为 dujiao.base_url、DUJIAO_USERNAME、DUJIAO_PASSWORD、jwt_refresh_interval：config.yaml:6-10、internal/config/config.go、.env.example:4-6。

### 请求 / 响应 DTO 盘点

- 通用：Response、PageResponse、Pagination，定义于 model.go:11-29。
- 认证：LoginRequest、LoginResponseData、AdminUser，定义于 model.go:33-48。
- 订单：Order 覆盖标识、状态、金额、支付时间、创建/更新时间、Items、Children；OrderItem 覆盖 product_id、sku_id、多语言 title、sku_snapshot、数量、金额和发货快照字段：model.go:52-98。
- 发货：CreateFulfillmentRequest、FulfillmentResponse，model.go:102-119。
- 卡密：CreateCardSecretBatchRequest、CreateCardSecretBatchResponse，model.go:123-135。
- 商品：Product 和 SKU，含多语言标题、fulfillment_type、库存、is_active、spec_values：model.go:139-182。
- 看板：DashboardOverview、DashboardKPI、DashboardFunnel、DashboardAlert：model.go:186-235；库存告警为 InventoryAlert：model.go:239-248。
- SKU/多语言展示辅助函数集中在 model.go:262-360。订单展示使用 GetOrderItemDisplayName；发货分组使用 GetOrderItemGroupKey，handler.go:885-913，按商品 + SKU 区分。

### 业务命令与 HTTP 调用链

| Telegram 入口 | 命令 / 回调 | 上游调用链 | 关键状态 / 副作用 |
|---|---|---|---|
| OnSales | /sales -> sales|today/week/month | GetDashboardOverview | 只读销量；handler.go:62-73、355-385 |
| OnOrders | /orders | ListFulfillingOrders -> orders status=fulfilling、partially_delivered 各一次 | 展示待处理叶子订单；handler.go:75-110 |
| OnCards | /cards -> cards|product_id -> cards_sku|sku_id -> 文本/文件 | ListProducts 分页；CreateCardSecretBatch | 会话保存商品/SKU 选择；handler.go:112-145、387-454、627-668 |
| OnFulfill | /fulfill -> fulfill|product_id:sku_id -> 文本/文件 | ListFulfillingOrders；每个订单 CreateFulfillment，成功后 UpdateOrderStatus，最后 GetOrder | FIFO；卡密不足时尽可能完成整单；handler.go:149-199、456-497、671-776 |
| OnParentFulfill | /pfulfill -> pfulfill|parent_id -> 文本/文件 | ListFulfillingOrders；每个子订单 CreateFulfillment + UpdateOrderStatus；全成功后 GetOrder 母订单 | 严格总数量校验；handler.go:201-245、499-563、778-870 |
| OnStock | /stock | ListProducts 分页 | 本地按自动发货商品和 SKU 库存格式化；handler.go:248-268、939-953 |
| 后台库存检查 | 无命令，启动后 ticker | GetInventoryAlerts | 按阈值和 product_id:sku_id 去重，立即告警及 08/12/18 定时提醒；main.go:47-50、handler.go:1035-1158 |
| 后台支付检查 | 无命令，启动后 ticker | ListRecentOrders -> updated_at desc 分页 | 首次播种已支付订单；后续按非空 paid_at 发送新订单告警；main.go:52-54、handler.go:1160-1230 |

Telegram 路由在 internal/bot/bot.go:64-76 注册，命令菜单在 bot.go:83-94 定义。

### 测试覆盖盘点

| 测试文件 | 覆盖内容 | 对集成变更的保护范围 | 缺口 |
|---|---|---|---|
| internal/api/client_test.go:15-73 | httptest mock 订单列表 | 路径、排序参数、分页页码、ListRecentOrders 扫描 | 只覆盖订单列表；登录、Bearer、业务错误、其它 endpoint 无 HTTP contract test |
| internal/model/model_test.go:8-185 | 商品/SKU/订单项/库存告警名称和 JSON 解码 | 保护 sku_snapshot、sku_spec_values、多语言回退、分页和订单 envelope | 未覆盖所有 DTO 字段及请求 JSON 序列化 |
| internal/model/model_test.go:187-278 | Response、PageResponse、订单、dashboard DTO | 保护基础 response shape 和 dashboard 映射 | 未验证 API client 对 status_code != 0 的处理 |
| internal/handler/handler_test.go:11-124 | 解析工具、callback 数据、订单商品摘要 | 保护输入解析和 SKU 展示 | 未构造 telebot context 执行完整命令链 |
| internal/handler/handler_test.go:125-338 | SKU 分组、库存格式化、叶子订单解析 | 保护 /orders、/fulfill、/stock 核心本地逻辑 | 未验证 API 调用次数、请求 DTO 或状态更新顺序 |
| internal/handler/handler_test.go:340-415 | 支付订单去重、库存告警、支付告警格式化 | 保护两个后台 checker 的筛选/展示逻辑 | 没有 checker + fake API 集成测试 |
| internal/bot/bot_test.go:5-33 | 命令菜单包含 8 个命令 | 保护 Telegram 命令面 | 不验证实际 handler 回调行为 |
| internal/state/state_test.go:8-88 | 会话状态 TTL、读写、清理、覆盖 | 间接保护 cards、fulfill、pfulfill 多步状态容器 | 不验证业务状态字段 schema |

### 相关本地文档与规格

- .trellis/spec/backend/database-guidelines.md：确认无本地数据库，数据契约来自 dujiao-next Admin API；列出 endpoint、envelope、分页、JWT、DTO 和写操作约定。
- .trellis/spec/backend/directory-structure.md：确认 internal/api/client.go 是 HTTP 边界，internal/model/model.go 是 JSON 映射边界，internal/handler/handler.go 负责命令工作流。
- .trellis/spec/backend/quality-guidelines.md：当前质量门禁和 API/模型边界约定。
- .trellis/spec/guides/cross-layer-thinking-guide.md：适用于 upstream API -> DTO -> handler -> Telegram 展示的数据流审查。
- README.md:5-23、53-56、208-213：命令、运行依赖和包职责。
- docs/DUJIAO_NEXT_ORDER_RECOVERY.md:3-76：订单状态更新和已交付内容不可通过库存编辑回写的边界。

### External references (docs, versions)

- 上游仓库：https://github.com/dujiao-next/dujiao-next；本地恢复边界文档记录的检查提交为 c81ab92f946cafc8c2630c0f5f6e6fb17e0ce989，来源 docs/DUJIAO_NEXT_ORDER_RECOVERY.md:9-12。
- 上游相关路径由本地恢复文档引用：internal/http/handlers/admin/card_secret_admin.go、internal/service/card_secret_service.go、internal/service/fulfillment_service.go、internal/service/order_service.go 等。
- 本地 Go 版本为 go 1.23；依赖 gopkg.in/telebot.v3 v3.3.8、gopkg.in/yaml.v3 v3.0.1，见 go.mod:1-8。

## Caveats / Not Found

- 本轮只审查本地 checkout，没有重新拉取或在线核验当前 upstream HEAD。兼容性结论以当前 Go 客户端、仓库内规格和 docs/DUJIAO_NEXT_ORDER_RECOVERY.md 记录的上游提交为准，不代表 2026-09-07 的 upstream 最新契约。
- 未发现其它直接调用 dujiao-next 的 Go HTTP client：rg 只在 internal/api/client.go 找到 Admin API 请求构造；handler.go:1010-1032 的 http.Get 是 Telegram Bot 文件下载，不是 dujiao-next endpoint。
- 未发现独立 API schema、OpenAPI、代码生成 DTO 或 contract-test 目录；DTO 全部手写在 internal/model/model.go。
- GetDashboardOverview 接收原始 query string，当前调用方只传固定 range；上游查询参数变更时，这是低层方法与 handler 的影响点。
- ListProducts 与 ListOrders 的分页由客户端/handler 手写；ListFulfillingOrders 对每个 status 固定只拉第 1 页、每页 100 条，超过 100 条时可能漏单。
- /fulfill、/pfulfill 忽略 UpdateOrderStatus 错误，可能出现 fulfillment 已创建但订单状态未完成的部分成功状态：handler.go:718-720、828-830。
- processParentFulfillSecrets 使用 children[i].ID 查询状态，而 results 只追加数量非零的子订单；存在零数量子订单时，结果索引可能错位：handler.go:808-825、846-853。
- 登录、订单详情、PATCH、发货、卡密、商品、dashboard、inventory-alerts 均缺少端到端 HTTP mock；现有测试主要只能在 JSON helper 和一条订单列表路径上发现契约变化。
- 上游 API 的 HTTP 状态码处理没有独立断言；代码按 status_code 判断业务成功，且注释明确上游可能始终返回 HTTP 200。
