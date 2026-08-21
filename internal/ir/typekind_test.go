package ir

import "testing"

func TestIRFieldTypePredicates(t *testing.T) {
	cases := []struct {
		dbType                         string
		isUuid, isText, isInt, isFloat bool
		isBool, isDate, isDt, isJson   bool
		isNumeric, isEnum              bool
		enumName                       string
	}{
		{dbType: "uuid", isUuid: true, enumName: "uuid"},
		{dbType: "string", isText: true, enumName: "string"},
		{dbType: "text", isText: true, enumName: "text"},
		{dbType: "int", isInt: true, isNumeric: true, enumName: "int"},
		{dbType: "bigint", isInt: true, isNumeric: true, enumName: "bigint"},
		{dbType: "float", isFloat: true, isNumeric: true, enumName: "float"},
		{dbType: "decimal", isFloat: true, isNumeric: true, enumName: "decimal"},
		{dbType: "boolean", isBool: true, enumName: "boolean"},
		{dbType: "date", isDate: true, enumName: "date"},
		{dbType: "datetime", isDt: true, enumName: "datetime"},
		{dbType: "json", isJson: true, enumName: "json"},
		{dbType: "jsonb", isJson: true, enumName: "jsonb"},
		{dbType: "Status", isEnum: true, enumName: "Status"},
		{dbType: "array(int)", enumName: "int"},
		{dbType: "array(Status)", isEnum: true, enumName: "Status"},
	}
	for _, c := range cases {
		f := IRField{DatabaseType: c.dbType}
		if got := f.IsUuid(); got != c.isUuid {
			t.Errorf("%q IsUuid() = %v, want %v", c.dbType, got, c.isUuid)
		}
		if got := f.IsText(); got != c.isText {
			t.Errorf("%q IsText() = %v, want %v", c.dbType, got, c.isText)
		}
		if got := f.IsInteger(); got != c.isInt {
			t.Errorf("%q IsInteger() = %v, want %v", c.dbType, got, c.isInt)
		}
		if got := f.IsFloat(); got != c.isFloat {
			t.Errorf("%q IsFloat() = %v, want %v", c.dbType, got, c.isFloat)
		}
		if got := f.IsNumeric(); got != c.isNumeric {
			t.Errorf("%q IsNumeric() = %v, want %v", c.dbType, got, c.isNumeric)
		}
		if got := f.IsBoolean(); got != c.isBool {
			t.Errorf("%q IsBoolean() = %v, want %v", c.dbType, got, c.isBool)
		}
		if got := f.IsDate(); got != c.isDate {
			t.Errorf("%q IsDate() = %v, want %v", c.dbType, got, c.isDate)
		}
		if got := f.IsDateTime(); got != c.isDt {
			t.Errorf("%q IsDateTime() = %v, want %v", c.dbType, got, c.isDt)
		}
		if got := f.IsJson(); got != c.isJson {
			t.Errorf("%q IsJson() = %v, want %v", c.dbType, got, c.isJson)
		}
		if got := f.IsEnum(); got != c.isEnum {
			t.Errorf("%q IsEnum() = %v, want %v", c.dbType, got, c.isEnum)
		}
		if got := f.EnumTypeName(); got != c.enumName {
			t.Errorf("%q EnumTypeName() = %q, want %q", c.dbType, got, c.enumName)
		}
	}
}

func TestIRFieldSpecificTypePredicates(t *testing.T) {
	cases := []struct {
		dbType                        string
		isString, isBigInt, isDecimal bool
		isJsonB                       bool
	}{
		{dbType: "string", isString: true},
		{dbType: "text", isString: false},
		{dbType: "int"},
		{dbType: "bigint", isBigInt: true},
		{dbType: "float"},
		{dbType: "decimal", isDecimal: true},
		{dbType: "json"},
		{dbType: "jsonb", isJsonB: true},
	}
	for _, c := range cases {
		f := IRField{DatabaseType: c.dbType}
		if got := f.IsString(); got != c.isString {
			t.Errorf("%q IsString() = %v, want %v", c.dbType, got, c.isString)
		}
		if got := f.IsBigInt(); got != c.isBigInt {
			t.Errorf("%q IsBigInt() = %v, want %v", c.dbType, got, c.isBigInt)
		}
		if got := f.IsDecimal(); got != c.isDecimal {
			t.Errorf("%q IsDecimal() = %v, want %v", c.dbType, got, c.isDecimal)
		}
		if got := f.IsJsonB(); got != c.isJsonB {
			t.Errorf("%q IsJsonB() = %v, want %v", c.dbType, got, c.isJsonB)
		}
	}
}

