BEGIN;

SET LOCAL lock_timeout = '5s';

-- The patient a file is about, separate from files.user_id (the uploader). Nullable: historical rows are not mapped.
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS patient_profile_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_files_patient_profile_id ON files (patient_profile_id);

-- ADD CONSTRAINT has no IF NOT EXISTS, so drop first to stay rerunnable.
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_patient_profiles_attachments;
ALTER TABLE files
    ADD CONSTRAINT fk_patient_profiles_attachments FOREIGN KEY (patient_profile_id)
        REFERENCES patient_profiles (id) ON UPDATE RESTRICT ON DELETE RESTRICT;

COMMIT;
