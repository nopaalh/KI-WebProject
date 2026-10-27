package models

type PatientProfile struct {
	ID       uint   `gorm:"primaryKey"`
	UserID   uint   `gorm:"not null;uniqueIndex"`
	UserRole string `gorm:"not null;default:patient;check:patient_profiles_patient_role_check,user_role = 'patient'" json:"-"`

	WeightKG float64 `gorm:"column:weight_kg;type:numeric(6,2);not null;check:patient_profiles_weight_check,weight_kg > 0"`
	HeightCM float64 `gorm:"column:height_cm;type:numeric(6,2);not null;check:patient_profiles_height_check,height_cm > 0"`
	Age      int     `gorm:"type:smallint;not null;check:patient_profiles_age_check,age >= 0"`

	NIKAES []byte `gorm:"column:nik_aes;not null;check:patient_profiles_nik_aes_check,octet_length(nik_aes) > 0" json:"-"`
	NIKDES []byte `gorm:"column:nik_des;not null;check:patient_profiles_nik_des_check,octet_length(nik_des) > 0" json:"-"`
	NIKRC4 []byte `gorm:"column:nik_rc4;not null;check:patient_profiles_nik_rc4_check,octet_length(nik_rc4) > 0" json:"-"`

	BPJSNumberAES []byte `gorm:"column:bpjs_number_aes;not null;check:patient_profiles_bpjs_aes_check,octet_length(bpjs_number_aes) > 0" json:"-"`
	BPJSNumberDES []byte `gorm:"column:bpjs_number_des;not null;check:patient_profiles_bpjs_des_check,octet_length(bpjs_number_des) > 0" json:"-"`
	BPJSNumberRC4 []byte `gorm:"column:bpjs_number_rc4;not null;check:patient_profiles_bpjs_rc4_check,octet_length(bpjs_number_rc4) > 0" json:"-"`
}
