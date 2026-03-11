#!/bin/bash
set -e

API="http://localhost:18080"
KEY="Authorization: Bearer changeme"
CT="Content-Type: application/json"

echo "========================================"
echo "  HookRelay End-to-End Test"
echo "========================================"
echo ""

# ---- 1. Health check ----
echo "1. Health check..."
curl -sf "$API/healthz" | python3 -m json.tool
echo ""

echo "   Readiness check..."
curl -sf "$API/readyz" | python3 -m json.tool
echo ""

# ---- 2. Create sources ----
echo "2. Creating sources..."

echo "   [source: test-app] (no verification)"
curl -sf -X POST "$API/api/v1/sources" \
  -H "$KEY" -H "$CT" \
  -d '{
    "name": "test-app",
    "verify_type": "none",
    "verify_config": {},
    "event_type_path": "type",
    "idempotency_path": "id",
    "description": "Test application"
  }' | python3 -m json.tool
echo ""

echo "   [source: secure-app] (bearer token)"
curl -sf -X POST "$API/api/v1/sources" \
  -H "$KEY" -H "$CT" \
  -d '{
    "name": "secure-app",
    "verify_type": "bearer_token",
    "verify_config": {"token": "secret-token-123"},
    "event_type_path": "event",
    "description": "Secure application with bearer token"
  }' | python3 -m json.tool
echo ""

# ---- 3. Create subscriptions ----
echo "3. Creating subscriptions..."

echo "   [sub: all events → receiver]"
curl -sf -X POST "$API/api/v1/subscriptions" \
  -H "$KEY" -H "$CT" \
  -d '{
    "name": "all-to-receiver",
    "source_id": 1,
    "event_filter": ["*"],
    "target_url": "http://localhost:9999/webhook",
    "signing_secret": "my-signing-secret",
    "max_retries": 3,
    "timeout_seconds": 10
  }' | python3 -m json.tool
echo ""

echo "   [sub: order events → receiver]"
curl -sf -X POST "$API/api/v1/subscriptions" \
  -H "$KEY" -H "$CT" \
  -d '{
    "name": "orders-only",
    "source_id": 1,
    "event_filter": ["order.*"],
    "target_url": "http://localhost:9999/webhook",
    "max_retries": 3
  }' | python3 -m json.tool
echo ""

echo "   [sub: fail endpoint → test retry]"
curl -sf -X POST "$API/api/v1/subscriptions" \
  -H "$KEY" -H "$CT" \
  -d '{
    "name": "fail-endpoint",
    "source_id": 1,
    "event_filter": ["fail.*"],
    "target_url": "http://localhost:9999/webhook/fail",
    "max_retries": 2
  }' | python3 -m json.tool
echo ""

# ---- 4. Send webhook events ----
echo "4. Sending webhook events..."
echo ""

echo "   [event: order.created] → should match sub 1 (all) + sub 2 (order.*)"
curl -sf -X POST "$API/ingest/test-app" \
  -H "$CT" \
  -d '{
    "id": "evt_001",
    "type": "order.created",
    "data": {"order_id": "ORD-123", "amount": 9900, "currency": "CNY"}
  }' | python3 -m json.tool
echo ""

echo "   [event: user.signup] → should match sub 1 (all) only"
curl -sf -X POST "$API/ingest/test-app" \
  -H "$CT" \
  -d '{
    "id": "evt_002",
    "type": "user.signup",
    "data": {"user_id": "USR-456", "email": "test@example.com"}
  }' | python3 -m json.tool
echo ""

echo "   [event: order.created duplicate] → should be deduped by id=evt_001"
curl -sf -X POST "$API/ingest/test-app" \
  -H "$CT" \
  -d '{
    "id": "evt_001",
    "type": "order.created",
    "data": {"order_id": "ORD-123", "amount": 9900}
  }' | python3 -m json.tool
echo ""

echo "   [event: fail.test] → should trigger retry on fail endpoint"
curl -sf -X POST "$API/ingest/test-app" \
  -H "$CT" \
  -d '{
    "id": "evt_003",
    "type": "fail.test",
    "data": {"message": "this delivery will fail"}
  }' | python3 -m json.tool
echo ""

echo "   [event: secure-app with bearer token]"
curl -sf -X POST "$API/ingest/secure-app" \
  -H "$CT" \
  -H "Authorization: Bearer secret-token-123" \
  -d '{
    "event": "payment.completed",
    "amount": 5000
  }' | python3 -m json.tool
echo ""

echo "   [event: secure-app with WRONG token] → should be rejected"
curl -s -X POST "$API/ingest/secure-app" \
  -H "$CT" \
  -H "Authorization: Bearer wrong-token" \
  -d '{
    "event": "payment.completed",
    "amount": 5000
  }' | python3 -m json.tool
echo ""

# ---- 5. Wait for deliveries ----
echo "5. Waiting 3 seconds for deliveries..."
sleep 3
echo ""

# ---- 6. Check results ----
echo "6. Checking results..."
echo ""

echo "   All events:"
curl -sf "$API/api/v1/events?page_size=10" \
  -H "$KEY" | python3 -m json.tool
echo ""

echo "   Event #1 deliveries:"
curl -sf "$API/api/v1/events/1/deliveries" \
  -H "$KEY" | python3 -m json.tool
echo ""

echo "   Dead letters:"
curl -sf "$API/api/v1/dead-letters" \
  -H "$KEY" | python3 -m json.tool
echo ""

echo "   Overview stats:"
curl -sf "$API/api/v1/stats/overview" \
  -H "$KEY" | python3 -m json.tool
echo ""

echo "========================================"
echo "  Test complete!"
echo "  Check test-receiver terminal for"
echo "  received webhook details."
echo "========================================"
