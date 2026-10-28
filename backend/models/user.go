package models

const (
	RolePatient = "patient"
	RoleDoctor  = "doctor"
)

type User struct {
	ID      uint   `gorm:"primaryKey;uniqueIndex:idx_users_id_role,priority:1"`
	NameAES []byte `gorm:"column:name_aes;not null" json:"-"`
	NameDES []byte `gorm:"column:name_des;not null" json:"-"`
	NameRC4 []byte `gorm:"column:name_rc4;not null" json:"-"`

	// UsernameIndex = HMAC(kunci master, username), dipakai untuk login dan menolak username kembar.
	UsernameIndex []byte `gorm:"column:username_index;not null;uniqueIndex" json:"-"`
	UsernameAES   []byte `gorm:"column:username_aes;not null" json:"-"`
	UsernameDES   []byte `gorm:"column:username_des;not null" json:"-"`
	UsernameRC4   []byte `gorm:"column:username_rc4;not null" json:"-"`
	PasswordHash  string `gorm:"not null"`
	Role          string `gorm:"not null;default:patient;check:users_role_check,role IN ('patient','doctor');uniqueIndex:idx_users_id_role,priority:2"`

	Files          []File          `gorm:"foreignKey:UserID"`
	PatientProfile *PatientProfile `gorm:"foreignKey:UserID,UserRole;references:ID,Role;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT" json:"-"`
}
