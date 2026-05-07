DROP TABLE IF EXISTS marts.mart_supplier_scorecard;
DROP TABLE IF EXISTS marts.mart_master_current;
DROP TABLE IF EXISTS marts.mart_kpi_daily CASCADE;
DROP TABLE IF EXISTS marts.mart_calculation_input;
DROP TABLE IF EXISTS marts.mart_demand_history CASCADE;
DROP SCHEMA IF EXISTS marts CASCADE;
-- mart_reader role оставляем (может использоваться другими БД-объектами).
