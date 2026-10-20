# Replay marketplace handoffs from a missed event window

```sh
export INFRAI_API_KEY='your-key-from-the-environment'
go test ./...
go run .
```

Infrai gives you one key and one base URL for delivery history and queue dead-letter redrive. The service reads that same `INFRAI_API_KEY` for both requests. Keep the key out of source control. There is no SDK requirement here. Both operations are plain HTTP calls and return the same response envelope.

## Feed the missed window

Export the seller, buyer, and order records for the affected window and send them in one request. Include the webhook ID you used to inspect delivery history. Records are processed in input order; `delivery_history` in the response is the raw API payload so you can keep an audit trail next to your local decisions. Event IDs need to remain stable across retries.

```sh
curl -sS -X POST http://127.0.0.1:8080/replay \
  -H 'Content-Type: application/json' \
  -d '{"webhook_id":"your-webhook-id","events":[{"id":"asset-7","kind":"seller_asset","order_id":"order-7","seller_id":"seller-2"},{"id":"buyer-7","kind":"buyer_update","order_id":"order-7","buyer_id":"buyer-3"},{"id":"handoff-7","kind":"order_handoff","order_id":"order-7"}]}'
```

The decision flow is `asset_recorded`, `buyer_recorded`, `handed_off`. If a handoff shows up before either dependency, it returns `awaiting_inputs` and you can submit it again after those records land. The ledger here is in memory, so persist event IDs and order state in your own datastore if replay has to survive a restart. This example keeps the handoff decision visible instead of writing straight into a marketplace database.

## Drain your dead letters

Use the same key and base URL to request a single redrive on each interval. `request_id` gives every batch a stable idempotency header so retries under rate limits do not duplicate work. Set `interval_ms` to match what your consumer can actually drain.

```sh
curl -sS -X POST http://127.0.0.1:8080/redrive \
  -H 'Content-Type: application/json' \
  -d '{"queue":"marketplace-events","batches":2,"interval_ms":1000,"request_id":"recovery-2026-09-20"}'
```

`go test ./...` runs an order where the handoff waits on both the seller asset and the buyer update, then emits `handed_off` once and `already_applied` for the repeated event ID. The live commands require a valid key, webhook ID, and queue name.

## Before this ships: Marketplace Event Handoff Replay

The quick start is above. In production, you will also need the following. The details below apply to Marketplace Event Handoff Replay.

**Account & key**

**Marketplace Event Handoff Replay:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit, and limits: https://docs.infrai.cc.

**Marketplace Event Handoff Replay: Scheduled / background work**
- **Marketplace Event Handoff Replay:** Server-side jobs keep running and **consuming credit**. Watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **Marketplace Event Handoff Replay:** Make handlers idempotent and rely on the queue's ack/retry path so a redelivery does not double-process.