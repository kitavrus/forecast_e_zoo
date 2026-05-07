-- current_version.sql
-- Возвращает последнюю committed версию ETL run'а — глобальную версию всего набора витрин.
-- Используется когда конкретный mart не указан (общая версия snapshot'а).
SELECT id, committed_at
FROM marts.etl_runs
WHERE status = 'committed'
ORDER BY committed_at DESC
LIMIT 1;
