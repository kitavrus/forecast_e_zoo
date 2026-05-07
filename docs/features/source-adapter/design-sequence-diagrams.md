# Design Sequence Diagrams — source-adapter

Sequence-диаграммы (Mermaid) для каждого endpoint + cron-tick. Сгруппированы по endpoint-семьям.

---

## 0. Cron tick → daily-load (внутрипроцессный)

```mermaid
sequenceDiagram
  autonumber
  participant Cron as Scheduler<br/>(gocron)
  participant Loader
  participant Lock as PG advisory lock
  participant Reader as SourceReader
  participant ERP
  participant Val as Validator
  participant Repo as Repository
  participant Snap as SnapshotService
  participant Metrics as Prometheus

  Cron->>Loader: Trigger daily-load (02:00 Kyiv)
  Loader->>Lock: pg_try_advisory_lock(hash('daily-load'))
  alt lock taken
    Lock-->>Loader: false
    Loader->>Metrics: load_skipped_total++
    Loader-->>Cron: skip silently
  else lock acquired
    Lock-->>Loader: true
    Loader->>Repo: INSERT loads(id, status='running')
    loop по сущностям (master → facts)
      Loader->>Reader: ReadX(ctx, since)
      Reader->>ERP: HTTPS pull (cursor)
      ERP-->>Reader: batch DTO
      Reader-->>Loader: domain rows
      Loader->>Val: Validate(entity, rows)
      Val-->>Loader: severity per row
      alt severity = critical
        Loader->>Repo: INSERT reject_log
      else
        Loader->>Repo: UPSERT staging<br/>(load_id = newID)
      end
    end
    Loader->>Repo: SELECT diff for master_change_log
    Loader->>Repo: INSERT master_change_log
    Loader->>Loader: lines_failed/lines_total > 1%?
    alt quality OK
      Loader->>Snap: Flip(loadID)
      Snap->>Repo: BEGIN; UPDATE snapshot_pointer; COMMIT
      Snap-->>Loader: ok
      Loader->>Repo: UPDATE loads(status='committed')
      Loader->>Metrics: load_success_total++
    else quality failed
      Loader->>Repo: UPDATE loads(status='failed', reason)
      Loader->>Metrics: load_failed_total{reason="quality"}++
    end
    Loader->>Lock: pg_advisory_unlock
  end
```

## 1. GET /v1/healthz

```mermaid
sequenceDiagram
  autonumber
  participant Client
  participant Fiber as Fiber router
  participant H as handler/healthz
  participant Repo as Repository
  participant PG

  Client->>Fiber: GET /v1/healthz
  Note over Fiber: NO JWT (public)
  Fiber->>H: handle
  H->>Repo: PingDB(ctx)
  Repo->>PG: SELECT 1
  PG-->>Repo: 1
  Repo-->>H: ok
  H-->>Client: 200 {status:"ok", db:"up", version:"x.y.z", current_snapshot_id:"<uuid>|null"}
```

## 2. GET /v1/snapshots, GET /v1/snapshots/current

```mermaid
sequenceDiagram
  autonumber
  participant C as Client (X-Flow / IT)
  participant J as JWT mw
  participant H as handler/snapshots
  participant S as service
  participant R as repository
  participant PG

  C->>J: GET /v1/snapshots/current + Authorization: Bearer
  J-->>H: claims ok
  H->>S: GetCurrent()
  S->>R: SelectSnapshotPointer()
  R->>PG: SELECT current_load_id, previous_load_id, committed_at FROM snapshot_pointer
  PG-->>R: row
  R-->>S: SnapshotPointer
  S-->>H: dto
  H-->>C: 200 {current_load_id, previous_load_id, committed_at}

  C->>J: GET /v1/snapshots?limit=10
  J-->>H: ok
  H->>S: ListSnapshots(limit)
  S->>R: SelectLoads(status='committed', limit)
  R->>PG: SELECT id, started_at, finished_at, entities_summary FROM loads WHERE status='committed' ORDER BY finished_at DESC LIMIT $1
  PG-->>R: rows
  R-->>S: []Load
  S-->>H: dto
  H-->>C: 200 {snapshots:[...]}
```

## 3. GET /v1/{master_entity} (products, product_barcodes, category, location, supplier)

