#!/usr/bin/env bash
# E2E pipeline test runner.
#
# Прогоняет полный 8-stage pipeline через admin endpoints всех 7 микросервисов
# и проверяет, что mock-erp получил purchase orders.
#
# Использование:
#   bash tests/e2e/run.sh                     # full run (предполагает make e2e-up уже выполнен)
#   bash tests/e2e/run.sh --scale-small       # маленький seed (10 prod / 2 loc / 7 days / 3 sup)
#   bash tests/e2e/run.sh --skip-up           # не делать make e2e-up (compose уже запущен)
#   bash tests/e2e/run.sh --cleanup           # после прогона docker compose down -v
#
# Требования: docker, jq, curl, go (для tests/e2e/cmd/jwtgen).
set -euo pipefail

# Подгружаем .env чтобы SEED_DAYS и др. совпадали с docker-compose mock-erp.
[ -f .env ] && set -a && source .env && set +a

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT_DIR"

# ──────────────────────────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────────────────────────
log_info() { printf '\033[34m[..]\033[0m %s\n' "$*"; }
log_ok()   { printf '\033[32m[OK]\033[0m %s\n' "$*"; }
log_fail() { printf '\033[31m[FAIL]\033[0m %s\n' "$*"; exit 1; }
log_warn() { printf '\033[33m[WARN]\033[0m %s\n' "$*"; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || log_fail "missing required tool: $1"
}

require_cmd docker
require_cmd curl
require_cmd jq
require_cmd go

# ──────────────────────────────────────────────────────────────────────────────
# Flags
# ──────────────────────────────────────────────────────────────────────────────
SKIP_UP="false"
CLEANUP="false"
SCALE_SMALL="false"
for arg in "$@"; do
  case "$arg" in
    --skip-up)     SKIP_UP="true" ;;
    --cleanup)     CLEANUP="true" ;;
    --scale-small) SCALE_SMALL="true" ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \{0,1\}//' | head -20
      exit 0 ;;
    *) log_fail "unknown flag: $arg" ;;
  esac
done

if [[ "$SCALE_SMALL" == "true" ]]; then
  export SEED_PRODUCTS="${SEED_PRODUCTS:-10}"
  export SEED_LOCATIONS="${SEED_LOCATIONS:-2}"
  export SEED_DAYS="${SEED_DAYS:-7}"
  export SEED_SUPPLIERS="${SEED_SUPPLIERS:-3}"
  log_info "scale-small: SEED_PRODUCTS=$SEED_PRODUCTS LOCATIONS=$SEED_LOCATIONS DAYS=$SEED_DAYS SUPPLIERS=$SEED_SUPPLIERS"
fi

# ──────────────────────────────────────────────────────────────────────────────
# Config
# ──────────────────────────────────────────────────────────────────────────────
JWT_SECRET="${JWT_SECRET:-dev-secret-change-in-prod}"
PG_USER="${POSTGRES_USER:-adapter}"
PG_DB="${POSTGRES_DB:-source_adapter}"
MOCK_ERP_API_KEY="${MOCK_ERP_API_KEY:-test-api-key}"

# Endpoint hosts (per docker-compose ports).
SOURCE_ADAPTER="http://localhost:8080"
ETL="http://localhost:8081"
KPI="http://localhost:8083"
FORECAST="http://localhost:8084"
ORDERS="http://localhost:8086"
CHANNELS="http://localhost:8087"
MOCK_ERP="http://localhost:8090"

POLL_INTERVAL_SEC=5
POLL_TIMEOUT_SEC=600  # 10 min per stage

# psql exec helper. -t -A → пустая строка → берём count.
# stderr оставляем включённым — иначе ошибки SQL будут молчаливо проглочены.
pg_q() {
  local out
  if ! out=$(docker exec ezoo_pg psql -U "$PG_USER" -d "$PG_DB" -t -A -c "$1" 2>&1); then
    echo "pg_q failed for: $1" >&2
    echo "output: $out" >&2
    return 1
  fi
  if [[ "$out" == ERROR:* ]]; then
    echo "pg_q SQL error for: $1" >&2
    echo "$out" >&2
    return 1
  fi
  printf '%s' "$out" | tr -d '[:space:]'
}

