BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: this removes all saved DES and RC4 timings.
ALTER TABLE files
    DROP COLUMN IF EXISTS des_encrypt_time_ns,
    DROP COLUMN IF EXISTS des_decrypt_time_ns,
    DROP COLUMN IF EXISTS rc4_encrypt_time_ns,
    DROP COLUMN IF EXISTS rc4_decrypt_time_ns;

COMMIT;
