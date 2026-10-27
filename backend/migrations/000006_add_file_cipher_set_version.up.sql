BEGIN;

SET LOCAL lock_timeout = '5s';

-- 0 marks historical AES-only rows, 1 marks uploads stored with all three ciphers. Existing rows keep 0 because
-- they only have an AES copy; the type matches the int field GORM maps to bigint.
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS cipher_set_version BIGINT NOT NULL DEFAULT 0;

COMMIT;