# curl wrapper that fails on non-2xx.
api() {
  local method="$1" url="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS -X "$method" -H "Authorization: Bearer $ADMIN_JWT" \
      -H "Content-Type: application/json" -d "$body" "$url"
  else
    curl -fsS -X "$method" -H "Authorization: Bearer $ADMIN_JWT" "$url"
  fi
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 0: bring compose up (if requested) + wait healthy
# ──────────────────────────────────────────────────────────────────────────────
if [[ "$SKIP_UP" != "true" ]]; then
  log_info "Step 0: make e2e-up (build + start compose)"
  make e2e-up
fi

log_info "Step 0: waiting for all services healthy (timeout 5 min)..."
deadline=$(( $(date +%s) + 300 ))
while true; do
  starting_or_unhealthy=$(docker compose ps --format '{{.Service}} {{.Status}}' \
    | grep -cE 'starting|unhealthy' || true)
  total=$(docker compose ps --format '{{.Service}}' | wc -l | tr -d '[:space:]')
  if [[ "$starting_or_unhealthy" == "0" && "$total" -ge 9 ]]; then
    break
  fi
  if [[ $(date +%s) -gt $deadline ]]; then
    docker compose ps
    log_fail "services did not become healthy within 5 min"
  fi
  sleep 3
done
log_ok "Step 0: all $total services healthy"

# ──────────────────────────────────────────────────────────────────────────────
# Step 0.5: generate admin JWT
# ──────────────────────────────────────────────────────────────────────────────
log_info "Step 0.5: generate admin-cli JWT (HS256)"
ADMIN_JWT=$(go run ./tests/e2e/cmd/jwtgen \
  -role admin-cli -secret "$JWT_SECRET" -ttl 2h -sub e2e-runner)
[[ -n "$ADMIN_JWT" ]] || log_fail "failed to generate JWT"
log_ok "Step 0.5: JWT generated (issuer=admin-cli)"

# ──────────────────────────────────────────────────────────────────────────────
# Step 0.6: seed mock-erp (on-demand seeder API)
# Master bootstrap (idempotent) + N days of facts. SEED_DAYS из env (по умолчанию 90).
# ──────────────────────────────────────────────────────────────────────────────
SEED_DAYS_RUN="${SEED_DAYS:-90}"
ERP_API_KEY_RUN="${MOCK_ERP_API_KEY:-test-api-key}"
log_info "Step 0.6: mock-erp seed (initial + $SEED_DAYS_RUN days)"

INITIAL_RESP=$(curl -fsS -X POST -H "X-API-Key: $ERP_API_KEY_RUN" \
  "$MOCK_ERP/admin/seed/initial" || true)
[[ -n "$INITIAL_RESP" ]] || log_fail "Step 0.6: seed/initial failed"
log_ok "Step 0.6a: initial seed done — $(echo "$INITIAL_RESP" | jq -r '"products in master: ok, snapshots=" + (.total_stock_snapshots|tostring)')"

DAYS_RESP=$(curl -fsS -X POST --max-time 1800 -H "X-API-Key: $ERP_API_KEY_RUN" \
  "$MOCK_ERP/admin/seed/days?count=$SEED_DAYS_RUN" || true)
[[ -n "$DAYS_RESP" ]] || log_fail "Step 0.6: seed/days?count=$SEED_DAYS_RUN failed"
log_ok "Step 0.6b: $(echo "$DAYS_RESP" | jq -r '"days=" + (.days|tostring) + " receipts=" + (.receipts_added|tostring) + " movements=" + (.movements_added|tostring)')"

PIPELINE_START=$(date +%s)

# ──────────────────────────────────────────────────────────────────────────────
# Stage 1: source-adapter — POST /admin/loads (async)
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 1/8: source-adapter — POST /admin/loads"
S1_START=$(date +%s)

# Запоминаем последнюю существующую load_id (чтобы потом найти "новую").
PREV_LOAD_ID=$(pg_q "SELECT COALESCE(load_id::text, '') FROM loads ORDER BY started_at DESC LIMIT 1")

# POST /admin/loads возвращает 202 Accepted с load_id=00000000... (uuid.Nil),
# потому что фактическая запись loads создаётся в фоне scheduler-ом.
# Поэтому load_id вычитываем из БД (последний != PREV_LOAD_ID).
LOAD_RESP=$(api POST "$SOURCE_ADAPTER/admin/loads" '{}')
ACCEPTED=$(echo "$LOAD_RESP" | jq -r '.status // empty')
[[ "$ACCEPTED" == "accepted" ]] || log_fail "Stage 1: POST /admin/loads not accepted: $LOAD_RESP"

# Дожидаемся появления новой записи в loads.
deadline=$(( $(date +%s) + 60 ))
LOAD_ID=""
while true; do
  LATEST=$(pg_q "SELECT COALESCE(load_id::text, '') FROM loads ORDER BY started_at DESC LIMIT 1")
  if [[ -n "$LATEST" && "$LATEST" != "$PREV_LOAD_ID" ]]; then
    LOAD_ID="$LATEST"
    break
  fi
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 1: no new load row appeared in 60s"
  fi
  sleep 2
done
log_info "Stage 1: tracking load_id=$LOAD_ID"

# Poll until status=committed.
deadline=$(( $(date +%s) + POLL_TIMEOUT_SEC ))
while true; do
  STATUS=$(api GET "$SOURCE_ADAPTER/admin/loads/$LOAD_ID" | jq -r '.status')
  case "$STATUS" in
    committed)        break ;;
    failed|aborted)   log_fail "Stage 1: load $LOAD_ID status=$STATUS" ;;
    running|pending)  : ;;
    *)                log_fail "Stage 1: unexpected status: $STATUS" ;;
  esac
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 1: timeout waiting for load $LOAD_ID (last status=$STATUS)"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

