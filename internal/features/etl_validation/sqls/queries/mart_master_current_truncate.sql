-- TRUNCATE marts.mart_master_current.
-- Выполняется отдельным Exec-ом до mart_master_current_truncate_insert.sql,
-- т.к. INSERT использует $-параметры и pgx запрещает multi-statement
-- prepared query (SQLSTATE 42601).
TRUNCATE TABLE marts.mart_master_current;
