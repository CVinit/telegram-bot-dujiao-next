# Research: 上游 dujiao-next 历史与 API 风险

- Query: 研究官方仓库最近的 release、commit、changelog、README、API 路由与响应模型，判断截至 2026-09-07 已落地或正在进行、会影响本 Telegram 机器人的 API 变化。
- Scope: mixed
- Date: 2026-09-07

## Findings

### 结论

官方仓库截至 2026-09-07 的最新正式 release 是 v1.4.7（2026-09-02）；main 最新可见提交为 2026-09-03 的 f06035ca359b。没有看到明确宣布删除当前 Admin API 路径的 release note；当前 Bot 依赖的登录、订单列表/详情/状态更新、人工交付、批量导入卡密、商品分页、仪表盘概览和库存告警路由仍存在。

升级风险中高：上游在 2026-07 下旬完成了从旧目录到模块化 transport/application/domain 的大规模内部重构，外部 JSON 契约也出现实质扩展；最近版本进一步收紧履约校验、修正父子订单与上游采购归属、增加退款相关状态和字段。生产建议只跟 release，升级前做 HTTP fixture 回归。

### 已落地的 release / commit 变化

- v1.4.7（2026-09-02）：Stripe Checkout Session 自动带订单邮箱；支付渠道商品名统一改为订单号；修正 Stripe 换汇回调金额校验/币种标注；okpay 协议升级为 HMAC-SHA256 + timestamp/nonce；加强采购单连接归属和上游订单号一致性。
- v1.4.6 / v1.4.5（2026-08-27）：新增 Feishu 通知、BEpusdt 指定收款地址；修复并发优惠券次数绕过、退款手续费回冲/历史退款兼容、支付手续费和商品名问题，并增强订单履约校验。
- v1.4.3（2026-08-07）：微信支付平台证书/公钥验签、去掉订单超时/取消邮件通知、增强游客订单风控。
- v1.4.2 / v1.4.1 / v1.4.0：官方安装管理器、二进制一键升级、单二进制/fullstack 构建；v1.4.0 还集中合并退款/部分退款、分销、订单配置、通知中心、商品分类等能力。
- v1.3.1（2026-07-08）：上游同步参数改为后台可配置；商品映射支持上游状态/商品状态筛选和名称搜索；优惠券规则增强；修复“上游交付商品导致通知误报”。

### 直接影响 Bot 的 API / 数据契约

#### 1. 订单响应字段扩展，金额和状态语义变复杂

官方当前 internal/modules/order/transport/presenter/order.go 的订单详情包含 allowed_payment_channel_ids、refund_records、fulfillment、children；订单项增加 original_unit_price、original_total_price、wholesale_discount_amount。金额使用 money.Amount，internal/shared/money/amount.go 固定输出两位小数字符串，同时接受 JSON number/string。

本地 internal/model/model.go 仍是旧字段集合，金额保留 string 是正确方向，但会丢新字段。internal/handler/handler.go 只展示传统订单状态，未处理退款/部分退款分支。未知 JSON 字段不会让 Go 解码失败，风险在于展示和决策不完整。

风险：中。

#### 2. 履约状态与 Bot 的二次 PATCH 存在高风险

官方当前常量包含 pending_payment、paid、fulfilling、partially_delivered、partially_refunded、delivered、completed、canceled、refunded。官方父订单汇总明确区分：全部 completed 才是 completed；delivered + completed 全部完成才是 delivered；部分完成是 partially_delivered；退款会产生 partially_refunded/refunded。

官方当前 internal/modules/fulfillment/application/service.go 的人工交付只接受 paid 或 fulfilling 的叶子订单，创建 fulfillment 后事务内直接把叶子订单写为 delivered，再同步父订单；重复交付返回 ErrFulfillmentExists。

本地 internal/handler/handler.go:711-720、821-830 创建 fulfillment 后又 PATCH completed，且忽略 PATCH 错误。这个调用与当前上游“创建人工交付即 delivered”的语义不完全一致，可能导致成功交付后状态更新失败但 Bot 仍报告成功。

本地 internal/api/client.go:281-294 只按 fulfilling、partially_delivered 查询；上游人工交付允许 paid 直接交付。若部署版本或调用场景依赖 status 过滤，需明确纳入 paid。

风险：高。优先回归叶子订单、母子订单部分交付、重复交付、退款后查看/发货，以及创建 fulfillment 后 PATCH 返回。

#### 3. 卡密导入已增加 deduplicate

官方 internal/modules/cardsecret/transport/http/admin_handler.go 的批量请求新增 Deduplicate *bool，路由仍为 POST /api/v1/admin/card-secrets/batch。

