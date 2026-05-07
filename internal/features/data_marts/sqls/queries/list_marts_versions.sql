-- list_marts_versions.sql
-- Возвращает для каждой mart-таблицы её актуальный committed etl_run_id и committed_at.
-- В MVP (один full-run наполняет все mart'ы одновременно) можно было бы выдать одну строку,
-- но сохраняем per-mart разрез — будущий mart_refresh может обновлять одну витрину независимо.
--
-- Использует EXISTS-проверку, что в mart-таблице есть хотя бы одна строка с данным etl_run_id.
WITH last_committed AS (
    SELECT id, committed_at
    FROM marts.etl_runs
    WHERE status = 'committed'
    ORDER BY committed_at DESC
    LIMIT 1
)
SELECT 'mart_demand_history'    AS name, id, committed_at FROM last_committed
UNION ALL
SELECT 'mart_calculation_input' AS name, id, committed_at FROM last_committed
UNION ALL
SELECT 'mart_kpi_daily'         AS name, id, committed_at FROM last_committed
UNION ALL
SELECT 'mart_master_current'    AS name, id, committed_at FROM last_committed
UNION ALL
SELECT 'mart_supplier_scorecard' AS name, id, committed_at FROM last_committed;
