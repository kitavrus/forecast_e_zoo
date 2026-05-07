-- Rollback Module 5: forecast schema.
DROP TABLE IF EXISTS forecast.replenishment_plans CASCADE;
DROP TABLE IF EXISTS forecast.calculation_lines CASCADE;
DROP TABLE IF EXISTS forecast.forecasts CASCADE;
DROP TABLE IF EXISTS forecast.forecast_runs CASCADE;
DROP SCHEMA IF EXISTS forecast CASCADE;
