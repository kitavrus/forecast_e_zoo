# Pipeline Report: kpi-calibration (Модуль 4)
**Дата:** 2026-05-07
**Tier:** M (compact)

## Что сделано

Реализован KPI engine: cron tick 04:00 Europe/Kyiv → читает marts → resolve hierarchical calibrations → считает OSA/OTIF/Stock Days → пишет в `kpi.kpi_snapshots` (partitioned by as_of_date). 5 endpoints + admin recompute.

## Endpoints

| Method | Path | Auth | Описание |
|---|---|---|---|
| GET | /v1/kpi/snapshots | admin/etl/it-read | List + filters + cursor |
| GET | /v1/kpi/snapshots/:id | admin/etl/it-read | Single snapshot |
| GET | /v1/kpi/calibrations | admin/etl/it-read | List with hierarchy |
| PUT | /v1/kpi/calibrations/:id | admin-cli | Update calibration |
| POST | /v1/kpi/snapshots/refresh | admin-cli | Ondemand recompute (202/409) |

## KPI definitions

- **OSA:** sum(time_in_stock) / total_observation_time per (product_id, location_id, as_of_date)
- **OTIF:** (заказы on-time AND полные) / total orders per supplier per period
- **Stock Days:** current_stock / avg_daily_demand_30d

## Калибровка
Hierarchical resolver: product_location > location > supplier > category > global. JSON params per calibration row.

## Метрики прогона
- 9 git коммитов (8 фаз + docs sync)
- 1 миграция (2001_kpi_schema)
- 4 Prometheus метрики
- ~30 unit + 1 integration test

## Quality gates
- `go build ./...` — OK
- `go test -race -count=1 ./internal/features/kpi/...` — OK
- `golangci-lint run` — 0 issues

## Известные ограничения
- Только 3 KPI (OSA/OTIF/Stock Days). Другие (SLA, fill rate, forecast accuracy) — следующая итерация.
- Calibration hierarchy зашита в коде. UI для edit — нет (только REST PUT).
- KPI snapshot retention 365d, partitioned by as_of_date (RANGE month).
- Quality threshold 5% (если >5% строк выпадают с ошибкой при расчёте → fail run).

## Артефакты
- design.md, spec-interview/output.md, code-plan + status, 8 phase-файлов
- Реализация: internal/features/kpi/{handler,service,repository,calculators,calibration,engine,scheduler,...}
