BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: this removes the link between files and patients. Dropping the column also drops its index.
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_patient_profiles_attachments;
ALTER TABLE files DROP COLUMN IF EXISTS patient_profile_id;

COMMIT;
