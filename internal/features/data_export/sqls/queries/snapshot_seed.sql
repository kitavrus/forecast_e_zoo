-- Инициализация single-row snapshot_pointer (id = 1). Вызывается одноразово на старте сервиса.
INSERT INTO snapshot_pointer (id) VALUES (1) ON CONFLICT DO NOTHING;