func TestIRFieldIsPatchable(t *testing.T) {
	cases := []struct {
		name string
		f    IRField
		want bool
	}{
		{"scalar", IRField{Name: "title", DatabaseType: "string"}, true},
		{"single-fk", IRField{Name: "order", DatabaseType: "uuid", IsRelation: true}, true},
		{"collection-fk", IRField{Name: "items", DatabaseType: "uuid", IsRelation: true, IsMany: true}, false},
		{"array", IRField{Name: "tags", DatabaseType: "array(string)"}, false},
		{"enum", IRField{Name: "status", DatabaseType: "Status"}, false},
		{"json", IRField{Name: "meta", DatabaseType: "json"}, false},
		{"primary", IRField{Name: "id", DatabaseType: "uuid", IsPrimary: true}, false},
		{"readonly", IRField{Name: "slug", DatabaseType: "string", IsReadonly: true}, false},
		{"sensitive", IRField{Name: "password", DatabaseType: "string"}, false},
		{"feature", IRField{Name: "version", DatabaseType: "int"}, false},
	}
	for _, c := range cases {
		if got := c.f.IsPatchable(); got != c.want {
			t.Errorf("%s IsPatchable() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIRFieldIsTextOnly(t *testing.T) {
	if !(IRField{DatabaseType: "text"}).IsTextOnly() {
		t.Error("text should be IsTextOnly")
	}
	if (IRField{DatabaseType: "string"}).IsTextOnly() {
		t.Error("string should not be IsTextOnly")
	}
}

func TestIREntityTableNameAndForeignKeyColumnName(t *testing.T) {
	e := IREntity{Name: "IPv4Rule", NamePlural: "IPv4Rules"}
	if got := e.TableName(); got != "i_pv4rules" {
		t.Errorf("TableName() = %q, want %q (acronyms must match ToDatabaseColumnName)", got, "i_pv4rules")
	}

	r := IRRelation{FieldName: "ipv4Address"}
	if got := r.ForeignKeyColumnName(); got != "ipv4address_id" {
		t.Errorf("ForeignKeyColumnName() = %q, want %q (must match ToDatabaseColumnName, not template snakecase)", got, "ipv4address_id")
	}
	// A field already suffixed with Id must not get a double Id.
	if got := (IRRelation{FieldName: "orderId"}).ForeignKeyColumnName(); got != "order_id" {
		t.Errorf("ForeignKeyColumnName(orderId) = %q, want order_id", got)
	}
}

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"uuid":          "uuid",
		"string":        "text",
		"text":          "text",
		"int":           "int",
		"bigint":        "bigint",
		"float":         "float",
		"decimal":       "decimal",
		"boolean":       "bool",
		"date":          "date",
		"datetime":      "datetime",
		"json":          "json",
		"jsonb":         "json",
		"Status":        "enum",
		"array(int)":    "array",
		"array(Status)": "array",
	}
	for dbType, want := range cases {
		if got := KindOf(dbType); got != want {
			t.Errorf("KindOf(%q) = %q, want %q", dbType, got, want)
		}
	}
	if got := (SeedValue{DBType: "array(uuid)"}).Kind(); got != "array" {
		t.Errorf("SeedValue.Kind() = %q, want array", got)
	}
	if got := (SeedValue{DBType: "array(uuid)"}).ElementType(); got != "uuid" {
		t.Errorf("SeedValue.ElementType() = %q, want uuid", got)
	}
}

func TestIRPermissionsIsPublic(t *testing.T) {
	p := &IRPermissions{Read: []string{"*"}, Create: []string{"Admin"}, Update: nil, Delete: []string{}}
	if !p.IsPublic("read") {
		t.Error("IsPublic(read) = false, want true")
	}
	if p.IsPublic("create") {
		t.Error("IsPublic(create) = true, want false")
	}
	if p.IsPublic("update") {
		t.Error("IsPublic(update) = true, want false")
	}
	if p.IsPublic("delete") {
		t.Error("IsPublic(delete) = true, want false")
	}
	var nilPerms *IRPermissions
	if nilPerms.IsPublic("read") {
		t.Error("nil IsPublic(read) = true, want false")
	}
}

func TestIREndpointAuthPolicy(t *testing.T) {
	cases := map[string]string{
		"list":   "Read",
		"get":    "Read",
		"create": "Create",
		"update": "Update",
		"patch":  "Update",
		"delete": "Delete",
	}
	for op, want := range cases {
		if got := (IREndpoint{Operation: op}).AuthPolicy(); got != want {
			t.Errorf("AuthPolicy(%q) = %q, want %q", op, got, want)
		}
	}
}
