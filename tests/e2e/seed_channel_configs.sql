-- Seed supplier_channel_config для E2E теста.
-- 50 поставщиков подключены через api_key к mock-erp.
--
-- supplier_id формат должен совпадать с тем, что генерирует mock-erp seeder
-- (mock-erp/app/seeder.py → seed_suppliers): id=f"SUP-{i + 1:04d}" → SUP-0001..SUP-0050.
INSERT INTO channels.supplier_channel_config
    (supplier_id, channel_type, endpoint_url, auth_mode, auth_credentials_ref,
     timeout_sec, retry_max, is_active, created_at, updated_at)
SELECT
    'SUP-' || LPAD(g::text, 4, '0')        AS supplier_id,
    'erp_api'                              AS channel_type,
    'http://mock-erp:8090/api/v1/orders'   AS endpoint_url,
    'api_key'                              AS auth_mode,
    'MOCK_ERP_API_KEY'                     AS auth_credentials_ref,
    30                                     AS timeout_sec,
    3                                      AS retry_max,
    TRUE                                   AS is_active,
    NOW()                                  AS created_at,
    NOW()                                  AS updated_at
FROM generate_series(1, 50) AS g
ON CONFLICT (supplier_id) DO UPDATE SET
    channel_type         = EXCLUDED.channel_type,
    endpoint_url         = EXCLUDED.endpoint_url,
    auth_mode            = EXCLUDED.auth_mode,
    auth_credentials_ref = EXCLUDED.auth_credentials_ref,
    timeout_sec          = EXCLUDED.timeout_sec,
    retry_max            = EXCLUDED.retry_max,
    is_active            = EXCLUDED.is_active,
    updated_at           = NOW();