> Шаблон одинаков для всех master-сущностей. Дальше показан `products`.

```mermaid
sequenceDiagram
  autonumber
  participant C as X-Flow ETL
  participant J as JWT mw
  participant H as handler/products
  participant S as service.ListProducts
  participant Snap as SnapshotService
  participant R as repository
  participant PG

  C->>J: GET /v1/products?since=2026-05-01&cursor=PRD-100&limit=1000
  J-->>H: claims{role:"x-flow-etl"}
  H->>Snap: Current()
  alt no snapshot
    Snap-->>H: ErrSnapshotNotReady
    H-->>C: 503 {code:"snapshot_not_ready"} + Retry-After:60
  else
    Snap-->>H: load_id=L1
    H->>S: List(loadID=L1, since, cursor, limit)
    S->>R: SelectProducts(L1, since, cursor, limit)
    R->>PG: SELECT * FROM products WHERE load_id=$1 AND updated_at>=$2 AND product_id>$3 ORDER BY product_id LIMIT $4
    PG-->>R: rows stream
    R-->>S: rows
    S-->>H: rows
    H->>H: Set X-Snapshot-Id, X-Load-Id, ETag, Cache-Control
    H-->>C: 200 NDJSON stream
  end
```

## 4. GET /v1/store_assortment, GET /v1/store_assortment/lifecycle_events

```mermaid
sequenceDiagram
  autonumber
  participant C
  participant J as JWT mw
  participant H as handler/store_assortment
  participant S as service
  participant R as repository
  participant PG

  C->>J: GET /v1/store_assortment?state=active,phasing_out&location_id=STORE-005
  J-->>H: ok
  H->>S: ListAssortment({states:[...], locationID:"STORE-005"})
  S->>R: SelectAssortment(loadID, filters)
  R->>PG: SELECT * FROM store_assortment WHERE load_id=$1 AND lifecycle_state = ANY($2) AND location_id = $3
  PG-->>R: rows
  R-->>H: rows
  H-->>C: 200 NDJSON (X-Snapshot-Id, ETag)

  C->>J: GET /v1/store_assortment/lifecycle_events?since=2026-05-01
  J-->>H: ok
  H->>S: ListLifecycleEvents(since)
  S->>R: SelectLifecycleEvents(loadID, since)
  R->>PG: SELECT * FROM store_assortment_lifecycle_events WHERE load_id=$1 AND transition_at>=$2
  PG-->>R: rows
  R-->>H: rows
  H-->>C: 200 NDJSON
```

**Response payload (ADR-016 / Q-016) — `StoreAssortmentLifecycleEventResponse`:**

| Поле | Тип | Описание |
|---|---|---|
| `eventId` | UUID string | ID события lifecycle |
| `eventType` | enum | `started` \| `stopped` \| `promo_started` \| `promo_stopped` |
| `locationId` | string | ID точки (магазина/DC) |
| `productId` | string | ID товара |
| `effectiveAt` | timestamp | Когда событие вступило в силу |
| `reason` | string? | Например `out_of_stock`, `promo_id=PR123` |
| `promoId` | string? | Если `eventType` = `promo_*` |
| `priorState` | enum? | `active` \| `inactive` \| `promo` |
| `newState` | enum | `active` \| `inactive` \| `promo` |
| `sourceLoadId` | UUID string | ID загрузки, в которой обнаружено событие |
| `createdAt` | timestamp | Время записи в `master_change_log` адаптера |

JSON Schema: `additionalProperties: false`. Полное Go-определение — см.
[design-go-layers.md](design-go-layers.md) §3.1 `StoreAssortmentLifecycleEventResponse`.
```

## 5. GET /v1/master_change_log

```mermaid
sequenceDiagram
  autonumber
  participant C
  participant J
  participant H as handler/master_change_log
  participant S
  participant R
  participant PG

  C->>J: GET /v1/master_change_log?entity=products&since=2026-05-06&field=brand
  J-->>H: ok
  H->>S: ListChanges({entity, since, field})
  S->>R: SelectMasterChangeLog(filters)
  R->>PG: SELECT * FROM master_change_log WHERE entity=$1 AND changed_at>=$2 AND ($3='' OR field=$3) ORDER BY changed_at
  PG-->>R: events
  R-->>H: events
  H-->>C: 200 NDJSON {event_id, entity, entity_pk, field, old_value, new_value, changed_at, load_id}