PRODUCTS_COUNT=$(pg_q "SELECT count(*) FROM products")
RECEIPT_COUNT=$(pg_q "SELECT count(*) FROM receipt_line")
LOCSTOCK_COUNT=$(pg_q "SELECT count(*) FROM location_stock_snapshot")
SUPPLIERS_COUNT=$(pg_q "SELECT count(*) FROM supplier")
S1_DUR=$(( $(date +%s) - S1_START ))
log_ok "Stage 1: load=$LOAD_ID committed in ${S1_DUR}s — products=$PRODUCTS_COUNT receipt_line=$RECEIPT_COUNT location_stock_snapshot=$LOCSTOCK_COUNT supplier=$SUPPLIERS_COUNT"
[[ "$PRODUCTS_COUNT" -ge 1 ]]  || log_fail "Stage 1: products count too low ($PRODUCTS_COUNT)"
[[ "$SUPPLIERS_COUNT" -ge 1 ]] || log_fail "Stage 1: supplier count too low ($SUPPLIERS_COUNT)"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 2: etl — POST /api/v1/admin/etl-runs (async)
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 2/8: etl — POST /api/v1/admin/etl-runs"
S2_START=$(date +%s)

ETL_RESP=$(api POST "$ETL/api/v1/admin/etl-runs" '{"kind":"full","trigger":"manual"}')
ETL_ID=$(echo "$ETL_RESP" | jq -r '.id // empty')
[[ -n "$ETL_ID" && "$ETL_ID" != "null" ]] || log_fail "Stage 2: no id in response: $ETL_RESP"

deadline=$(( $(date +%s) + POLL_TIMEOUT_SEC ))
while true; do
  STATUS=$(api GET "$ETL/api/v1/admin/etl-runs/$ETL_ID" | jq -r '.status')
  case "$STATUS" in
    committed)       break ;;
    failed|aborted)  log_fail "Stage 2: etl-run $ETL_ID status=$STATUS" ;;
    running|pending) : ;;
    *)               log_fail "Stage 2: unexpected status: $STATUS" ;;
  esac
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 2: timeout waiting for etl-run $ETL_ID (last status=$STATUS)"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

DEMAND_COUNT=$(pg_q "SELECT count(*) FROM marts.mart_demand_history")
CALC_COUNT=$(pg_q "SELECT count(*) FROM marts.mart_calculation_input")
S2_DUR=$(( $(date +%s) - S2_START ))
log_ok "Stage 2: etl-run=$ETL_ID committed in ${S2_DUR}s — mart_demand_history=$DEMAND_COUNT mart_calculation_input=$CALC_COUNT"
[[ "$CALC_COUNT" -ge 1 ]] || log_fail "Stage 2: mart_calculation_input is empty"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 3: kpi — POST /v1/kpi/snapshots/refresh (async, sync trigger fire-and-forget)
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 3/8: kpi — POST /v1/kpi/snapshots/refresh"
S3_START=$(date +%s)

