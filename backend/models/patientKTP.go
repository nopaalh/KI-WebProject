package models

type KTPEncryptedValue struct {
	AES []byte `gorm:"column:aes;type:bytea;not null" json:"-"`
	DES []byte `gorm:"column:des;type:bytea;not null" json:"-"`
	RC4 []byte `gorm:"column:rc4;type:bytea;not null" json:"-"`
}

type PatientKTP struct {
	ID               uint `gorm:"primaryKey"`
	PatientProfileID uint `gorm:"not null;uniqueIndex"`

	NIK           KTPEncryptedValue `gorm:"embedded;embeddedPrefix:nik_" json:"-"`
	FullName      KTPEncryptedValue `gorm:"embedded;embeddedPrefix:full_name_" json:"-"`
	BirthPlace    KTPEncryptedValue `gorm:"embedded;embeddedPrefix:birth_place_" json:"-"`
	BirthDate     KTPEncryptedValue `gorm:"embedded;embeddedPrefix:birth_date_" json:"-"`
	Gender        KTPEncryptedValue `gorm:"embedded;embeddedPrefix:gender_" json:"-"`
	BloodType     KTPEncryptedValue `gorm:"embedded;embeddedPrefix:blood_type_" json:"-"`
	Address       KTPEncryptedValue `gorm:"embedded;embeddedPrefix:address_" json:"-"`
	RT            KTPEncryptedValue `gorm:"embedded;embeddedPrefix:rt_" json:"-"`
	RW            KTPEncryptedValue `gorm:"embedded;embeddedPrefix:rw_" json:"-"`
	Village       KTPEncryptedValue `gorm:"embedded;embeddedPrefix:village_" json:"-"`
	District      KTPEncryptedValue `gorm:"embedded;embeddedPrefix:district_" json:"-"`
	Religion      KTPEncryptedValue `gorm:"embedded;embeddedPrefix:religion_" json:"-"`
	MaritalStatus KTPEncryptedValue `gorm:"embedded;embeddedPrefix:marital_status_" json:"-"`
	Occupation    KTPEncryptedValue `gorm:"embedded;embeddedPrefix:occupation_" json:"-"`
	Citizenship   KTPEncryptedValue `gorm:"embedded;embeddedPrefix:citizenship_" json:"-"`

	Photo KTPEncryptedValue `gorm:"embedded;embeddedPrefix:photo_" json:"-"`
}
