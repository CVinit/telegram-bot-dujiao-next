# Research: upstream-api-contract

- Query: 研究官方 GitHub 仓库 dujiao-next/dujiao-next 当前 main 分支相对本项目可能影响 Telegram 管理机器人集成的 API、数据模型与接口变化，重点覆盖商品、SKU、订单、库存、卡密、认证、分页和错误结构。
- Scope: mixed
- Date: 2026-09-07

## Findings

### 研究基线与总体结论

官方 GitHub main 当前 HEAD 为 f06035ca359b0e4e99908fa6ae232af85e923d7f，提交时间为 2026-09-03，提交信息为订单详情页优先展示卡密与子订单。提交页：<https://github.com/dujiao-next/dujiao-next/commit/f06035ca359b0e4e99908fa6ae232af85e923d7f>。

截至已读取内容，没有发现本项目正在使用的核心 Admin API 路径被删除或重命名。主要影响是响应字段和语义比本项目模型更丰富，以及认证、履约校验和卡密 SKU 约束更严格。当前本项目已经正确使用 status_code == 0、Bearer JWT、pagination，以及 SKU 快照展示逻辑。

### 1. 核心路由对照

官方 admin 路由统一挂在 /api/v1/admin 下，并通过 JWT + RBAC 保护；证据：官方 internal/app/httpserver/routes_admin.go:87-157。

- 认证：官方 internal/modules/identity/adminauth/transport/http/routes.go:5-19 注册 POST /login 和 POST /login/verify-2fa。本项目 internal/api/client.go:77 使用 POST /api/v1/admin/login，普通登录路径兼容，但没有完成 2FA challenge 流程。
- 订单：官方 internal/modules/order/transport/http/routes.go:5-13 注册 GET /orders、GET /orders/:id、PATCH /orders/:id。本项目 internal/api/client.go:241、310、317 已使用对应路径。
- 履约：官方 internal/modules/fulfillment/transport/http/routes.go:5-12 注册 POST /fulfillments 和 GET /orders/:id/fulfillment/download。本项目只使用创建履约。
- 商品：官方 internal/modules/catalog/product/transport/http/routes.go:14-28 注册 GET /products、GET /products/:id 以及写接口。本项目只读商品列表。
- 卡密：官方 internal/modules/cardsecret/transport/http/routes.go:5-20 注册批量录入、CSV 导入、列表、更新、批量状态、批量删除、导出、统计和批次接口。本项目只使用批量录入。
- 仪表盘：官方 internal/modules/dashboard/transport/http/routes.go:5-9 注册 overview、trends、rankings、inventory-alerts。本项目已使用 overview 和 inventory-alerts。

结论：现有 Bot 主流程的路径兼容性较好；新增能力主要是 2FA、履约下载、卡密查询/导出和更完整的 DTO。

### 2. 商品、SKU 与库存模型

官方商品 presenter 位于 internal/modules/catalog/product/transport/presenter/product.go:15-56，除基础字段外，还包含 seo_meta、images、tags、purchase_type、最小/最大购买数量、stock_display_mode、stock_display、库存范围、stock_quantity_hidden、stock_status、is_sold_out、支付渠道、促销和会员价等。

官方 SKU presenter 位于同文件 :97-118，包含 id、sku_code、spec_values、price_amount、手动/自动库存、upstream_stock、stock_status、库存展示范围/隐藏字段、is_sold_out、is_active、促销价和会员价。

官方领域模型还确认：Product 具有 wholesale_prices、images、tags、购买数量限制、库存展示模式、is_affiliate_enabled、is_mapped、sort_order、is_active 等字段；ProductSKU 的默认编码仍为 DEFAULT，规格字段为 spec_values，库存包括 manual/auto/upstream 多组字段。证据：internal/modules/catalog/product/domain/product.go:12-52、domain/sku.go:10-38。

本项目 internal/model/model.go:137-182 只映射基础商品、SKU、价格、库存、上下游库存、启用状态和规格。对当前 /cards、/stock、商品选择流程足够，但还不能读取上游的售罄、隐藏库存、范围库存、购买限制、促销价和会员价语义。

类型风险：官方 presenter 的 auto_stock_available 使用 int64，本项目 Product/SKU 使用 int。普通库存通常无影响，但跨平台或极大库存值存在溢出风险。

SKU 关键契约没有改变：使用 sku_id 标识 SKU，使用 sku_snapshot.spec_values 展示下单时规格，sku_code 作为回退；DEFAULT 应显示为“默认规格”。本项目 internal/model/model.go:270-335 已实现该规则，handler 也已按商品 + SKU 分组，避免不同 SKU 合并。

### 3. 订单、订单项、子订单与履约

