-- Возвращает первый running load (если есть).
SELECT load_id, started_at, source
  FROM loads
 WHERE status = 'running'
 LIMIT 1;
