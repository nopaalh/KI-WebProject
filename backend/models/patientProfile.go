package models

type PatientProfile struct {
	ID       uint   `gorm:"primaryKey"`
	UserID   uint   `gorm:"not null;uniqueIndex"`
	UserRole string `gorm:"not null;default:patient;check:patient_profiles_patient_role_check,user_role = 'patient'" json:"-"`

	WeightKGAES []byte `gorm:"column:weight_kg_aes;not null" json:"-"`
	WeightKGDES []byte `gorm:"column:weight_kg_des;not null" json:"-"`
	WeightKGRC4 []byte `gorm:"column:weight_kg_rc4;not null" json:"-"`

	HeightCMAES []byte `gorm:"column:height_cm_aes;not null" json:"-"`
	HeightCMDES []byte `gorm:"column:height_cm_des;not null" json:"-"`
	HeightCMRC4 []byte `gorm:"column:height_cm_rc4;not null" json:"-"`

	AgeAES []byte `gorm:"column:age_aes;not null" json:"-"`
	AgeDES []byte `gorm:"column:age_des;not null" json:"-"`
	AgeRC4 []byte `gorm:"column:age_rc4;not null" json:"-"`

	NIKAES []byte `gorm:"column:nik_aes;not null;check:patient_profiles_nik_aes_check,octet_length(nik_aes) > 0" json:"-"`
	NIKDES []byte `gorm:"column:nik_des;not null;check:patient_profiles_nik_des_check,octet_length(nik_des) > 0" json:"-"`
	NIKRC4 []byte `gorm:"column:nik_rc4;check:patient_profiles_nik_rc4_check,octet_length(nik_rc4) > 0" json:"-"`

	BPJSNumberAES []byte `gorm:"column:bpjs_number_aes;not null;check:patient_profiles_bpjs_aes_check,octet_length(bpjs_number_aes) > 0" json:"-"`
	BPJSNumberDES []byte `gorm:"column:bpjs_number_des;not null;check:patient_profiles_bpjs_des_check,octet_length(bpjs_number_des) > 0" json:"-"`
	BPJSNumberRC4 []byte `gorm:"column:bpjs_number_rc4;check:patient_profiles_bpjs_rc4_check,octet_length(bpjs_number_rc4) > 0" json:"-"`

	Attachments []File `gorm:"foreignKey:PatientProfileID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT" json:"-"`

	KTP *PatientKTP `gorm:"foreignKey:PatientProfileID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT" json:"-"`
}
