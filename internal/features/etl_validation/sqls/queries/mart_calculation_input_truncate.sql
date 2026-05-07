-- TRUNCATE marts.mart_calculation_input.
-- Выполняется отдельным Exec-ом до mart_calculation_input_truncate_insert.sql
-- (pgx запрещает multi-statement prepared query с $-параметрами,
-- SQLSTATE 42601).
TRUNCATE TABLE marts.mart_calculation_input;
