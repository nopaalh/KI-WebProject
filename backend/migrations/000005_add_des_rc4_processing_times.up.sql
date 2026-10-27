BEGIN;

SET LOCAL lock_timeout = '5s';

-- Mirrors 000001 for DES and RC4. Existing rows get 0, meaning "not measured".
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS des_encrypt_time_ns BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS des_decrypt_time_ns BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rc4_encrypt_time_ns BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rc4_decrypt_time_ns BIGINT NOT NULL DEFAULT 0;

COMMIT;
