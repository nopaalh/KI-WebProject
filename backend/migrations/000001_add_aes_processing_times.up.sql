BEGIN;

SET LOCAL lock_timeout = '5s';

-- Tolerate columns already added by the application's existing AutoMigrate.
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS aes_encrypt_time_ns BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS aes_decrypt_time_ns BIGINT NOT NULL DEFAULT 0;

COMMIT;