```

## 6. GET /v1/supplier_stock_snapshot

```mermaid
sequenceDiagram
  autonumber
  participant C
  participant J
  participant H as handler/supplier_stock
  participant S
  participant R
  participant PG

  C->>J: GET /v1/supplier_stock_snapshot?supplier_id=SUP-RC-UA&since=2026-05-01
  J-->>H: ok
  H->>S: ListSupplierStock(filters)
  S->>R: SelectSupplierStockSnapshot(loadID, filters)
  R->>PG: SELECT * FROM supplier_stock_snapshot WHERE load_id=$1 AND ($2='' OR supplier_id=$2) AND snapshot_at >= $3
  PG-->>R: rows (может быть пусто — Q-009)
  R-->>H: rows
  H-->>C: 200 NDJSON (даже пустой)
```

## 7. GET /v1/supply_spec, /v1/promo, /v1/supply_plan, /v1/order_rule

```mermaid
sequenceDiagram
  autonumber
  participant C
  participant J
  participant H as handler/{entity}
  participant S as service
  participant R as repository
  participant PG

  C->>J: GET /v1/{entity}?since=...
  J-->>H: ok
  H->>S: List(filters)
  S->>R: Select{Entity}(loadID, filters)
  R->>PG: SELECT * FROM {entity} WHERE load_id=$1 AND updated_at>=$2 ORDER BY pk LIMIT $3
  PG-->>R: rows
  R-->>H: rows
  H-->>C: 200 NDJSON
