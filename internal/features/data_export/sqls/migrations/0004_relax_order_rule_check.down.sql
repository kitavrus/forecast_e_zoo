-- Re-add original CHECK (product_id IS NOT NULL OR category_id IS NOT NULL).
-- Перед откатом убедитесь, что в order_rule нет location-wide rows
-- (product_id IS NULL AND category_id IS NULL) — иначе ALTER TABLE упадёт.

ALTER TABLE order_rule
    ADD CONSTRAINT order_rule_check
    CHECK (product_id IS NOT NULL OR category_id IS NOT NULL);
