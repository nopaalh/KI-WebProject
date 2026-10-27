package models

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func parseModelSchema(t *testing.T, model any) *schema.Schema {
	t.Helper()
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse model schema: %v", err)
	}
	return s
}

func TestPatientProfileSchema(t *testing.T) {
	s := parseModelSchema(t, &PatientProfile{})
	if s.Table != "patient_profiles" {
		t.Fatalf("unexpected table: %s", s.Table)
	}
	for _, name := range []string{"weight_kg", "height_cm", "age"} {
		field := s.LookUpField(name)
		if field == nil || !field.NotNull {
			t.Fatalf("missing required plaintext measurement: %s", name)
		}
	}
	for _, name := range []string{"nik_aes", "nik_des", "nik_rc4", "bpjs_number_aes", "bpjs_number_des", "bpjs_number_rc4"} {
		field := s.LookUpField(name)
		if field == nil || field.DataType != schema.Bytes || !field.NotNull {
			t.Fatalf("expected required binary ciphertext column: %s", name)
		}
	}
	for _, name := range []string{"nik", "bpjs_number"} {
		if s.LookUpField(name) != nil {
			t.Fatalf("unexpected plaintext identity column: %s", name)
		}
	}

	checks := s.ParseCheckConstraints()
	for name, expression := range map[string]string{
		"patient_profiles_patient_role_check": "user_role = 'patient'",
		"patient_profiles_weight_check":       "weight_kg > 0",
		"patient_profiles_height_check":       "height_cm > 0",
		"patient_profiles_age_check":          "age >= 0",
		"patient_profiles_nik_aes_check":      "octet_length(nik_aes) > 0",
		"patient_profiles_nik_des_check":      "octet_length(nik_des) > 0",
		"patient_profiles_nik_rc4_check":      "octet_length(nik_rc4) > 0",
		"patient_profiles_bpjs_aes_check":     "octet_length(bpjs_number_aes) > 0",
		"patient_profiles_bpjs_des_check":     "octet_length(bpjs_number_des) > 0",
		"patient_profiles_bpjs_rc4_check":     "octet_length(bpjs_number_rc4) > 0",
	} {
		if check, ok := checks[name]; !ok || check.Constraint != expression {
			t.Errorf("missing or incorrect check %s: %+v", name, check)
		}
	}

	uniqueUser := false
	for _, index := range s.ParseIndexes() {
		if index.Class == "UNIQUE" && len(index.Fields) == 1 && index.Fields[0].DBName == "user_id" {
			uniqueUser = true
		}
	}
	if !uniqueUser {
		t.Fatal("a user must have at most one patient profile")
	}
	if s.LookUpField("UserRole").DefaultValue != RolePatient {
		t.Fatal("profile role must default to patient")
	}
}

func TestPatientProfileReferencesPatientUser(t *testing.T) {
	s := parseModelSchema(t, &User{})
	rel := s.Relationships.Relations["PatientProfile"]
	if rel == nil || rel.Type != schema.HasOne {
		t.Fatal("expected a has-one patient profile association")
	}
	constraint := rel.ParseConstraint()
	if constraint == nil || constraint.Schema.Table != "patient_profiles" || constraint.ReferenceSchema.Table != "users" {
		t.Fatalf("unexpected foreign key: %+v", constraint)
	}
	if len(constraint.ForeignKeys) != 2 || len(constraint.References) != 2 {
		t.Fatal("foreign key must bind both user identity and role")
	}
	for i, pair := range [][2]string{{"user_id", "id"}, {"user_role", "role"}} {
		if constraint.ForeignKeys[i].DBName != pair[0] || constraint.References[i].DBName != pair[1] {
			t.Fatalf("unexpected foreign key mapping at %d", i)
		}
	}
	if constraint.OnUpdate != "RESTRICT" || constraint.OnDelete != "RESTRICT" {
		t.Fatal("must not cascade user role changes or deletion into a profile")
	}

	for _, index := range s.ParseIndexes() {
		if index.Name == "idx_users_id_role" && index.Class == "UNIQUE" && len(index.Fields) == 2 &&
			index.Fields[0].DBName == "id" && index.Fields[1].DBName == "role" {
			return
		}
	}
	t.Fatal("composite foreign key needs a matching unique key on users(id, role)")
}

func TestPatientProfileJSONDoesNotExposeCiphertext(t *testing.T) {
	profile := PatientProfile{
		ID: 1, UserID: 2, UserRole: RolePatient,
		WeightKG: 65.5, HeightCM: 170, Age: 21,
		NIKAES: []byte("synthetic-envelope"), NIKDES: []byte("synthetic-envelope"), NIKRC4: []byte("synthetic-envelope"),
		BPJSNumberAES: []byte("synthetic-envelope"), BPJSNumberDES: []byte("synthetic-envelope"), BPJSNumberRC4: []byte("synthetic-envelope"),
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for key := range fields {
		if strings.Contains(key, "NIK") || strings.Contains(key, "BPJS") || key == "UserRole" {
			t.Fatalf("internal field exposed by JSON: %s", key)
		}
	}
}
