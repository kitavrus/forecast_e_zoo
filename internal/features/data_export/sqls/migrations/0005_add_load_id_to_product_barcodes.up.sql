-- =============================================================================
-- Migration 0005: добавить колонку load_id в product_barcodes.
-- =============================================================================
-- product_barcodes в миграции 0001 не имела load_id (1:N отношение, считалось
-- что трекинг идёт через FK к products). Однако publical handler GET
-- /v1/product_barcodes должен фильтровать по current snapshot.load_id —
-- иначе невозможно корректно отдать «срез» master при rolling update.
--
-- Добавляем load_id (NULLABLE для обратной совместимости с существующими
-- строками, заполняемых при следующем successful load).
-- =============================================================================

ALTER TABLE product_barcodes
    ADD COLUMN IF NOT EXISTS load_id uuid NULL REFERENCES loads(load_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_product_barcodes_load_id
    ON product_barcodes (load_id);
