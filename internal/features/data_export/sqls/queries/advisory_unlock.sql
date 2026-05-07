-- Освобождение advisory lock. $1 = key (bigint).
SELECT pg_advisory_unlock($1::bigint) AS unlocked;
