BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: this removes every stored role. Run 000003 down first, its foreign key needs idx_users_id_role.
DROP INDEX IF EXISTS idx_users_id_role;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users DROP COLUMN IF EXISTS role;

COMMIT;
