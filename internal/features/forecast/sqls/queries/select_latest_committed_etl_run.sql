-- Возвращает id последнего committed marts.etl_runs (snapshot версия).
SELECT id
FROM marts.etl_runs
WHERE status = 'committed'
ORDER BY committed_at DESC
LIMIT 1
