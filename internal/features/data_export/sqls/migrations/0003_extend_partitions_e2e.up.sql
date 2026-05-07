-- =============================================================================
-- Migration 0003: extend partitions for 365-day E2E history.
-- Создаёт monthly RANGE-партиции для всех 4 партиционированных facts-таблиц
-- на диапазон 2025-05 .. 2026-07 (15 месяцев). Идемпотентно (IF NOT EXISTS):
-- 4 партиции из 0002 (2026-04..07) пропускаются.
-- =============================================================================

-- ============================================================
-- receipt_line
-- ============================================================
CREATE TABLE IF NOT EXISTS receipt_line_y2025m05
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m06
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m07
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m08
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m09
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m10
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m11
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2025m12
    PARTITION OF receipt_line
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m01
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m02
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m03
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m04
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m05
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m06
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m07
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- ============================================================
-- location_stock_snapshot
-- ============================================================
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m05
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m06
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m07
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m08
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m09
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m10
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m11
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2025m12
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m01
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m02
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m03
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m04
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m05
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m06
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m07
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- ============================================================
-- stock_movement
-- ============================================================
CREATE TABLE IF NOT EXISTS stock_movement_y2025m05
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m06
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m07
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m08
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m09
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m10
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m11
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2025m12
    PARTITION OF stock_movement
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m01
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m02
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m03
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m04
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m05
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m06
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m07
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- ============================================================
-- supplier_stock_snapshot
-- ============================================================
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m05
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m06
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m07
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m08
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m09
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m10
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m11
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2025m12
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m01
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m02
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m03
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m04
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m05
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m06
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m07
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
