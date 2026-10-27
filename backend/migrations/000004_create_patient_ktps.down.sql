BEGIN;

SET LOCAL lock_timeout = '5s';

-- Destructive: this deletes every encrypted KTP record.
DROP TABLE IF EXISTS patient_ktps;

COMMIT;