```

## 8. POST /v1/exports

```mermaid
sequenceDiagram
  autonumber
  participant C
  participant J
  participant H as handler/exports
  participant S as service.CreateExport
  participant R as repository
  participant PG
  participant W as exports worker
  participant FS as Local FS

  C->>J: POST /v1/exports {entity, snapshot_id?, format:"parquet"}
  J-->>H: ok
  H->>S: Create(req)
  S->>R: InsertExport(id, status='queued', ...)
  R->>PG: INSERT INTO exports
  PG-->>R: ok
  R-->>S: id
  S-->>H: id
  H-->>C: 202 {export_id, status:"queued"}

  Note over W: периодический poller<br/>(каждые 5s)
  W->>R: ClaimNextExport()
  R->>PG: UPDATE exports SET status='running' WHERE id=(SELECT id FROM exports WHERE status='queued' LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING *
  PG-->>R: claimed row
  R-->>W: export
  W->>R: SelectEntityRowsForSnapshot(entity, snapshot_id)
  R->>PG: SELECT ... cursor batches
  PG-->>R: stream
  W->>FS: WriteParquet(/var/exports/{id}.parquet)
  FS-->>W: ok
  W->>R: UPDATE exports SET status='ready', size_bytes=...
```

## 9. GET /v1/exports/{id}

```mermaid
sequenceDiagram
  autonumber
  participant C
  participant J
  participant H as handler/exports
  participant R
  participant PG
  participant FS

  C->>J: GET /v1/exports/{id}
  J-->>H: ok
  H->>R: SelectExportByID(id)
  R->>PG: SELECT * FROM exports WHERE id=$1
  PG-->>R: row
  R-->>H: row

  alt status=queued/running
    H-->>C: 200 {status, progress, started_at}
  else status=ready
    H-->>C: 200 {status:"ready", download_url:"/v1/exports/{id}/download", size_bytes}
  else status=failed
    H-->>C: 500 {status:"failed", error}
  else not found
    H-->>C: 404
  end

  Note over C,H: Download (отдельный endpoint)
  C->>J: GET /v1/exports/{id}/download
  J-->>H: ok
  H->>R: VerifyReady(id)
  H->>FS: Fiber c.SendFile("/var/exports/{id}.parquet")
  FS-->>C: stream parquet
```

## 10. POST /admin/loads (manual trigger)

```mermaid
sequenceDiagram
  autonumber
  participant Op as IT E-Zoo
  participant J as JWT mw
  participant Role as Role mw (admin only)
  participant Aud as Audit mw
  participant H as handler/admin_loads
  participant S as service.StartLoad
  participant Lock as PG advisory lock
  participant L as Loader
  participant R as repository
  participant PG

  Op->>J: POST /admin/loads + Bearer
  J-->>Role: claims{role:"admin-cli"}
  Role-->>Aud: ok
  Aud->>R: InsertAuditAccess(requester, endpoint, query)
  Aud-->>H: ok
  H->>S: Start()
  S->>Lock: pg_try_advisory_lock('daily-load')
  alt уже занят
    Lock-->>S: false
    S->>R: SELECT id FROM loads WHERE status='running' ORDER BY started_at DESC LIMIT 1
    R-->>S: currentLoadID
    S-->>H: ErrLoadAlreadyRunning(currentLoadID)
    H-->>Op: 409 {code:"load_already_running", currentLoadId}
  else свободен
    Lock-->>S: true
    S->>R: INSERT loads(id, status='running', source='manual')
    Note over S,L: Запуск loader в горутине (async)
    S->>L: go Run(ctx, newLoadID)
    S-->>H: newLoadID
    H-->>Op: 202 {load_id, status:"running"}
  end
```

## 11. POST /admin/loads/{id}/retry

```mermaid
sequenceDiagram
  autonumber
  participant Op
  participant J
  participant Role
  participant Aud
  participant H as handler/admin_loads
  participant S
  participant Lock
  participant L as Loader
  participant R
  participant PG

  Op->>J: POST /admin/loads/{id}/retry
  J-->>Role: ok
  Role-->>Aud: ok
  Aud->>R: INSERT audit_access
  Aud-->>H: ok
  H->>S: Retry(failedLoadID)

  S->>R: SELECT loads WHERE id=$1
  R->>PG: SELECT
  PG-->>R: load row
  R-->>S: load

  alt status != 'failed'
    S-->>H: ErrCannotRetry
    H-->>Op: 409 {code:"cannot_retry", reason:"current status=committed/running"}
  else status='failed'
    S->>Lock: pg_try_advisory_lock('daily-load')
    alt occupied
      Lock-->>S: false
      S-->>H: ErrLoadAlreadyRunning
      H-->>Op: 409
    else
      Lock-->>S: true
      S->>R: INSERT loads(id=newID, status='running', source='retry', parent_id=failedLoadID)
      S->>L: go Run(ctx, newID)
      S-->>H: newID
      H-->>Op: 202 {load_id, retried_from}
    end
  end
```

## 12. GET /admin/loads/{id}

```mermaid
sequenceDiagram
  autonumber
  participant Op
  participant J
  participant Role
  participant Aud
  participant H as handler/admin_loads
  participant S
  participant R
  participant PG

  Op->>J: GET /admin/loads/{id}
  J-->>Role-->>Aud: ok
  Aud->>R: INSERT audit_access
  H->>S: Get(id)
  S->>R: SelectLoadByID(id)
  R->>PG: SELECT * FROM loads WHERE id=$1
  PG-->>R: row
  R-->>S: load
  S->>R: CountRejectLog(loadID)
  R->>PG: SELECT entity, severity, count(*) FROM reject_log WHERE load_id=$1 GROUP BY ...
  PG-->>R: summary
  R-->>S: rejectSummary
  S-->>H: dto
  H-->>Op: 200 {load_id, status, started_at, finished_at, entities_summary, reject_summary}
```

## 13. GET /admin/reject-log

```mermaid
sequenceDiagram
  autonumber
  participant Op
  participant J
  participant Role
  participant Aud
  participant H as handler/admin_reject_log
  participant S
  participant R
  participant PG

  Op->>J: GET /admin/reject-log?load_id=...&entity=...&limit=500
  J-->>Role-->>Aud: ok
  Aud->>R: INSERT audit_access
  H->>S: List(filters)
  S->>R: SelectRejectLog(filters)
  R->>PG: SELECT * FROM reject_log WHERE load_id=$1 AND ($2='' OR entity=$2) ORDER BY detected_at LIMIT $3
  PG-->>R: rows
  R-->>S: rows
  S-->>H: dto
  H-->>Op: 200 NDJSON {load_id, entity, pk_value, severity, reason, raw, detected_at}
```
