-- Rollback Module 6 (order-builder).

-- Возвращаем replenishment_plans 'converted' → 'approved' до отката CHECK.
UPDATE forecast.replenishment_plans
SET status = 'approved'
WHERE status = 'converted';

ALTER TABLE forecast.replenishment_plans
    DROP CONSTRAINT IF EXISTS replenishment_plans_status_check;
ALTER TABLE forecast.replenishment_plans
    ADD CONSTRAINT replenishment_plans_status_check
    CHECK (status IN ('draft','approved','cancelled'));

DROP TABLE IF EXISTS orders.po_status_history;
DROP TABLE IF EXISTS orders.po_lines;
DROP TABLE IF EXISTS orders.purchase_orders_2026_07;
DROP TABLE IF EXISTS orders.purchase_orders_2026_06;
DROP TABLE IF EXISTS orders.purchase_orders_2026_05;
DROP TABLE IF EXISTS orders.purchase_orders;
DROP SEQUENCE IF EXISTS orders.po_number_seq;
DROP SCHEMA IF EXISTS orders;
