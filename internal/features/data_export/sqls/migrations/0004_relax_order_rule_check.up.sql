-- =============================================================================
-- Migration 0004: relax order_rule scope CHECK to allow location-wide rules.
-- =============================================================================
-- Прежний CHECK (product_id IS NOT NULL OR category_id IS NOT NULL) запрещал
-- location-wide правила (rule_type='safety_stock' для всей точки), которые
-- активно используются в e_zoo / mock-erp seed (один rule на location, без
-- привязки к продукту/категории). Снимаем ограничение — scope правил
-- определяется присутствием/отсутствием product_id/category_id в payload.
-- =============================================================================

ALTER TABLE order_rule DROP CONSTRAINT IF EXISTS order_rule_check;
