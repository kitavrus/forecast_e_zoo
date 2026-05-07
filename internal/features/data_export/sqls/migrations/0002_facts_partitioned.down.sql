-- Down 0002 — DROP CASCADE снимает все партиции автоматически.

DROP TABLE IF EXISTS supplier_stock_snapshot CASCADE;
DROP TABLE IF EXISTS stock_movement CASCADE;
DROP TABLE IF EXISTS location_stock_snapshot CASCADE;
DROP TABLE IF EXISTS receipt_line CASCADE;
