-- Список всех каналов (admin endpoint).
SELECT
    supplier_id,
    channel_type,
    endpoint_url,
    auth_mode,
    auth_credentials_ref,
    timeout_sec,
    retry_max,
    is_active,
    created_at,
    updated_at
FROM channels.supplier_channel_config
ORDER BY supplier_id ASC
