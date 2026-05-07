-- Channel config для supplier (только активный).
-- Параметры: $1 = supplier_id (text).
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
WHERE supplier_id = $1::text AND is_active = TRUE
