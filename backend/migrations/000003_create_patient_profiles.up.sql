BEGIN;

SET LOCAL lock_timeout = '5s';

-- Requires 000002 (users.role and idx_users_id_role). The nik_rc4 and bpjs_number_rc4 columns stay nullable to match the GORM model.
CREATE TABLE IF NOT EXISTS patient_profiles (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT        NOT NULL,
    user_role       TEXT          NOT NULL DEFAULT 'patient',
    weight_kg       NUMERIC(6, 2) NOT NULL,
    height_cm       NUMERIC(6, 2) NOT NULL,
    age             SMALLINT      NOT NULL,
    nik_aes         BYTEA         NOT NULL,
    nik_des         BYTEA         NOT NULL,
    nik_rc4         BYTEA,
    bpjs_number_aes BYTEA         NOT NULL,
    bpjs_number_des BYTEA         NOT NULL,
    bpjs_number_rc4 BYTEA,
    CONSTRAINT patient_profiles_age_check          CHECK (age >= 0),
    CONSTRAINT patient_profiles_bpjs_aes_check     CHECK (octet_length(bpjs_number_aes) > 0),
    CONSTRAINT patient_profiles_bpjs_des_check     CHECK (octet_length(bpjs_number_des) > 0),
    CONSTRAINT patient_profiles_bpjs_rc4_check     CHECK (octet_length(bpjs_number_rc4) > 0),
    CONSTRAINT patient_profiles_height_check       CHECK (height_cm > 0),
    CONSTRAINT patient_profiles_nik_aes_check      CHECK (octet_length(nik_aes) > 0),
    CONSTRAINT patient_profiles_nik_des_check      CHECK (octet_length(nik_des) > 0),
    CONSTRAINT patient_profiles_nik_rc4_check      CHECK (octet_length(nik_rc4) > 0),
    CONSTRAINT patient_profiles_patient_role_check CHECK (user_role = 'patient'),
    CONSTRAINT patient_profiles_weight_check       CHECK (weight_kg > 0),
    CONSTRAINT fk_users_patient_profile FOREIGN KEY (user_id, user_role)
        REFERENCES users (id, role) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_patient_profiles_user_id ON patient_profiles (user_id);

COMMIT;
