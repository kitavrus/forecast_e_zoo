-- Down 0003 — DROP только новых партиций (2025-05 .. 2026-03).
-- Партиции 2026-04..07 остаются (из миграции 0002).

DROP TABLE IF EXISTS receipt_line_y2026m03;
DROP TABLE IF EXISTS receipt_line_y2026m02;
DROP TABLE IF EXISTS receipt_line_y2026m01;
DROP TABLE IF EXISTS receipt_line_y2025m12;
DROP TABLE IF EXISTS receipt_line_y2025m11;
DROP TABLE IF EXISTS receipt_line_y2025m10;
DROP TABLE IF EXISTS receipt_line_y2025m09;
DROP TABLE IF EXISTS receipt_line_y2025m08;
DROP TABLE IF EXISTS receipt_line_y2025m07;
DROP TABLE IF EXISTS receipt_line_y2025m06;
DROP TABLE IF EXISTS receipt_line_y2025m05;

DROP TABLE IF EXISTS location_stock_snapshot_y2026m03;
DROP TABLE IF EXISTS location_stock_snapshot_y2026m02;
DROP TABLE IF EXISTS location_stock_snapshot_y2026m01;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m12;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m11;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m10;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m09;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m08;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m07;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m06;
DROP TABLE IF EXISTS location_stock_snapshot_y2025m05;

DROP TABLE IF EXISTS stock_movement_y2026m03;
DROP TABLE IF EXISTS stock_movement_y2026m02;
DROP TABLE IF EXISTS stock_movement_y2026m01;
DROP TABLE IF EXISTS stock_movement_y2025m12;
DROP TABLE IF EXISTS stock_movement_y2025m11;
DROP TABLE IF EXISTS stock_movement_y2025m10;
DROP TABLE IF EXISTS stock_movement_y2025m09;
DROP TABLE IF EXISTS stock_movement_y2025m08;
DROP TABLE IF EXISTS stock_movement_y2025m07;
DROP TABLE IF EXISTS stock_movement_y2025m06;
DROP TABLE IF EXISTS stock_movement_y2025m05;

DROP TABLE IF EXISTS supplier_stock_snapshot_y2026m03;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2026m02;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2026m01;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m12;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m11;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m10;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m09;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m08;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m07;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m06;
DROP TABLE IF EXISTS supplier_stock_snapshot_y2025m05;
