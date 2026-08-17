package ir

import "testing"

func entityWithFields(fields ...IRField) IREntity {
	return IREntity{Name: "Order", Fields: fields}
}

func TestIRField_IsSensitive(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"password", true},
		{"Password", true},
		{"PASSWORD", true},
		{"passwordHash", false},
		{"email", false},
	}
	for _, tc := range cases {
		f := IRField{Name: tc.name}
		if got := f.IsSensitive(); got != tc.want {
			t.Errorf("IsSensitive(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIRField_IsFeatureField(t *testing.T) {
	if !(IRField{Name: "createdAt"}).IsFeatureField() {
		t.Error("createdAt should be a feature field")
	}
	if !(IRField{Name: "version"}).IsFeatureField() {
		t.Error("version should be a feature field")
	}
	if (IRField{Name: "title"}).IsFeatureField() {
		t.Error("title should not be a feature field")
	}
}

func TestIRField_IsConcurrencyToken(t *testing.T) {
	if !(IRField{Name: "version"}).IsConcurrencyToken() {
		t.Error("version should be a concurrency token")
	}
	if (IRField{Name: "createdAt"}).IsConcurrencyToken() {
		t.Error("createdAt should not be a concurrency token")
	}
}

func TestIREntity_ReadFields(t *testing.T) {
	e := entityWithFields(
		IRField{Name: "id", IsPrimary: true, DatabaseType: "uuid"},
		IRField{Name: "title", DatabaseType: "string"},
		IRField{Name: "password", DatabaseType: "string"},    // sensitive -> excluded
		IRField{Name: "internalNotes", IsHidden: true},       // hidden -> excluded
		IRField{Name: "balance", IsReadonly: true},           // readonly -> readable
		IRField{Name: "createdAt", DatabaseType: "datetime"}, // feature -> readable
	)
	got := e.ReadFields()
	if len(got) != 4 {
		t.Fatalf("got %d read fields, want 4 (id, title, balance, createdAt): %+v", len(got), names(got))
	}
	for _, f := range got {
		if f.Name == "password" || f.Name == "internalNotes" {
			t.Errorf("field %q must not be readable", f.Name)
		}
	}
}

func TestIREntity_CreateFields(t *testing.T) {
	e := entityWithFields(
		IRField{Name: "id", IsPrimary: true},
		IRField{Name: "title", DatabaseType: "string"},
		IRField{Name: "password", DatabaseType: "string"},
		IRField{Name: "balance", IsReadonly: true},
		IRField{Name: "createdAt", DatabaseType: "datetime"},
		IRField{Name: "internalNotes", IsHidden: true}, // hidden but client-settable -> kept
	)
	got := e.CreateFields()
	if len(got) != 2 {
		t.Fatalf("got %d create fields, want 2 (title, internalNotes): %+v", len(got), names(got))
	}
	for _, f := range got {
		if f.Name != "title" && f.Name != "internalNotes" {
			t.Errorf("unexpected create field %q", f.Name)
		}
	}
}

func TestIREntity_UpdateFieldsIncludesVersion(t *testing.T) {
	e := entityWithFields(
		IRField{Name: "id", IsPrimary: true},
		IRField{Name: "title", DatabaseType: "string"},
		IRField{Name: "createdAt", DatabaseType: "datetime"}, // feature -> excluded
		IRField{Name: "version", DatabaseType: "int"},        // concurrency token -> kept
	)
	got := e.UpdateFields()
	if len(got) != 2 {
		t.Fatalf("got %d update fields, want 2 (title, version): %+v", len(got), names(got))
	}
	foundVersion := false
	for _, f := range got {
		if f.Name == "version" {
			foundVersion = true
		}
		if f.Name == "createdAt" {
			t.Error("createdAt must not be in update fields")
		}
	}
	if !foundVersion {
		t.Error("version must be in update fields")
	}
}

func TestIREntity_PatchFields(t *testing.T) {
	e := entityWithFields(
		IRField{Name: "id", IsPrimary: true},
		IRField{Name: "title", DatabaseType: "string"},                    // scalar -> patchable
		IRField{Name: "categoryId", DatabaseType: "uuid", IsRelation: true}, // single FK -> patchable
		IRField{Name: "tags", DatabaseType: "array(string)"},              // array -> excluded
		IRField{Name: "status", DatabaseType: "OrderStatus"},              // enum -> excluded
		IRField{Name: "meta", DatabaseType: "jsonb"},                      // json -> excluded
		IRField{Name: "password", DatabaseType: "string"},                 // sensitive -> excluded
		IRField{Name: "balance", IsReadonly: true},                        // readonly -> excluded
		IRField{Name: "createdAt", DatabaseType: "datetime"},              // feature -> excluded
		IRField{Name: "version", DatabaseType: "int"},                     // concurrency token -> kept
	)
	got := e.PatchFields()
	want := []string{"title", "categoryId", "version"}
	if len(got) != len(want) {
		t.Fatalf("got %d patch fields, want %d (%v): %+v", len(got), len(want), want, names(got))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("patch field[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestIREntity_PatchFieldsNoVersionWithoutLock(t *testing.T) {
	e := entityWithFields(
		IRField{Name: "id", IsPrimary: true},
		IRField{Name: "title", DatabaseType: "string"},
		IRField{Name: "createdAt", DatabaseType: "datetime"}, // no version field declared
	)
	got := e.PatchFields()
	if len(got) != 1 || got[0].Name != "title" {
		t.Fatalf("got %v, want just [title] (no version when lock absent)", names(got))
	}
}

func names(fields []IRField) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.Name
	}
	return out
}