官方订单领域模型 internal/modules/order/domain/order.go:10-50 包含 parent_id、user_id、guest_email、guest_locale、状态、币种、原始/折扣/支付/退款金额、支付/取消时间、items、单独的 fulfillment 关联和 children 子订单。

官方订单详情 presenter internal/modules/order/transport/presenter/order.go:64-89 明确返回 fulfillment 和递归 children，并包含 allowed_payment_channel_ids、退款记录等。订单项 presenter :157-204 使用 title、sku_snapshot、tags、quantity，并返回 original_unit_price、unit_price、original_total_price、total_price、各类折扣、promotion_name、fulfillment_type、人工表单快照和 instructions。

本项目 internal/model/model.go:52-119 已有 children、sku_id、sku_snapshot、主要金额、状态、时间和履约请求/响应，但缺少 parent_id、订单返回中的 fulfillment 关联、原始价格、批发折扣、promotion ID/name、allowed payment channel IDs、refund records 等。当前 Bot 的待发货和母子订单流程仍可工作；若后续展示完整详情，应补齐这些 DTO 字段。

官方履约创建请求仍是 order_id、payload、delivery_data；证据：internal/modules/fulfillment/transport/http/admin_handler.go:58-100，与本项目请求结构兼容。官方下载处理会先取主订单 fulfillment，再拼接子订单 fulfillment；证据：同文件 :103-136。

提交记录显示履约校验在收紧：f57247b（2026-08-27，增强订单履约校验）以及 47cc620（2026-09-02，采购单归属和上游订单号校验）。没有改变 Bot 已用路径，但不要假设任意 order_id/payload 都会被接受。

### 4. 卡密与 SKU 绑定

官方批量录入请求 internal/modules/cardsecret/transport/http/admin_handler.go:42-56 包含 product_id、sku_id、secrets、batch_no、note，并新增可选 deduplicate；响应 :157-161 返回 created、batch_id、batch_no。

本项目 internal/model/model.go:121-135 已映射前五项，没有 deduplicate。若需要明确控制重复卡密行为，后续应增加该字段，且不能只依据 created 数量判断所有输入都入库。

官方卡密列表 admin_handler.go:229-291 支持 product_id、sku_id、batch_id、status、secret、batch_no 和 page/page_size，并返回标准分页 envelope。官方还提供 CSV 导入、状态更新、批量状态、批量删除、导出、可用卡密导出、统计和批次查询。

对当前 Telegram 集成最重要的是：/cards 按 SKU 选择并发送 product_id + sku_id + secrets 符合官方当前契约。本项目 handler 已保存 SKU 列表并处理 cards_sku 回调，模型请求也已包含 sku_id。

### 5. 认证与 JWT/2FA

官方管理员登录请求 internal/modules/identity/adminauth/transport/http/admin_login_handler.go:82-87 至少有 username、password，并允许 captcha_payload。成功且未启用 2FA 时 :141-150 返回 requires_totp=false、token、user.id、user.username、expires_at。

启用 2FA 时 :131-138 返回 requires_totp=true、challenge_token、challenge_expires_at，不返回可直接使用的 JWT；随后调用 POST /api/v1/admin/login/verify-2fa。验证请求字段在 internal/modules/identity/adminauth/transport/http/admin_2fa_handler.go:290-291 为 challenge_token 和验证码相关字段。

本项目 LoginResponseData 只有 requires_totp、token、expires_at、user；internal/api/client.go:108-110 遇到 RequiresTOTP 会直接报错“Bot 暂不支持”。这是当前最明确的认证集成缺口。

普通请求仍使用 Authorization: Bearer <token>，本项目设置于 internal/api/client.go:152-157。由于官方还使用 RBAC，登录成功不等于该账号拥有所有 Admin 权限。

### 6. 分页与错误结构

官方统一响应结构 internal/platform/http/response/response.go:13-26 为普通 {status_code, msg, data}，分页响应再增加 pagination；分页字段为 page、page_size、total、total_page。

官方成功响应 :67-83 固定 status_code=0、msg=success；业务错误 :86-103 通常仍使用 HTTP 200，但 envelope 的 status_code 非零。基础设施还提供 ErrorWithHTTPStatus，允许真实 HTTP 状态码与同一业务 envelope 并存；错误 data 在有 request ID 时可能携带 request_id。

本项目 internal/model/model.go:9-29 与 internal/api/client.go:170-218 对普通和分页 envelope 的解码一致，并正确检查 status_code。当前缺口是错误解析只保留 status_code/msg，不保留真实 HTTP 状态和 data.request_id，诊断 401/403/500 时信息不足。

官方另有 ChannelResponse，额外字段为 error_code 和 request_id；它属于渠道 API，不应直接套用于当前 Admin API。