KPI_RESP=$(api POST "$KPI/v1/kpi/snapshots/refresh" '{}')
KPI_RUN_ID=$(echo "$KPI_RESP" | jq -r '.run_id // empty')
KPI_STARTED=$(echo "$KPI_RESP" | jq -r '.started')
[[ "$KPI_STARTED" == "true" ]] || log_warn "Stage 3: KPI started=$KPI_STARTED (run_id=$KPI_RUN_ID) — already running?"

# KPI write to kpi.kpi_snapshots in background; poll DB count growth.
deadline=$(( $(date +%s) + POLL_TIMEOUT_SEC ))
while true; do
  KPI_COUNT=$(pg_q "SELECT count(*) FROM kpi.kpi_snapshots")
  if [[ "$KPI_COUNT" -ge 1 ]]; then
    sleep 2  # дать дописать
    break
  fi
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 3: timeout waiting for kpi_snapshots rows"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

OSA_COUNT=$(pg_q "SELECT count(*) FROM kpi.kpi_snapshots WHERE kpi_name='osa'")
OTIF_COUNT=$(pg_q "SELECT count(*) FROM kpi.kpi_snapshots WHERE kpi_name='otif'")
STOCK_COUNT=$(pg_q "SELECT count(*) FROM kpi.kpi_snapshots WHERE kpi_name='stock_days'")
S3_DUR=$(( $(date +%s) - S3_START ))
log_ok "Stage 3: kpi run=$KPI_RUN_ID in ${S3_DUR}s — osa=$OSA_COUNT otif=$OTIF_COUNT stock_days=$STOCK_COUNT"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 4: forecast — POST /v1/forecast/runs/refresh (async, has GetRun for polling)
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 4/8: forecast — POST /v1/forecast/runs/refresh"
S4_START=$(date +%s)

FORECAST_RESP=$(api POST "$FORECAST/v1/forecast/runs/refresh" '{"horizon_days":60}')
FORECAST_RUN_ID=$(echo "$FORECAST_RESP" | jq -r '.run_id // empty')
[[ -n "$FORECAST_RUN_ID" && "$FORECAST_RUN_ID" != "null" ]] || log_fail "Stage 4: no run_id: $FORECAST_RESP"

deadline=$(( $(date +%s) + POLL_TIMEOUT_SEC ))
while true; do
  RUN_JSON=$(api GET "$FORECAST/v1/forecast/runs/$FORECAST_RUN_ID")
  STATUS=$(echo "$RUN_JSON" | jq -r '.status')
  case "$STATUS" in
    committed)        break ;;
    failed|aborted)   log_fail "Stage 4: forecast run $FORECAST_RUN_ID status=$STATUS" ;;
    running|pending)  : ;;
    *)                log_fail "Stage 4: unexpected status: $STATUS (resp=$RUN_JSON)" ;;
  esac
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 4: timeout waiting for forecast run $FORECAST_RUN_ID (last status=$STATUS)"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

FCAST_COUNT=$(pg_q "SELECT count(*) FROM forecast.forecasts")
PLANS_DRAFT_COUNT=$(pg_q "SELECT count(*) FROM forecast.replenishment_plans WHERE status='draft'")
S4_DUR=$(( $(date +%s) - S4_START ))
log_ok "Stage 4: forecast run=$FORECAST_RUN_ID committed in ${S4_DUR}s — forecasts=$FCAST_COUNT draft_plans=$PLANS_DRAFT_COUNT"
[[ "$PLANS_DRAFT_COUNT" -ge 1 ]] || log_fail "Stage 4: no draft replenishment_plans created"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 5: approve all draft plans
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 5/8: approve all draft replenishment plans"
S5_START=$(date +%s)