本地 internal/model/model.go 的 CreateCardSecretBatchRequest 没有该字段，internal/handler/handler.go 使用上游默认值。官方 v1.1.0 已落地导入卡密去重开关；循环卡密/重复卡密场景的 created 数量和库存结果可能随后台默认设置变化，Bot 无法显式选择策略。

风险：中，不是路径断裂，而是已落地行为开关未暴露。

#### 4. 商品和 SKU 库存字段扩展

官方商品 presenter 提供 manual_stock_available、auto_stock_available（int64）、stock_status、is_sold_out、stock_display_mode、stock_display、stock_range_min/max、stock_quantity_hidden，SKU 也有同类字段；Admin 商品列表仍是 GET /api/v1/admin/products。

本地只使用 auto_stock_available 整数，没有处理模糊库存、隐藏库存或售罄状态。v1.4.0 已落地更准确的自动交付库存信息，因此 /stock 可能与上游后台或店面显示口径不同。

风险：中。建议未来读取 stock_status/is_sold_out/stock_display，不要只依赖精确库存数。

#### 5. Admin 登录增加 TOTP challenge，当前 Bot 明确不兼容

官方登录仍是 POST /api/v1/admin/login，但请求可带 captcha_payload；开启 2FA 时返回 requires_totp=true、challenge_token、challenge_expires_at，而不是直接返回 token。验证路由为 POST /api/v1/admin/login/verify-2fa。

本地 internal/api/client.go:103-110 遇到 requires_totp 直接报 Bot 暂不支持。后台 2FA 是 v1.0.5 已落地能力，因此使用开启后台 2FA 的管理员账号时，Bot 会启动登录失败。

风险：高。短期需使用未开启后台 2FA 的专用管理员账号；长期需实现人工输入或配置 TOTP 的 challenge 流程。

#### 6. 订单筛选扩展，现有路由仍兼容但分页有容量风险

官方订单列表仍为 GET /api/v1/admin/orders，支持 page/page_size/status/user_id/user_keyword/order_no/guest_email/created_from/created_to/product_keyword/sort_by/sort_order；详情和状态更新仍为 GET /api/v1/admin/orders/:id、PATCH /api/v1/admin/orders/:id。

本地 internal/api/client.go 使用的参数仍在官方契约内；但 ListFulfillingOrders 每个状态只取第 1 页、每页 100，待发货订单超过 100 时会漏单。

风险：中高，属于现有容量问题，不是新版本才出现的路径变化。

#### 7. Dashboard 增加趋势/排行，overview 也扩展

官方 dashboard 保留 GET /api/v1/admin/dashboard/overview 和 GET /api/v1/admin/dashboard/inventory-alerts，并新增 GET /api/v1/admin/dashboard/trends、GET /api/v1/admin/dashboard/rankings。overview 当前包含 currency，KPI 增加处理订单、退款、利润、支付成功率等指标；查询支持 range/from/to/tz/force_refresh。

本地只用 overview；新增字段会被忽略，不会直接造成解析失败，但 /sales 不是完整利润报表。

风险：低到中。

### 正在进行或近期 main 变化

- 2026-07-26 左右出现 remove structural compatibility aliases、split runtime from task consumers、separate platform connection and migrations 等大规模垂直迁移。它们主要改变上游内部包路径和依赖边界，不直接删除 HTTP 路径，但跟 main 部署的回归面较大。
- 2026-09-02 的 47cc620 强化采购单回调归属：认证连接必须匹配，已登记上游订单号必须一致。Bot 当前不直接调用采购单回调 API，但未来接入采购/同步时不能只凭本地订单号回调。
- 2026-09-03 的订单详情优先展示卡密/子订单、游客查单入口和 Vault 移动端布局改动主要是前端，不改变 Bot 当前 Admin API 调用。
- v1.4.7 的 okpay HMAC、Stripe/BEpusdt/微信支付改动主要作用于支付网关；Bot 不代发支付回调，直接影响较小，但可能间接改变订单 paid_at/status/total_amount，应做真实订单通知和销量回归。

### 关键风险矩阵

| 优先级 | 风险 | 受影响代码 | 建议验证 |
|---|---|---|---|
| P0 | 后台 2FA 登录失败 | internal/api/client.go:103-110 | 用开启 2FA 的账号验证 challenge；确认运行账号策略 |
| P0 | fulfillment 后二次 PATCH completed 与 delivered 语义不一致 | internal/handler/handler.go:711-720,821-830 | 叶子/母子订单发货回归；记录 PATCH 返回，不再吞错 |
| P1 | paid 是否纳入待发货、分页漏单 | internal/api/client.go:281-294 | 制造超过 100 条订单，测试 paid/fulfilling/partially_delivered |
| P1 | 卡密去重开关未暴露 | internal/model/model.go:123-129 | 重复/循环卡密测试默认行为与显式 true/false |
| P1 | 订单/库存字段未映射 | internal/model/model.go:52-181 | 用 release JSON fixture 验证退款、批发折扣、库存显示 |
| P2 | dashboard 新查询/趋势排行未使用 | internal/api/client.go:353-378 | 对比后台 range/tz/force_refresh 结果 |

