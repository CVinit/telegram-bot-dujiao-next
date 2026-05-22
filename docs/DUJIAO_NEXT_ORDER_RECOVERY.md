# Dujiao-Next Order Recovery Boundary

This note records what the current upstream dujiao-next source allows for
delivery recovery scenarios that matter to this Telegram admin bot.
The bot's successful-payment alert uses the Admin order list sorted by
`updated_at desc` and filters orders with a non-empty `paid_at`; dujiao-next
does not expose a direct `paid_at is not null` list filter in the checked source.

Source checked on 2026-05-22:

- Repository: <https://github.com/dujiao-next/dujiao-next>
- Commit: `c81ab92f946cafc8c2630c0f5f6e6fb17e0ce989`

## Delivered Card Secrets

The Admin API can update inventory records through
`PUT /api/v1/admin/card-secrets/:id`. Upstream
`CardSecretService.UpdateCardSecret` accepts a replacement `secret` value even
for an existing card-secret row, and card-secret status may be changed only
between `available`, `reserved`, and `used`.

That is not the same as editing what was delivered to an order.

Auto fulfillment copies selected inventory secret strings into a separate
`fulfillments.payload` snapshot when delivery is created. Admin fulfillment
download reads that payload snapshot from the order fulfillment record. The
current upstream router and `FulfillmentRepository` expose create/read flows,
but no update operation for an existing fulfillment payload.

Practical result:

- You can edit the inventory card-secret row.
- That edit does not rewrite already delivered order content.
- Changing an already delivered payload requires upstream support for a
  fulfillment-update recovery flow or direct database repair with an audit
  plan.

Relevant upstream files:

- `internal/http/handlers/admin/card_secret_admin.go`
- `internal/service/card_secret_service.go`
- `internal/service/fulfillment_service.go`
- `internal/models/fulfillment.go`
- `internal/repository/fulfillment_repository.go`

## Canceled Orders

Admin order status updates go through `PATCH /api/v1/admin/orders/:id` and
`OrderService.UpdateOrderStatus`. The upstream status transition table allows
forward transitions such as:

- `pending_payment -> paid | canceled`
- `paid -> fulfilling | delivered | refund-related statuses`
- `fulfilling -> delivered | refund-related statuses`

`canceled` has no outgoing transition. A normal Admin API request that attempts
to change a canceled order back to `paid`, `fulfilling`, or `delivered` is
rejected as an invalid status transition.

Practical result:

- If callback recovery happens while the order is still `pending_payment`, an
  admin can verify payment and move the order to `paid`; normal manual
  fulfillment can continue from there.
- After the order is already `canceled`, this bot should not expose a
  "revive to fulfilling" command because upstream normal-flow APIs reject it.
- A safe canceled-order recovery feature belongs upstream as an explicit
  audited operation that handles payment state, stock/card reservations,
  status side effects, notifications, and callbacks together.

Relevant upstream files:

- `internal/constants/constants.go`
- `internal/http/handlers/admin/order_admin.go`
- `internal/service/order_service.go`
- `internal/service/order_service_child.go`
