BEGIN;

SET LOCAL lock_timeout = '5s';

-- Requires 000003. Every KTP field is stored once per cipher, as the GORM embedded prefixes do.
CREATE TABLE IF NOT EXISTS patient_ktps (
    id                 BIGSERIAL PRIMARY KEY,
    patient_profile_id BIGINT NOT NULL,
    nik_aes BYTEA NOT NULL,
    nik_des BYTEA NOT NULL,
    nik_rc4 BYTEA NOT NULL,
    full_name_aes BYTEA NOT NULL,
    full_name_des BYTEA NOT NULL,
    full_name_rc4 BYTEA NOT NULL,
    birth_place_aes BYTEA NOT NULL,
    birth_place_des BYTEA NOT NULL,
    birth_place_rc4 BYTEA NOT NULL,
    birth_date_aes BYTEA NOT NULL,
    birth_date_des BYTEA NOT NULL,
    birth_date_rc4 BYTEA NOT NULL,
    gender_aes BYTEA NOT NULL,
    gender_des BYTEA NOT NULL,
    gender_rc4 BYTEA NOT NULL,
    blood_type_aes BYTEA NOT NULL,
    blood_type_des BYTEA NOT NULL,
    blood_type_rc4 BYTEA NOT NULL,
    address_aes BYTEA NOT NULL,
    address_des BYTEA NOT NULL,
    address_rc4 BYTEA NOT NULL,
    rt_aes BYTEA NOT NULL,
    rt_des BYTEA NOT NULL,
    rt_rc4 BYTEA NOT NULL,
    rw_aes BYTEA NOT NULL,
    rw_des BYTEA NOT NULL,
    rw_rc4 BYTEA NOT NULL,
    village_aes BYTEA NOT NULL,
    village_des BYTEA NOT NULL,
    village_rc4 BYTEA NOT NULL,
    district_aes BYTEA NOT NULL,
    district_des BYTEA NOT NULL,
    district_rc4 BYTEA NOT NULL,
    religion_aes BYTEA NOT NULL,
    religion_des BYTEA NOT NULL,
    religion_rc4 BYTEA NOT NULL,
    marital_status_aes BYTEA NOT NULL,
    marital_status_des BYTEA NOT NULL,
    marital_status_rc4 BYTEA NOT NULL,
    occupation_aes BYTEA NOT NULL,
    occupation_des BYTEA NOT NULL,
    occupation_rc4 BYTEA NOT NULL,
    citizenship_aes BYTEA NOT NULL,
    citizenship_des BYTEA NOT NULL,
    citizenship_rc4 BYTEA NOT NULL,
    photo_aes BYTEA NOT NULL,
    photo_des BYTEA NOT NULL,
    photo_rc4 BYTEA NOT NULL,
    CONSTRAINT fk_patient_profiles_ktp FOREIGN KEY (patient_profile_id)
        REFERENCES patient_profiles (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_patient_ktps_patient_profile_id ON patient_ktps (patient_profile_id);

COMMIT;
