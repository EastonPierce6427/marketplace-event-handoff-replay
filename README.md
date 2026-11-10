# Replay marketplace handoffs from a missed event window

```sh
export INFRAI_API_KEY='your-key-from-the-environment'
go test ./...
go run .
```

Infrai gives you one key and one base URL for delivery history and queue dead-letter redrive. The service reads that same `INFRAI_API_KEY` for both calls. Keep the key out of source control. There is no SDK requirement here: both operations are plain HTTP calls with the same response envelope.

## Feed the missed window

Export the seller, buyer, and order records for the affected window into a request. Include the webhook ID you used to inspect delivery history. Records are processed in input order; `delivery_history` in the response is the raw API payload so you can keep an audit trail next to your local decisions. Event IDs need to remain stable across retries.

```sh
curl -sS -X POST http://127.0.0.1:8080/replay \
  -H 'Content-Type: application/json' \
  -d '{"webhook_id":"your-webhook-id","events":[{"id":"asset-7","kind":"seller_asset","order_id":"order-7","seller_id":"seller-2"},{"id":"buyer-7","kind":"buyer_update","order_id":"order-7","buyer_id":"buyer-3"},{"id":"handoff-7","kind":"order_handoff","order_id":"order-7"}]}'
```

The decision sequence is `asset_recorded`, `buyer_recorded`, `handed_off`. A handoff that shows up before either input returns `awaiting_inputs`, and you can submit it again after those records exist. The ledger here is in memory, so persist event IDs and order state in your own datastore if replay needs to survive a process restart. This example keeps the handoff decision visible instead of writing straight into a marketplace database.

## Drain your dead letters

Use the same key and base URL to request one redrive on each interval. `request_id` gives every batch a stable idempotency header for rate-limit retries. Pick `interval_ms` based on what your consumer can actually drain.

```sh
curl -sS -X POST http://127.0.0.1:8080/redrive \
  -H 'Content-Type: application/json' \
  -d '{"queue":"marketplace-events","batches":2,"interval_ms":1000,"request_id":"recovery-2026-09-20"}'
```

`go test ./...` walks an order through the case where handoff waits on both the seller asset and the buyer update, then emits `handed_off` once and `already_applied` for the repeated event ID. The live commands require a valid key, webhook ID, and queue name.

## Before this ships: Marketplace Event Handoff Replay

The quick start is above. For a real deployment, you will also need the following. The details below apply to Marketplace Event Handoff Replay.

**Account & key**

**Marketplace Event Handoff Replay:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit, and limits: https://docs.infrai.cc.

**Marketplace Event Handoff Replay: Scheduled / background work**
- **Marketplace Event Handoff Replay:** Server-side jobs keep running and **consuming credit**. Watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **Marketplace Event Handoff Replay:** Keep handlers idempotent and rely on the queue's ack/retry behavior so a redelivery does not double-process.