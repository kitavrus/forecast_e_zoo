-- Down 0001 — откатывает таблицы в обратном порядке.
-- CASCADE снимает FK от потенциальных дочерних таблиц (на случай, если 0002 уже накатили,
-- хотя golang-migrate откатывает ровно по версиям и до сюда не дойдёт без отката 0002).

DROP TABLE IF EXISTS master_change_log CASCADE;
DROP TABLE IF EXISTS supply_plan CASCADE;
DROP TABLE IF EXISTS order_rule CASCADE;
DROP TABLE IF EXISTS promo CASCADE;
DROP TABLE IF EXISTS supply_spec CASCADE;
DROP TABLE IF EXISTS store_assortment_lifecycle_events CASCADE;
DROP TABLE IF EXISTS store_assortment CASCADE;
DROP TABLE IF EXISTS product_barcodes CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS supplier CASCADE;
DROP TABLE IF EXISTS location CASCADE;
DROP TABLE IF EXISTS category CASCADE;

DROP TABLE IF EXISTS audit_access CASCADE;
DROP TABLE IF EXISTS entity_checkpoint CASCADE;
DROP TABLE IF EXISTS reject_log CASCADE;
DROP TABLE IF EXISTS snapshot_pointer CASCADE;
DROP TABLE IF EXISTS loads CASCADE;

-- ltree extension не сносим: расширения могут быть нужны другим базам/схемам.