### Files found

本地：

- README.md：Bot 命令、部署方式、当前 Admin API 功能边界。
- internal/api/client.go：登录、envelope、分页以及订单/履约/卡密/商品/dashboard 调用。
- internal/model/model.go：Bot 侧 JSON 模型。
- internal/handler/handler.go：Telegram 流程、库存/订单提醒和发货编排。
- docs/DUJIAO_NEXT_ORDER_RECOVERY.md：旧 upstream commit 下的订单补发边界。
- .trellis/spec/backend/index.md、database-guidelines.md、error-handling.md、quality-guidelines.md：本地 API 契约与验证约束。

上游官方源码：

- internal/modules/identity/adminauth/transport/http/admin_login_handler.go、routes.go：Admin 登录和 2FA challenge。
- internal/modules/order/transport/http/admin_handler.go、routes.go、presenter/order.go：订单列表、详情、状态更新和 JSON DTO。
- internal/modules/order/application/order_status.go：父子订单状态汇总。
- internal/modules/fulfillment/application/service.go、transport/http/admin_handler.go、routes.go：人工交付状态和路由。
- internal/modules/cardsecret/transport/http/admin_handler.go、routes.go：卡密批量导入及 deduplicate。
- internal/modules/catalog/product/transport/http/admin_handler.go、routes.go、presenter/product.go：商品/SKU 库存 DTO。
- internal/modules/dashboard/transport/http/routes.go、application/types.go、internal/modules/reporting/transport/http/query.go：dashboard 路由、响应和查询参数。
- internal/platform/http/response/response.go、internal/shared/money/amount.go、internal/constants/constants.go：响应 envelope、金额和状态常量。

### Official references

仅使用官方仓库内容：

- README：https://github.com/dujiao-next/dujiao-next/blob/main/README.md
- Releases：https://github.com/dujiao-next/dujiao-next/releases
- 最新 release v1.4.7：https://github.com/dujiao-next/dujiao-next/releases/tag/v1.4.7
- v1.4.6：https://github.com/dujiao-next/dujiao-next/releases/tag/v1.4.6
- v1.4.3：https://github.com/dujiao-next/dujiao-next/releases/tag/v1.4.3
- v1.3.1：https://github.com/dujiao-next/dujiao-next/releases/tag/v1.3.1
- main commits：https://github.com/dujiao-next/dujiao-next/commits/main/
- 关键 commit：f57247b4eebe20d803a78fc4dba265c2649bd88d、840d117ed3faa5096a6809089dd20d8c4af30c2b、3784e4743444dfb1ab5a6ec81f6fed1a4dc4869c、47cc6207c6796ccfda0526a3708be39e1b59eba3。

## Related specs

- .trellis/spec/backend/database-guidelines.md：Admin API 契约、响应 envelope、分页和模型映射。
- .trellis/spec/backend/error-handling.md：统一处理 status_code，不要只看 HTTP 状态；后台循环错误当前会静默。
- .trellis/spec/backend/quality-guidelines.md：API 变更需补 httptest/模型合同测试，调用保持在 internal/api.Client。
- .trellis/spec/backend/index.md：后端分层与现有包/测试优先。
- docs/DUJIAO_NEXT_ORDER_RECOVERY.md：旧版源码基线下的交付 payload、取消订单恢复边界，需要按 v1.4.7 当前源码重新复核。

## Caveats / Not Found

- 本次只使用 github.com/dujiao-next/dujiao-next 官方 GitHub 的 README、Release、commit 和 raw 源码；没有使用博客、第三方镜像、论坛或非官方文档。
- GitHub API 查询到的 release 最新为 v1.4.7，main 提交最新为 2026-09-03；未看到 2026-09-04 至 2026-09-07 的官方新记录。页面后续变化需要刷新本报告。
- 本地 docs/DUJIAO_NEXT_ORDER_RECOVERY.md 基线为 2026-05-22 的 c81ab92，早于 v1.4.7；其中“取消订单无出站转换”“已交付 payload 无更新接口”等结论本次没有逐项完整复验，不能直接视为当前 release 的最终契约。
- 没有运行真实 dujiao-next 实例，无法证明具体部署的配置、权限种子或数据库迁移状态；影响判断来自官方 release/changelog 和当前源码。
- 上游源码使用 money.Amount 等内部类型；Bot 不应导入上游 Go 包，应以 HTTP JSON fixture 做兼容测试。
