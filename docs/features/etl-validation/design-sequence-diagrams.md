# Design Sequence Diagrams — etl-validation

Sequence-диаграммы для каждого endpoint + cron-tick.

---

## 0. Cron tick → ETL run (внутрипроцессный)

```mermaid
sequenceDiagram
  autonumber
  participant Cron as Scheduler<br/>(gocron)
  participant Pipe as EtlPipeline
  participant Lock as PG advisory lock
  participant Repo as Repository<br/>(marts.etl_runs)
  participant Ext as Extractor<br/>(HTTP client)
  participant SA as source-adapter API
  participant Stg as staging
  participant Val as ValidationEngine
  participant Trans as Transformer
  participant Load as Loader
  participant Metrics as Prometheus

  Cron->>Pipe: tick (02:30 Kyiv)
  Pipe->>Lock: pg_try_advisory_lock(hash('etl-run'))
  alt lock taken
    Lock-->>Pipe: false
    Pipe->>Metrics: etl_skipped_lock_taken_total++
    Pipe-->>Cron: skip silently
  else lock acquired
    Lock-->>Pipe: true
    Pipe->>Repo: INSERT etl_runs(id=uuidv4, status='running', source_load_id=NULL)
    Pipe->>Ext: GetCurrentSnapshot()
    Ext->>SA: GET /v1/snapshots/current (JWT x-flow-etl)
    alt snapshot not ready
      SA-->>Ext: 503 snapshot_not_ready
      Ext-->>Pipe: ErrSnapshotNotReady
      Pipe->>Repo: UPDATE etl_runs status='aborted', reason='snapshot_not_ready'
      Pipe->>Metrics: etl_skipped_no_snapshot_total++
      Pipe->>Lock: pg_advisory_unlock(...)
      Pipe-->>Cron: skip
    else ok
      SA-->>Ext: {current_load_id: Y}
      Ext-->>Pipe: Y
      Pipe->>Repo: UPDATE etl_runs SET source_load_id=Y
      loop по сущностям (master → facts → доп.)
        Pipe->>Ext: StreamEntity(entity, Y, etag)
        Ext->>SA: GET /v1/{entity}?snapshot=Y (JWT, ETag)
        SA-->>Ext: 200 NDJSON stream
        Ext-->>Pipe: rows
        Pipe->>Stg: INSERT INTO stg_{entity}
      end
      Pipe->>Val: CheckBatch(stg_*)
      Val-->>Pipe: violations[]
      Pipe->>Repo: INSERT INTO reject_log (per violation)
      alt critical/total > 1%
        Pipe->>Repo: UPDATE etl_runs status='failed', reason=ErrQualityThresholdExceeded
        Pipe->>Metrics: etl_run_failed_total{reason="quality"}++
        Pipe->>Lock: unlock
      else quality ok
        loop по 5 mart-таблицам
          Pipe->>Trans: Build{mart}(runID, Y)
          Trans->>Load: UpsertAndFlip(runID, mart)
          Load->>Repo: INSERT/UPSERT mart_* (один tx)
        end
        Pipe->>Repo: UPDATE etl_runs status='committed', committed_at=now()
        Pipe->>Metrics: etl_run_success_total++, etl_run_duration_seconds=...
        Pipe->>Lock: unlock
      end
    end
  end
```

---

## 1. POST /admin/etl-runs (force start)

```mermaid
sequenceDiagram
  autonumber
  actor DevOps
  participant API as Fiber router
  participant JWT as JWT middleware
  participant Audit as audit middleware
  participant H as handler.AdminEtlRuns
  participant V as validators
  participant S as service.EtlRun
  participant Pipe as EtlPipeline
  participant Repo as Repository
  participant Err as errorspkg.WriteJSON

  DevOps->>API: POST /admin/etl-runs (Bearer JWT admin-cli)
  API->>JWT: parse + verify
  JWT-->>API: claims{role=admin-cli, sub=user@x-flow}
  API->>Audit: INSERT audit_access(method, path, sub)
  API->>H: invoke
  H->>V: validate request DTO (none)
  H->>S: TriggerRun(ctx, requester)
  S->>Pipe: TryStart(ctx, source='admin', requester)
  alt advisory lock busy
    Pipe-->>S: ErrEtlRunAlreadyRunning + currentRunID
    S-->>H: error
    H->>Err: WriteJSON 409 + currentRunID
  else snapshot not ready
    Pipe-->>S: ErrSnapshotNotReady
    S-->>H: error
    H->>Err: WriteJSON 503
  else ok
    Pipe->>Repo: INSERT etl_runs running
    Pipe-->>S: runID
    Note over Pipe: pipeline продолжает в фоне (goroutine)
    S-->>H: {run_id, status='running', started_at}
    H-->>DevOps: 202 Accepted + JSON
  end
```

---

## 2. POST /admin/etl-runs/{id}/retry

