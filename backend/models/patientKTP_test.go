package models

import (
	"encoding/json"
	"testing"

	"gorm.io/gorm/schema"
)

func TestPatientKTPSchema(t *testing.T) {
	s := parseModelSchema(t, &PatientKTP{})
	if s.Table != "patient_ktps" {
		t.Fatalf("unexpected table: %s", s.Table)
	}

	prefixes := []string{
		"nik", "full_name", "birth_place", "birth_date", "gender", "blood_type",
		"address", "rt", "rw", "village", "district", "religion", "marital_status",
		"occupation", "citizenship", "photo",
	}
	for _, prefix := range prefixes {
		if s.FieldsByDBName[prefix] != nil {
			t.Errorf("unexpected plaintext column: %s", prefix)
		}
		for _, cipher := range []string{"aes", "des", "rc4"} {
			name := prefix + "_" + cipher
			field := s.FieldsByDBName[name]
			if field == nil || field.DataType != "bytea" || !field.NotNull {
				t.Errorf("missing required bytea ciphertext column: %s", name)
			}
		}
	}
	if len(s.FieldsByDBName) != 2+len(prefixes)*3 {
		t.Fatalf("expected two IDs and 48 ciphertext columns, got %d", len(s.FieldsByDBName))
	}
	if field := s.FieldsByDBName["patient_profile_id"]; field == nil || !field.NotNull {
		t.Fatal("patient_profile_id must be required")
	}
	for _, index := range s.ParseIndexes() {
		if index.Class == "UNIQUE" && len(index.Fields) == 1 && index.Fields[0].DBName == "patient_profile_id" {
			return
		}
	}
	t.Fatal("a patient profile must have at most one KTP")
}

func TestPatientKTPReferencesProfile(t *testing.T) {
	s := parseModelSchema(t, &PatientProfile{})
	rel := s.Relationships.Relations["KTP"]
	if rel == nil || rel.Type != schema.HasOne {
		t.Fatal("expected a has-one KTP association")
	}
	constraint := rel.ParseConstraint()
	if constraint == nil || constraint.Schema.Table != "patient_ktps" || constraint.ReferenceSchema.Table != "patient_profiles" {
		t.Fatalf("unexpected KTP foreign key: %+v", constraint)
	}
	if len(constraint.ForeignKeys) != 1 || len(constraint.References) != 1 ||
		constraint.ForeignKeys[0].DBName != "patient_profile_id" || constraint.References[0].DBName != "id" {
		t.Fatal("KTP must reference the profile ID, not randomized NIK ciphertext")
	}
	if constraint.OnUpdate != "RESTRICT" || constraint.OnDelete != "RESTRICT" {
		t.Fatal("profile changes/deletion must not silently cascade into a KTP")
	}
}

func TestPatientKTPJSONExcludesCiphertext(t *testing.T) {
	// The bytes are only a serialization fixture, not encrypted test data.
	fixture := KTPEncryptedValue{AES: []byte("fixture"), DES: []byte("fixture"), RC4: []byte("fixture")}
	ktp := PatientKTP{
		ID: 1, PatientProfileID: 2,
		NIK: fixture, FullName: fixture, BirthPlace: fixture, BirthDate: fixture,
		Gender: fixture, BloodType: fixture, Address: fixture, RT: fixture, RW: fixture,
		Village: fixture, District: fixture, Religion: fixture, MaritalStatus: fixture,
		Occupation: fixture, Citizenship: fixture, Photo: fixture,
	}
	encoded, err := json.Marshal(ktp)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["ID"] != float64(1) || fields["PatientProfileID"] != float64(2) {
		t.Fatalf("ciphertext/internal fields leaked in JSON: %s", encoded)
	}
}
