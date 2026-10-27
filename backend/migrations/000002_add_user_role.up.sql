BEGIN;

SET LOCAL lock_timeout = '5s';

-- Existing users become patients, the same default AutoMigrate applies.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'patient';

-- ADD CONSTRAINT has no IF NOT EXISTS, so drop first to stay rerunnable.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users
    ADD CONSTRAINT users_role_check CHECK (role IN ('patient', 'doctor'));

-- patient_profiles references (id, role) so only a patient user can own a profile.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_id_role ON users (id, role);

COMMIT;
