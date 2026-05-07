# Code Plan Status: data-marts

| Phase | Description | Status |
|---|---|---|
| 1 | constants + DTO + models | completed |
| 2 | sqls + repository + integration test | completed |
| 3 | MartReader interface + PG impl + cache + schemas | completed |
| 4 | service + handlers + mappers | completed |
| 5 | router + DI registration | completed |
| 6 | unit tests + validation | completed |

Last updated: phase 6 (validation).

## Quality gates

- `go build ./...` — passed
- `go test -race -count=1 ./internal/features/data_marts/...` — passed (handler 2.26s, repository 1.68s, service 1.96s)
- `golangci-lint run ./internal/features/data_marts/... ./internal/routers/... ./internal/app/...` — 0 issues
- Integration test (postgres:18-alpine) — определён, запускается с `-tags=integration`

## Curl сценарии

```bash
# JWT (issuer it-read or x-flow-etl) обязателен.

# 1. Список mart'ов + версии
curl -H "Authorization: Bearer $JWT" http://localhost:8080/v1/marts

# 2. Версия mart'а
curl -H "Authorization: Bearer $JWT" \
  http://localhost:8080/v1/marts/mart_kpi_daily/version

# 3. Schema
curl -H "Authorization: Bearer $JWT" \
  http://localhost:8080/v1/marts/mart_kpi_daily/schema

# 4. NDJSON streaming
curl -H "Authorization: Bearer $JWT" \
  "http://localhost:8080/v1/marts/mart_demand_history?limit=100"

# 5. Pagination follow-up (cursor из X-Next-Cursor header предыдущего ответа)
curl -H "Authorization: Bearer $JWT" \
  "http://localhost:8080/v1/marts/mart_demand_history?cursor=$NEXT&limit=100"
```

Готовность к Stage 8 (Code Review): да.