# List plans with status=draft. API supports cursor-based pagination; собираем все.
APPROVED_COUNT=0
FAILED_APPROVE=0
CURSOR=""
while true; do
  if [[ -n "$CURSOR" ]]; then
    LIST_URL="$FORECAST/v1/replenishment/plans?status=draft&cursor=$CURSOR"
  else
    LIST_URL="$FORECAST/v1/replenishment/plans?status=draft"
  fi
  PLANS_JSON=$(api GET "$LIST_URL")
  IDS=$(echo "$PLANS_JSON" | jq -r '.items[].id')
  if [[ -z "$IDS" ]]; then
    break
  fi
  for plan_id in $IDS; do
    if api POST "$FORECAST/v1/replenishment/plans/$plan_id/approve" '{"approved_by":"e2e-runner"}' >/dev/null 2>&1; then
      APPROVED_COUNT=$(( APPROVED_COUNT + 1 ))
    else
      FAILED_APPROVE=$(( FAILED_APPROVE + 1 ))
    fi
  done
  CURSOR=$(echo "$PLANS_JSON" | jq -r '.next_cursor // empty')
  [[ -z "$CURSOR" || "$CURSOR" == "null" ]] && break
done

PLANS_APPROVED_DB=$(pg_q "SELECT count(*) FROM forecast.replenishment_plans WHERE status='approved'")
S5_DUR=$(( $(date +%s) - S5_START ))
log_ok "Stage 5: approved=$APPROVED_COUNT failed=$FAILED_APPROVE in ${S5_DUR}s (DB approved=$PLANS_APPROVED_DB)"
[[ "$PLANS_APPROVED_DB" -ge 1 ]] || log_fail "Stage 5: no approved plans in DB"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 6: order-builder — POST /v1/orders/purchase-orders/build (async)
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 6/8: order-builder — POST /v1/orders/purchase-orders/build"
S6_START=$(date +%s)

BUILD_RESP=$(api POST "$ORDERS/v1/orders/purchase-orders/build" '{"max_plans":500}')
BUILD_RUN_ID=$(echo "$BUILD_RESP" | jq -r '.run_id // empty')
log_info "Stage 6: build run_id=$BUILD_RUN_ID started=$(echo "$BUILD_RESP" | jq -r '.started')"

# Poll DB: ждём появления PO в orders.purchase_orders со status=ready_to_send.
deadline=$(( $(date +%s) + POLL_TIMEOUT_SEC ))
while true; do
  PO_READY=$(pg_q "SELECT count(*) FROM orders.purchase_orders WHERE status='ready_to_send'")
  if [[ "$PO_READY" -ge 1 ]]; then
    sleep 2
    PO_READY_NEW=$(pg_q "SELECT count(*) FROM orders.purchase_orders WHERE status='ready_to_send'")
    if [[ "$PO_READY_NEW" == "$PO_READY" ]]; then
      break
    fi
  fi
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 6: timeout — purchase_orders.ready_to_send=$PO_READY"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

PO_TOTAL=$(pg_q "SELECT count(*) FROM orders.purchase_orders")
PO_READY=$(pg_q "SELECT count(*) FROM orders.purchase_orders WHERE status='ready_to_send'")
S6_DUR=$(( $(date +%s) - S6_START ))
log_ok "Stage 6: purchase_orders total=$PO_TOTAL ready_to_send=$PO_READY in ${S6_DUR}s"
[[ "$PO_READY" -ge 1 ]] || log_fail "Stage 6: no ready_to_send POs"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 7: channel-router — POST /v1/channels/send (async)
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 7/8: channel-router — POST /v1/channels/send"
S7_START=$(date +%s)

SEND_RESP=$(api POST "$CHANNELS/v1/channels/send" '{"max_pos":500}')
SEND_RUN_ID=$(echo "$SEND_RESP" | jq -r '.run_id // empty')
log_info "Stage 7: send run_id=$SEND_RUN_ID started=$(echo "$SEND_RESP" | jq -r '.started')"

# Wait for purchase_orders.status='sent' to grow / channels.send_attempts.status='accepted'.
deadline=$(( $(date +%s) + POLL_TIMEOUT_SEC ))
while true; do
  PO_SENT=$(pg_q "SELECT count(*) FROM orders.purchase_orders WHERE status='sent'")
  ACCEPTED=$(pg_q "SELECT count(*) FROM channels.send_attempts WHERE status='accepted'")
  if [[ "$PO_SENT" -ge 1 ]]; then
    sleep 2
    PO_SENT_NEW=$(pg_q "SELECT count(*) FROM orders.purchase_orders WHERE status='sent'")
    if [[ "$PO_SENT_NEW" == "$PO_SENT" ]]; then
      break
    fi
  fi
  if [[ $(date +%s) -gt $deadline ]]; then
    log_fail "Stage 7: timeout — purchase_orders.sent=$PO_SENT send_attempts.accepted=$ACCEPTED"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

