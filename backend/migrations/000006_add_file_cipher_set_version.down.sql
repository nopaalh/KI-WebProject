BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: rows stored with three ciphers lose their version. If the application starts again, AutoMigrate
-- recreates the column as 0 and those rows are rejected as inconsistent, so roll the application back as well.
ALTER TABLE files
    DROP COLUMN IF EXISTS cipher_set_version;

COMMIT;
