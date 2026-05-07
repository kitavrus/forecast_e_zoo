-- Blueprint для cron-pre-step: создание ежемесячной партиции.
-- В Go-коде подставляем %s для имени parent + child + range диапазона.
-- Пример конкретной реализации (формирует один CREATE TABLE IF NOT EXISTS):
--   CREATE TABLE IF NOT EXISTS receipt_line_y2026m08
--     PARTITION OF receipt_line FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
--
-- Ниже — параметризованный template, на старте сервиса используется через fmt.Sprintf:
--   $parent  — имя родительской таблицы (receipt_line | location_stock_snapshot | stock_movement | supplier_stock_snapshot)
--   $child   — имя дочерней (parent || _y<YYYY>m<MM>)
--   $from    — нижняя граница (date)
--   $to      — верхняя граница (date)
CREATE TABLE IF NOT EXISTS %[2]s
    PARTITION OF %[1]s
    FOR VALUES FROM ('%[3]s') TO ('%[4]s');
