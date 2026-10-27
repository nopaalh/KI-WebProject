BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: this deletes every patient profile. Run 000004 down first, its foreign key points here.
DROP TABLE IF EXISTS patient_profiles;

COMMIT;
