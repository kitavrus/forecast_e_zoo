-- Upsert channel config (PUT /v1/channels/configs/:supplier_id).
-- Параметры:
--   $1 supplier_id, $2 channel_type, $3 endpoint_url, $4 auth_mode,
--   $5 auth_credentials_ref (nullable), $6 timeout_sec, $7 retry_max, $8 is_active.
INSERT INTO channels.supplier_channel_config (
    supplier_id, channel_type, endpoint_url, auth_mode,
    auth_credentials_ref, timeout_sec, retry_max, is_active,
    created_at, updated_at
) VALUES ($1::text, $2::text, $3::text, $4::text,
          $5::text, $6::int, $7::int, $8::bool,
          now(), now())
ON CONFLICT (supplier_id) DO UPDATE SET
    channel_type         = EXCLUDED.channel_type,
    endpoint_url         = EXCLUDED.endpoint_url,
    auth_mode            = EXCLUDED.auth_mode,
    auth_credentials_ref = EXCLUDED.auth_credentials_ref,
    timeout_sec          = EXCLUDED.timeout_sec,
    retry_max            = EXCLUDED.retry_max,
    is_active            = EXCLUDED.is_active,
    updated_at           = now()
RETURNING supplier_id, channel_type, endpoint_url, auth_mode,
          auth_credentials_ref, timeout_sec, retry_max, is_active,
          created_at, updated_at
