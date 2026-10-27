BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: this removes all saved timings, including columns created by AutoMigrate.
ALTER TABLE files
    DROP COLUMN IF EXISTS aes_decrypt_time_ns,
    DROP COLUMN IF EXISTS aes_encrypt_time_ns;

COMMIT;
