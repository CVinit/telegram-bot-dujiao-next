# fix: 兼容 payment_channel_ids 字符串格式

## Goal

修复上游商品接口将 `payment_channel_ids` 返回为字符串时的反序列化失败，恢复 `/stock` 库存查询，同时保持现有 `[]uint` 使用方式。

## Requirements

* 支持 `payment_channel_ids` 为正常数字数组、JSON 字符串、逗号分隔字符串、空字符串和 `null`。
* 保持 `model.Product.PaymentChannelIDs` 的调用方类型为 `[]uint`。
* 增加模型层单元测试，覆盖兼容格式和非法输入。
* 只修改必要的模型与测试代码，不调整库存业务逻辑。

## Acceptance Criteria

* [ ] 商品 JSON 中 `payment_channel_ids: "[1,2]"` 可成功反序列化。
* [ ] 商品 JSON 中 `payment_channel_ids: [1,2]` 可成功反序列化。
* [ ] 空值和 `null` 不导致商品列表解析失败。
* [ ] 非法值返回清晰的反序列化错误。
* [ ] `go test ./...` 和 `go vet ./...` 通过。

## Definition of Done

* 完成最小补丁。
* 添加并运行聚焦单元测试。
* 完成格式化、测试和静态检查。

## Technical Approach

为 `[]uint` 增加自定义 JSON 兼容解析类型，或在 `Product` 层提供兼容的 `UnmarshalJSON`，确保字段对外仍暴露为 `[]uint`。优先选择影响范围最小、测试清晰的实现。

## Out of Scope

* 不修改上游 API。
* 不修改库存展示和分页逻辑。
* 不做无关重构。

## Technical Notes

* 受影响模型：`internal/model/model.go` 的 `Product.PaymentChannelIDs`。
* 入口：`internal/api/client.go` 的商品列表解析，错误最终由 `internal/handler/handler.go` 的 `/stock` 返回。