### 7. 提交记录信号

已读取的相关官方提交包括：3533261（2026-07-22，整理 API bounded modules）、1393ce7（2026-07-22，完成管理员认证模块垂直迁移）、4fb702f（2026-07-22，完成商品模块垂直迁移）、322d513（2026-07-22，完成卡密模块垂直迁移）、99978c4（2026-07-22，完成订单模块垂直迁移）、f57247b（2026-08-27，增强订单履约校验）、47cc620（2026-09-02，加强采购单与认证连接、上游订单号归属校验）、f06035c（2026-09-03，当前 HEAD，主要是订单详情前端展示顺序、子订单履约和移动端布局）。已读取内容没有显示本项目所用 Admin API 路径被删除或重命名。

提交浏览：<https://github.com/dujiao-next/dujiao-next/commits/main>。

## Files found

### 本项目

- internal/api/client.go：登录、JWT、统一 envelope、分页解包，以及订单/履约/卡密/商品/仪表盘调用。
- internal/model/model.go：响应、分页、认证、订单、订单项、履约、卡密、商品、SKU、库存告警模型和展示辅助函数。
- internal/handler/handler.go：/orders、/cards、/fulfill、/pfulfill、/stock 流程、回调和 SKU 展示/分组。
- internal/model/model_test.go、internal/api/client_test.go、internal/handler/handler_test.go：模型、分页、SKU 展示、HTTP 查询和流程测试。
- .trellis/spec/backend/index.md、.trellis/spec/backend/error-handling.md、.trellis/spec/backend/quality-guidelines.md：本项目 API 集成边界、错误处理、分页、JSON tag、SKU 身份和测试约定。

### 官方上游仓库

- internal/app/httpserver/routes_admin.go：Admin 路由、JWT/RBAC 保护和模块注册。
- internal/platform/http/response/response.go：成功、分页、业务错误、HTTP 状态错误和渠道响应。
- internal/modules/identity/adminauth/transport/http/{routes.go,admin_login_handler.go,admin_2fa_handler.go}：管理员登录和 2FA。
- internal/modules/catalog/product/{transport/http/routes.go,transport/presenter/product.go,domain/product.go,domain/sku.go}：商品/SKU 路由和模型。
- internal/modules/order/{transport/http/routes.go,transport/presenter/order.go,domain/order.go,domain/order_item.go}：订单、子订单、订单项快照。
- internal/modules/fulfillment/transport/http/{routes.go,admin_handler.go}：履约创建和下载。
- internal/modules/cardsecret/transport/http/{routes.go,admin_handler.go}：卡密录入、SKU 过滤和批次管理。
- internal/modules/dashboard/transport/http/routes.go：仪表盘和库存告警。

## Related specs

- .trellis/spec/backend/index.md
- .trellis/spec/backend/error-handling.md
- .trellis/spec/backend/quality-guidelines.md
- .trellis/spec/guides/cross-layer-thinking-guide.md

## Recommended integration watchlist

1. 高优先级：支持 2FA，区分 JWT 成功和 challenge 成功，增加 challenge_token、challenge_expires_at 以及 /login/verify-2fa 调用。
2. 高优先级：补齐订单 DTO，至少考虑 parent_id、fulfillment、original_unit_price、original_total_price、wholesale_discount_amount、promotion_id/name。
3. 中优先级：金额继续按字符串处理；评估库存字段从 int 改为 int64。
4. 中优先级：卡密批量录入增加可选 deduplicate；需要时增加卡密列表、批次和统计 API。
5. 低优先级：错误对象保留真实 HTTP 状态和 request_id，方便定位认证和服务端错误。
6. 低优先级：如 Bot 要复刻管理前端库存语义，再补充 stock_status、is_sold_out、stock_display* 等字段。

## Caveats / Not Found

- 本次按要求停止继续搜索；结论只基于截至 2026-09-07 已读取的官方 GitHub main HEAD、相关提交记录、官方源码文件和本项目当前工作区文件，没有继续拉取后续提交或运行远程实例。
- 没有发现官方为本项目专用的稳定 OpenAPI/Swagger 版本清单；契约证据来自官方路由、源码 JSON tag、presenter/domain 和提交记录。
- 没有对本项目最初接入时的上游 commit 做完整 git bisect，因此“变化”主要表示当前 main 相对本项目客户端模型的兼容性差异和新增能力，不等同于每个字段都能归因到单个提交。
- 官方部分 handler 返回领域对象或包装对象，真实 JSON 仍受 jsonmap.JSON、money.Amount、关联预加载和中间件影响；本文列出的是已确认的字段/路由契约，不替代真实实例回归。
- 未运行本项目测试或修改代码；本轮只写入指定 research 文件。
