# 调查 dujiao-next 上游 API 变化并完成机器人适配

## Goal

针对 dujiao-next 当前 `v1.4.7` / `main` 的 Admin API 与订单履约语义变化，修复 Telegram 管理机器人可能漏单、误报发货成功和模型契约缺失的问题，并用 HTTP fixture 和单元测试锁定兼容性。

## What I already know

- 当前机器人通过 `internal/api/client.go` 调用 Admin API，核心路径仍存在。
- 上游登录开启 2FA 时返回 challenge，不直接返回 JWT；本项目当前会直接失败。
- 上游创建人工 fulfillment 后会参与订单状态流转；本项目随后无条件 PATCH `completed`，且忽略错误。
- 当前待处理订单只查询 `fulfilling`、`partially_delivered`，每个状态固定只取第一页。
- 上游卡密批量导入新增可选 `deduplicate`；商品/SKU、订单和库存响应增加字段。
- 当前研究证据保存在 `research/` 下的三个文件中；本次先完成不依赖外部实例的代码和 fixture 验证。

## Requirements

- 保持所有 dujiao-next HTTP 调用集中在 `internal/api.Client`。
- 保留现有 Telegram 命令和 SKU 展示/分组行为。
- 待处理订单查询必须覆盖上游实际可发货状态，并完整扫描分页。
- 发货成功判定必须基于 API 调用结果，不得吞掉状态更新错误。
- 补充上游关键响应/请求字段，但不引入上游 Go 包。
- 为登录、2FA challenge、分页、fulfillment、状态更新、卡密请求和新增字段增加 focused tests。
- 本阶段不接入需要真实 TOTP 秘钥或真实 dujiao-next 实例的交互式 2FA；先保留清晰的 challenge 错误信息和可扩展模型。

## Acceptance Criteria

- [ ] `ListFulfillingOrders` 按确定的待发货状态逐一完整分页，不超过固定第一页。
- [ ] fulfillment 创建后的状态处理与当前上游契约一致；状态更新失败会返回/展示错误，不再静默成功。
- [ ] 登录响应可区分直接 JWT 和 2FA challenge，challenge 信息不会被误当作成功 token。
- [ ] `deduplicate` 请求字段可按需序列化，订单、退款、库存状态扩展字段可解码。
- [ ] 相关 API contract tests 覆盖成功、业务错误和关键请求参数。
- [ ] `gofmt -l cmd internal`、`go test ./...`、`go vet ./...`、`git diff --check` 通过。
- [ ] 不修改用户已有的 `AGENTS.md` 未跟踪文件。

## Definition of Done

- 代码修改保持必要范围，API/model/handler 分层不被打破。
- 测试和文档反映新的上游契约。
- 完成质量检查，并记录真实命令结果和剩余风险。

## Out of Scope

- 不重写 Telegram 框架或 HTTP 客户端。
- 不实现需要保存 TOTP 秘钥的完整交互式 2FA 流程。
- 不接入 Dashboard trends/rankings、卡密查询/导出等未被当前命令使用的新功能。
- 不处理上游内部目录重构本身。

## Technical Notes

- 本地集成面：`research/local-integration-surface.md`。
- 上游契约：`research/upstream-api-contract.md`。
- 上游历史与风险：`research/upstream-history-and-risk.md`。
- 关键代码：`internal/api/client.go`、`internal/model/model.go`、`internal/handler/handler.go` 及对应测试。
- 上游当前参考：`dujiao-next/dujiao-next`，HEAD `f06035ca359b0e4e99908fa6ae232af85e923d7f`。
