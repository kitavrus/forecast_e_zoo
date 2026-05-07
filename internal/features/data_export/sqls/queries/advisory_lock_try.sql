-- Не блокирующий advisory lock. Возвращает true, если лок взят, false — иначе.
-- $1 = key (bigint, FNV-64 hash от 'daily-load' или другой константы из фазы 12).
SELECT pg_try_advisory_lock($1::bigint) AS locked;
