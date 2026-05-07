# Spec Interview: kpi-calibration (inline, defaulted)

```yaml
# Triage
tier: M
touches: {db: true, fe: false, infra: false, external: false}
risk: reversible
novelty: standard-crud
mode: defaulted
```

## Domain

- KPI engine для трёх индустриальных KPI: OSA, OTIF, Stock Days.
- Источники: `marts.mart_demand_history`, `marts.mart_calculation_input`, `marts.mart_supplier_scorecard` (схема marts уже наполняется ETL Module 2).
- Snapshot'ы пишутся в новую schema `kpi`, отдельно от `marts.*`, чтобы read/write контракт KPI был изолирован.

## Принятые ответы (defaulted из research)

| ID | Вопрос | Принято |
|---|---|---|
| Q-001 | Cron schedule | **04:00 Europe/Kyiv** (после ETL 02:30, mart_kpi_daily обновлён). Configurable env `KPI_CRON_SCHEDULE`. |
| Q-002 | Чтение marts | **Прямой доступ через pgxpool** (тот же пул). HTTP `/v1/marts/:name` остаётся только для внешних потребителей. |
| Q-003 | Калибровки overrides | **Иерархия: location → supplier → category → global**. Resolver matches наиболее specific scope first. |
| Q-004 | Retention KPI snapshots | **365 дней** rolling, drop старых партиций (cleanup в backlog). |
| Q-005 | Партиционирование | **RANGE по as_of_date, monthly** — как `mart_kpi_daily`. |
| Q-006 | Recompute past dates | **Manual via POST** `/v1/kpi/snapshots/refresh?from_date=YYYY-MM-DD`. |
| Q-007 | Расположение кода | **internal/features/kpi/** в составе source-adapter binary. |

## Happy path

1. 04:00 — gocron tick.
2. KPI Engine берёт advisory-lock `kpi-engine-run` (advisory key — bigint от sha256("kpi-engine-run")).
3. Лок не получен → tick skipped (метрика `kpi_engine_run_total{result="skipped"}`), normal exit.
4. Лок получен → engine последовательно:
   - читает marts.mart_demand_history → калькулятор OSA → snapshots (per product_location);
   - читает marts.mart_calculation_input → калькулятор Stock Days → snapshots (per product_location);
   - читает marts.mart_supplier_scorecard → калькулятор OTIF → snapshots (per supplier).
5. Записывает агрегированные снапшоты для scope_type=global (среднее по всем).
6. Releases lock, метрика success, snapshot_count_total{kpi_name} наращивается.

## Edge cases

- Quality threshold: если >5% записей вылетает с ошибкой в одном KPI, run помечается failed; non-broken KPI всё равно записываются.
- Параллельный Cron + manual POST refresh — оба пытаются взять lock; второй получит 409 / skip.
- Calibration override — для product/location может не быть override; resolver fallback'ает на supplier → category → global.
- Если mart `mart_supplier_scorecard` пуст за неделю — OTIF не считается (просто 0 rows записывается).

## Error matrix

| Сценарий | Код | HTTP | supportMessage |
|---|---|---|---|
| GET /snapshots/:id, не найден | `kpi_snapshot_not_found` | 404 | KPI-001 |
| GET /calibrations/:id или PUT, не найден | `kpi_calibration_not_found` | 404 | KPI-002 |
| Невалидное `kpi_name` в query/body | `invalid_kpi_name` | 400 | KPI-003 |
| Пустые `params` в PUT calibration | `bad_request` | 400 | SA-REQ-001 |
| Неверный date format | `bad_request` | 400 | SA-REQ-001 |

## Open questions

Нет — все Q-001..Q-007 закрыты defaulted. Для post-MVP: автогенерация партиций KPI snapshots (как mart_partition_maintenance в Module 2).
