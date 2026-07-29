package specmeta

import "testing"

func TestIsPrimitive(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"string", true},
		{"text", true},
		{"int", true},
		{"bigint", true},
		{"float", true},
		{"decimal", true},
		{"boolean", true},
		{"date", true},
		{"datetime", true},
		{"uuid", true},
		{"json", true},
		{"jsonb", true},
		{"STRING", false},       // case-sensitive, callers must lowercase
		{"ProductStatus", false}, // enum
		{"enum", false},         // meta-type
		{"array", false},        // meta-type
		{"relation", false},     // meta-type
		{"", false},
	}

	for _, tt := range tests {
		got := IsPrimitive(tt.input)
		if got != tt.want {
			t.Errorf("IsPrimitive(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFieldTypesDerivedCorrectly(t *testing.T) {
	expected := len(PrimitiveFieldTypes) + len(MetaFieldTypes)
	if len(FieldTypes) != expected {
		t.Errorf("len(FieldTypes) = %d, want %d (= PrimitiveFieldTypes + MetaFieldTypes)", len(FieldTypes), expected)
	}

	primitiveSet := SliceToSet(PrimitiveFieldTypes)
	for _, ft := range FieldTypes {
		if !primitiveSet[ft] && ft != "relation" && ft != "array" && ft != "enum" {
			t.Errorf("FieldTypes contains unexpected type %q", ft)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"int", true},
		{"bigint", true},
		{"float", true},
		{"decimal", true},
		{"string", false},
		{"boolean", false},
		{"datetime", false},
		{"uuid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsNumeric(tt.input); got != tt.want {
			t.Errorf("IsNumeric(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsStringType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"string", true},
		{"text", true},
		{"int", false},
		{"boolean", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsStringType(tt.input); got != tt.want {
			t.Errorf("IsStringType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsBooleanType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"boolean", true},
		{"string", false},
		{"int", false},
		{"Boolean", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsBooleanType(tt.input); got != tt.want {
			t.Errorf("IsBooleanType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsIndexType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"btree", true},
		{"hash", true},
		{"gist", true},
		{"gin", true},
		{"brin", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsIndexType(tt.input); got != tt.want {
			t.Errorf("IsIndexType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsCacheProvider(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"redis", true},
		{"memcached", true},
		{"in-memory", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsCacheProvider(tt.input); got != tt.want {
			t.Errorf("IsCacheProvider(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsMultiTenancyMode(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"column", true},
		{"schema", true},
		{"database", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsMultiTenancyMode(tt.input); got != tt.want {
			t.Errorf("IsMultiTenancyMode(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsPermissionKey(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"read", true},
		{"create", true},
		{"update", true},
		{"delete", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsPermissionKey(tt.input); got != tt.want {
			t.Errorf("IsPermissionKey(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsStringValidationModifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"min", true},
		{"max", true},
		{"email", true},
		{"url", true},
		{"ipv4", true},
		{"regex", true},
		{"gte", false},
		{"gt", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsStringValidationModifier(tt.input); got != tt.want {
			t.Errorf("IsStringValidationModifier(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsNumericValidationModifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"gte", true},
		{"gt", true},
		{"lte", true},
		{"lt", true},
		{"min", false},
		{"max", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsNumericValidationModifier(tt.input); got != tt.want {
			t.Errorf("IsNumericValidationModifier(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFindAuthEntity(t *testing.T) {
	tests := []struct {
		name         string
		entityOrder  []string
		entityFields map[string][]string
		want         string
	}{
		{
			name:         "found",
			entityOrder:  []string{"Product", "User"},
			entityFields: map[string][]string{"Product": {"id", "name"}, "User": {"id", "email", "password"}},
			want:         "User",
		},
		{
			name:         "not found",
			entityOrder:  []string{"Product"},
			entityFields: map[string][]string{"Product": {"id", "name"}},
			want:         "",
		},
		{
			name:         "empty",
			entityOrder:  []string{},
			entityFields: map[string][]string{},
			want:         "",
		},
		{
			name:         "case insensitive",
			entityOrder:  []string{"User"},
			entityFields: map[string][]string{"User": {"Email", "Password"}},
			want:         "User",
		},
		{
			name:         "missing one field",
			entityOrder:  []string{"User"},
			entityFields: map[string][]string{"User": {"email"}},
			want:         "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindAuthEntity(tt.entityOrder, tt.entityFields)
			if got != tt.want {
				t.Errorf("FindAuthEntity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSortDirection(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"asc", true},
		{"desc", true},
		{"ASC", false},
		{"ascending", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsSortDirection(tt.input); got != tt.want {
			t.Errorf("IsSortDirection(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHasEmailAndPassword(t *testing.T) {
	tests := []struct {
		name       string
		fieldNames []string
		want       bool
	}{
		{"both present", []string{"id", "email", "password", "name"}, true},
		{"case insensitive", []string{"id", "Email", "Password"}, true},
		{"missing password", []string{"id", "email"}, false},
		{"missing email", []string{"id", "password"}, false},
		{"empty", []string{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasEmailAndPassword(tt.fieldNames); got != tt.want {
				t.Errorf("HasEmailAndPassword(%v) = %v, want %v", tt.fieldNames, got, tt.want)
			}
		})
	}
}

func TestFuncDefaultsContainsExpectedEntries(t *testing.T) {
	if _, ok := FuncDefaults["now"]; !ok {
		t.Error("FuncDefaults missing 'now'")
	}
	if _, ok := FuncDefaults["uuid"]; !ok {
		t.Error("FuncDefaults missing 'uuid'")
	}
	if types := FuncDefaults["now"]; len(types) != 2 || types[0] != "date" || types[1] != "datetime" {
		t.Errorf("FuncDefaults[now] = %v, want [date datetime]", types)
	}
	if types := FuncDefaults["uuid"]; len(types) != 1 || types[0] != "uuid" {
		t.Errorf("FuncDefaults[uuid] = %v, want [uuid]", types)
	}
}

func TestIsDatabase(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"postgresql", true},
		{"mysql", true},
		{"sqlite", true},
		{"mssql", true},
		{"mongodb", true},
		{"PostgreSQL", false}, // case-sensitive
		{"oracle", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsDatabase(tt.input); got != tt.want {
			t.Errorf("IsDatabase(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsAPIStyle(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"rest", true},
		{"graphql", true},
		{"grpc", true},
		{"REST", false}, // case-sensitive
		{"soap", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsAPIStyle(tt.input); got != tt.want {
			t.Errorf("IsAPIStyle(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsAuthType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"jwt", true},
		{"none", true},
		{"JWT", false},  // case-sensitive
		{"oauth", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsAuthType(tt.input); got != tt.want {
			t.Errorf("IsAuthType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBuildEntityFields(t *testing.T) {
	entityOrder := []string{"User", "Product"}
	getFields := func(name string) []string {
		switch name {
		case "User":
			return []string{"id", "email", "name"}
		case "Product":
			return []string{"id", "title", "price"}
		default:
			return nil
		}
	}

	result := BuildEntityFields(entityOrder, getFields)

	if len(result) != 2 {
		t.Fatalf("BuildEntityFields returned %d entries, want 2", len(result))
	}
	if fields := result["User"]; len(fields) != 3 || fields[0] != "id" || fields[1] != "email" {
		t.Errorf("User fields = %v, want [id email name]", fields)
	}
	if fields := result["Product"]; len(fields) != 3 || fields[0] != "id" || fields[2] != "price" {
		t.Errorf("Product fields = %v, want [id title price]", fields)
	}
}

func TestBuildEntityFieldsEmptyOrder(t *testing.T) {
	result := BuildEntityFields(nil, func(name string) []string { return nil })
	if len(result) != 0 {
		t.Errorf("BuildEntityFields(nil) returned %d entries, want 0", len(result))
	}
}