PO_SENT=$(pg_q "SELECT count(*) FROM orders.purchase_orders WHERE status='sent'")
ATTEMPTS_TOTAL=$(pg_q "SELECT count(*) FROM channels.send_attempts")
ATTEMPTS_ACCEPTED=$(pg_q "SELECT count(*) FROM channels.send_attempts WHERE status='accepted'")
ATTEMPTS_FAILED=$(pg_q "SELECT count(*) FROM channels.send_attempts WHERE status='failed'")
S7_DUR=$(( $(date +%s) - S7_START ))
log_ok "Stage 7: PO sent=$PO_SENT send_attempts total=$ATTEMPTS_TOTAL accepted=$ATTEMPTS_ACCEPTED failed=$ATTEMPTS_FAILED in ${S7_DUR}s"
[[ "$PO_SENT" -ge 1 ]] || log_fail "Stage 7: no PO transitioned to sent"

# ──────────────────────────────────────────────────────────────────────────────
# Stage 8: verify mock-erp received POs
# ──────────────────────────────────────────────────────────────────────────────
log_info "Stage 8/8: verify mock-erp received purchase orders"
S8_START=$(date +%s)

RECEIVED_JSON=$(curl -fsS -H "X-API-Key: $MOCK_ERP_API_KEY" "$MOCK_ERP/api/v1/orders/received")
RECEIVED_COUNT=$(echo "$RECEIVED_JSON" | jq 'length')
RECEIVED_DISTINCT_PO=$(echo "$RECEIVED_JSON" | jq '[.[].po_number] | unique | length')

S8_DUR=$(( $(date +%s) - S8_START ))
log_ok "Stage 8: mock-erp received_orders=$RECEIVED_COUNT distinct_po_numbers=$RECEIVED_DISTINCT_PO in ${S8_DUR}s"
[[ "$RECEIVED_COUNT" -ge "$PO_SENT" ]] \
  || log_warn "Stage 8: received ($RECEIVED_COUNT) < sent ($PO_SENT) — возможно retry/timeout мог увеличить разрыв"
[[ "$RECEIVED_COUNT" -ge 1 ]] || log_fail "Stage 8: mock-erp got nothing"

# ──────────────────────────────────────────────────────────────────────────────
# Summary
# ──────────────────────────────────────────────────────────────────────────────
TOTAL_DUR=$(( $(date +%s) - PIPELINE_START ))
echo ""
echo "════════════════════════════════════════════════════════════════════════"
printf "  E2E Pipeline Test: 8/8 stages PASSED in %ds\n" "$TOTAL_DUR"
echo "════════════════════════════════════════════════════════════════════════"
echo "  Stage 1 source-adapter         ${S1_DUR}s   products=$PRODUCTS_COUNT receipt_line=$RECEIPT_COUNT"
echo "  Stage 2 etl                    ${S2_DUR}s   demand_history=$DEMAND_COUNT calc_input=$CALC_COUNT"
echo "  Stage 3 kpi                    ${S3_DUR}s   osa=$OSA_COUNT otif=$OTIF_COUNT stock_days=$STOCK_COUNT"
echo "  Stage 4 forecast               ${S4_DUR}s   forecasts=$FCAST_COUNT draft_plans=$PLANS_DRAFT_COUNT"
echo "  Stage 5 approve plans          ${S5_DUR}s   approved=$PLANS_APPROVED_DB"
echo "  Stage 6 order-builder          ${S6_DUR}s   PO ready_to_send=$PO_READY"
echo "  Stage 7 channel-router         ${S7_DUR}s   PO sent=$PO_SENT attempts accepted=$ATTEMPTS_ACCEPTED"
echo "  Stage 8 mock-erp verify        ${S8_DUR}s   received=$RECEIVED_COUNT distinct=$RECEIVED_DISTINCT_PO"
echo "════════════════════════════════════════════════════════════════════════"

if [[ "$CLEANUP" == "true" ]]; then
  log_info "Cleanup: docker compose down -v"
  docker compose down -v
fi

exit 0