```mermaid
sequenceDiagram
  autonumber
  actor DevOps
  participant API as Fiber router
  participant H as handler.AdminEtlRuns
  participant S as service.EtlRun
  participant Repo as Repository
  participant Pipe as EtlPipeline

  DevOps->>API: POST /admin/etl-runs/{id}/retry (JWT admin-cli)
  API->>H: invoke (id)
  H->>S: Retry(ctx, id)
  S->>Repo: SELECT etl_runs WHERE id=$1
  alt not found
    Repo-->>S: ErrEtlRunNotFound
    S-->>H: 404
  else status NOT IN ('failed','aborted')
    Repo-->>S: row
    S-->>H: ErrCannotRetryEtl → 409
  else ok
    Repo-->>S: row{source_load_id=Y, status='failed'}
    S->>Pipe: TryStartWithFixedSnapshot(ctx, sourceLoadID=Y)
    alt advisory lock busy
      Pipe-->>S: ErrEtlRunAlreadyRunning
      S-->>H: 409
    else
      Pipe->>Repo: INSERT etl_runs (newID, source_load_id=Y, status='running', parent_run_id=id)
      Pipe-->>S: newRunID
      S-->>H: {run_id=newRunID, retry_of=id, status='running'}
      H-->>DevOps: 202 Accepted
    end
  end
```

---

## 3. GET /admin/etl-runs/{id}

```mermaid
sequenceDiagram
  autonumber
  actor User as DevOps / IT-read
  participant API as Fiber router
  participant JWT as JWT middleware
  participant H as handler
  participant S as service
  participant Repo as Repository

  User->>API: GET /admin/etl-runs/{id} (JWT admin-cli|it-read)
  API->>JWT: verify role ∈ {admin-cli, it-read}
  API->>H: invoke
  H->>S: Get(ctx, id)
  S->>Repo: SELECT FROM etl_runs WHERE id=$1
  alt not found
    Repo-->>S: ErrEtlRunNotFound
    S-->>H: 404
  else ok
    Repo-->>S: row + marts_summary JSONB
    S-->>H: DTO
    H-->>User: 200 JSON {id, status, started_at, committed_at, source_load_id, marts_summary, failure_reason}
  end
```

---

## 4. GET /admin/etl-runs (пагинированный список)

```mermaid
sequenceDiagram
  autonumber
  actor User as DevOps / IT-read
  participant API as Fiber router
  participant H as handler
  participant V as validators
  participant S as service
  participant Repo as Repository

  User->>API: GET /admin/etl-runs?status=...&limit=50&cursor=... (JWT)
  API->>H: invoke
  H->>V: parse + validate query (limit ≤ 100)
  H->>S: List(ctx, filters)
  S->>Repo: SELECT FROM etl_runs ORDER BY started_at DESC LIMIT $1 ...
  Repo-->>S: rows + nextCursor
  S-->>H: page DTO
  H-->>User: 200 JSON {items: [...], next_cursor: ...}
```

---

## 5. POST /admin/marts/{name}/refresh

```mermaid
sequenceDiagram
  autonumber
  actor DevOps
  participant API as Fiber router
  participant H as handler.AdminMarts
  participant V as validators
  participant S as service.MartRefresh
  participant Pipe as EtlPipeline (refresh-mode)
  participant Trans as Transformer

  DevOps->>API: POST /admin/marts/mart_supplier_scorecard/refresh (JWT admin-cli)
  API->>H: invoke
  H->>V: validate name
  alt name != mart_supplier_scorecard
    V-->>H: ErrMartRefreshNotSupported → 400
  else
    H->>S: Refresh(ctx, name)
    S->>Pipe: TryStartRefresh(ctx, mart=name)
    alt advisory lock busy
      Pipe-->>S: ErrEtlRunAlreadyRunning
      S-->>H: 409
    else
      Pipe->>Trans: BuildSupplierScorecard(runID, snapshotID)
      Pipe-->>S: runID
      S-->>H: {run_id, kind='mart_refresh', target='mart_supplier_scorecard', status='running'}
      H-->>DevOps: 202 Accepted
    end
  end
```

---

## 6. GET /admin/reject-log

```mermaid
sequenceDiagram
  autonumber
  actor DevOps
  participant API as Fiber router
  participant H as handler.AdminRejectLog
  participant V as validators
  participant S as service
  participant Repo as Repository

  DevOps->>API: GET /admin/reject-log?etl_run_id=...&entity=...&severity=critical&limit=100 (JWT admin-cli)
  API->>H: invoke
  H->>V: validate filters
  H->>S: List(ctx, filters)
  S->>Repo: SELECT FROM reject_log WHERE ... LIMIT $N
  Repo-->>S: rows
  S-->>H: page DTO
  H-->>DevOps: 200 JSON {items, next_cursor}
```

---

## 7. GET /healthz

```mermaid
sequenceDiagram
  autonumber
  participant K as Kubelet / docker healthcheck
  participant API as Fiber router
  participant H as handler.Healthz
  participant DB as pgxpool.Ping
  participant Cron as Scheduler

  K->>API: GET /healthz
  API->>H: invoke
  H->>DB: Ping(ctx, 200ms)
  H->>Cron: IsRunning()
  alt all OK
    H-->>K: 200 {"status":"ok"}
  else DB down
    H-->>K: 503 {"status":"db_down"}
  else cron not started
    H-->>K: 503 {"status":"scheduler_down"}
  end
```

---

## 8. GET /metrics

Prometheus exposition. Никакой бизнес-логики; только `promhttp.Handler()` через Fiber adapter. Метрики защищены отдельным портом или basic-auth (см. [design-infrastructure.md](design-infrastructure.md)).
