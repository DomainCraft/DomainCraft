package renderer

import (
	"testing"

	"github.com/DomainCraft/DomainCraft/pkg/logger"
)

// testMappings mirrors a typical bridge type_mappings.yaml (C#-flavored,
// matching domaincraft-bridge-csharp).
func testMappings() *typeMappings {
	return &typeMappings{
		Types: map[string]string{
			"uuid":    "Guid",
			"int":     "int",
			"bigint":  "long",
			"decimal": "decimal",
			"string":  "string",
		},
		Behaviors:      map[string]string{"set_null": "SetNull", "cascade": "Cascade"},
		ValueTypes:     []string{"Guid", "int", "long", "decimal", "bool"},
		ArrayFormat:    "List<%s>",
		NullableFormat: "?",
		Literals: map[string]LiteralSpec{
			"string":  {Default: `""`},
			"bigint":  {Suffix: "L", Default: "0L"},
			"decimal": {Suffix: "m", Default: "0m"},
			"uuid":    {Parse: "Guid.Parse(%s)", Default: "Guid.NewGuid()"},
			"json":    {Parse: "JsonDocument.Parse(%s, default)", Default: `JsonDocument.Parse("{}")`},
			"enum":    {Parse: "Enum.Parse<%s>(%s)", Member: "%s.%s"},
		},
		Array:       ArrayLiteralSpec{Open: "new %s { ", Close: " }", Empty: "new %s()"},
		ColumnSizes: map[string]string{"string": "255", "uuid": "36"},
		InputTypes:  map[string]string{"datetime": "DatePicker"},
	}
}

func TestBuildBridgeFuncs_NilMappings(t *testing.T) {
	funcs := buildBridgeFuncs(nil, logger.New())
	if _, ok := funcs["languageType"]; ok {
		t.Error("nil mappings should produce no bridge funcs")
	}
}

func TestBuildBridgeFuncs_LanguageType(t *testing.T) {
	fn := buildBridgeFuncs(testMappings(), logger.New())["languageType"].(func(string, bool) string)

	cases := []struct {
		name     string
		dbType   string
		nullable bool
		want     string
	}{
		{"explicit scalar", "uuid", false, "Guid"},
		{"explicit nullable gets suffix", "uuid", true, "Guid?"},
		{"array of mapped inner uses array format", "array(int)", false, "List<int>"},
		{"enum formats via pascalcase", "OrderStatus", false, "OrderStatus"},
		{"nullable enum without enum_nullable stays bare", "OrderStatus", true, "OrderStatus"},
		{"enum array wraps element", "array(OrderStatus)", false, "List<OrderStatus>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fn(c.dbType, c.nullable); got != c.want {
				t.Errorf("languageType(%q, %v) = %q, want %q", c.dbType, c.nullable, got, c.want)
			}
		})
	}

	m := testMappings()
	m.EnumNullable = true
	fnEnumNullable := buildBridgeFuncs(m, logger.New())["languageType"].(func(string, bool) string)
	if got := fnEnumNullable("OrderStatus", true); got != "OrderStatus?" {
		t.Errorf("nullable enum with enum_nullable = %q, want OrderStatus?", got)
	}

	// Unmapped primitive falls back to the raw type name (with a warning).
	fnWarn := buildBridgeFuncs(&typeMappings{Types: map[string]string{"int": "int"}}, logger.New())["languageType"].(func(string, bool) string)
	if got := fnWarn("boolean", false); got != "boolean" {
		t.Errorf("unmapped primitive = %q, want the raw type name as fallback", got)
	}
}

func TestBuildBridgeFuncs_DefaultArrayFormatAndOptionalFuncs(t *testing.T) {
	m := &typeMappings{Types: map[string]string{"int": "int32"}}
	funcs := buildBridgeFuncs(m, logger.New())

	if _, ok := funcs["columnSize"]; ok {
		t.Error("columnSize must be absent when no column_sizes are declared")
	}
	if _, ok := funcs["inputType"]; ok {
		t.Error("inputType must be absent when no input_types are declared")
	}
	arr := funcs["languageType"].(func(string, bool) string)
	if got := arr("array(int)", false); got != "int32[]" {
		t.Errorf("default array format = %q, want int32[]", got)
	}
	if got := arr("int", true); got != "int32" {
		t.Errorf("empty nullable_format must not append a suffix, got %q", got)
	}
}

func TestBuildBridgeFuncs_LiteralValue(t *testing.T) {
	literalValue := buildBridgeFuncs(testMappings(), logger.New())["literalValue"].(func(string, any) string)

	cases := []struct {
		name   string
		dbType string
		value  any
		want   string
	}{
		{"text is JSON-encoded", "string", `hi`, `"hi"`},
		{"numeric suffix", "decimal", "99.99", "99.99m"},
		{"bigint suffix", "bigint", int64(7), "7L"},
		{"uuid parse expression", "uuid", "550e8400-e29b-41d4-a716-446655440000", `Guid.Parse("550e8400-e29b-41d4-a716-446655440000")`},
		{"json parse expression", "json", `{"a":1}`, `JsonDocument.Parse("{\"a\":1}", default)`},
		{"enum parse expression pascalcases the member", "OrderStatus", "draft", `Enum.Parse<OrderStatus>("Draft")`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := literalValue(c.dbType, c.value); got != c.want {
				t.Errorf("literalValue(%q, %v) = %q, want %q", c.dbType, c.value, got, c.want)
			}
		})
	}
}

func TestBuildBridgeFuncs_LiteralDefault(t *testing.T) {
	literalDefault := buildBridgeFuncs(testMappings(), logger.New())["literalDefault"].(func(string) string)

	cases := []struct{ name, dbType, want string }{
		{"declared default expression", "uuid", "Guid.NewGuid()"},
		{"scalar default literal", "decimal", "0m"},
		{"no default spec falls back to default(T)", "boolean", "default(boolean)"},
		{"enum generic zero fallback", "OrderStatus", "(OrderStatus)0"},
		{"empty array literal", "array(int)", "new List<int>()"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := literalDefault(c.dbType); got != c.want {
				t.Errorf("literalDefault(%q) = %q, want %q", c.dbType, got, c.want)
			}
		})
	}

	// An enum may declare its own zero-valued member under its type name.
	m := testMappings()
	m.Literals["OrderStatus"] = LiteralSpec{Zero: "%s.None"}
	withZero := buildBridgeFuncs(m, logger.New())["literalDefault"].(func(string) string)
	if got := withZero("OrderStatus"); got != "OrderStatus.None" {
		t.Errorf("enum zero template = %q, want OrderStatus.None", got)
	}
}

func TestBuildBridgeFuncs_LiteralMember(t *testing.T) {
	literalMember := buildBridgeFuncs(testMappings(), logger.New())["literalMember"].(func(string, string) string)

	if got := literalMember("OrderStatus", "in_transit"); got != "OrderStatus.InTransit" {
		t.Errorf("member = %q, want OrderStatus.InTransit", got)
	}
	if got := literalMember("string", "plain"); got != `"plain"` {
		t.Errorf("non-enum member = %q, want the JSON-encoded value", got)
	}

	m := testMappings()
	m.Literals["enum"] = LiteralSpec{} // Member unset → Type.Value fallback
	fallback := buildBridgeFuncs(m, logger.New())["literalMember"].(func(string, string) string)
	if got := fallback("OrderStatus", "draft"); got != "OrderStatus.Draft" {
		t.Errorf("member fallback = %q, want OrderStatus.Draft", got)
	}
}

func TestBuildBridgeFuncs_ArrayLiteralAndHelpers(t *testing.T) {
	raw := buildBridgeFuncs(testMappings(), logger.New())

	open := raw["arrayLiteralOpen"].(func(string) string)
	if got := open("array(int)"); got != "new List<int> { " {
		t.Errorf("arrayLiteralOpen(array(int)) = %q", got)
	}
	if got := raw["arrayLiteralClose"].(func() string)(); got != " }" {
		t.Errorf("arrayLiteralClose = %q", got)
	}

	isValueType := raw["isValueType"].(func(string) bool)
	if !isValueType("uuid") || !isValueType("bigint") {
		t.Error("isValueType should be true for types mapped into the value_types set")
	}
	if isValueType("string") || isValueType("Unknown") {
		t.Error("isValueType should be false for reference or unmapped types")
	}

	deleteBehaviorName := raw["deleteBehaviorName"].(func(string) string)
	if got := deleteBehaviorName(" Set_Null "); got != "SetNull" {
		t.Errorf("mapped behavior = %q, want SetNull", got)
	}
	if got := deleteBehaviorName("restrict"); got != "Restrict" {
		t.Errorf("unmapped behavior = %q, want the PascalCase fallback Restrict", got)
	}

	columnSize := raw["columnSize"].(func(string) string)
	if got := columnSize("string"); got != "255" {
		t.Errorf("columnSize(string) = %q, want 255", got)
	}
	if got := columnSize("text"); got != "" {
		t.Errorf("columnSize(text) = %q, want empty when undeclared", got)
	}

	inputType := raw["inputType"].(func(string) string)
	if got := inputType("datetime"); got != "DatePicker" {
		t.Errorf("inputType(datetime) = %q, want DatePicker", got)
	}
	if got := inputType("array(datetime)"); got != "DatePicker" {
		t.Errorf("inputType(array(datetime)) = %q, want the inner type's mapping", got)
	}
	if got := inputType("unknown"); got != "Input" {
		t.Errorf("inputType(unknown) = %q, want the Input fallback", got)
	}
}

func TestMergeTypeMappings_AdapterWinsOnConflicts(t *testing.T) {
	base := &typeMappings{
		Types:          map[string]string{"uuid": "Guid", "int": "int"},
		Literals:       map[string]LiteralSpec{"uuid": {Parse: "Base(%s)"}, "decimal": {Suffix: "m"}},
		ArrayFormat:    "IList<%s>",
		NullableFormat: "?",
		EnumNullable:   false,
	}
	overlay := &typeMappings{
		Types:          map[string]string{"int": "Int32"},
		Literals:       map[string]LiteralSpec{"uuid": {Parse: "Adapter(%s)"}},
		NullableFormat: " | null",
		EnumNullable:   true,
	}

	merged := mergeTypeMappings(base, overlay)

	if merged.Types["uuid"] != "Guid" || merged.Types["int"] != "Int32" {
		t.Errorf("types merge = %v, want base kept and adapter override", merged.Types)
	}
	if merged.Literals["uuid"].Parse != "Adapter(%s)" {
		t.Errorf("literals merge: uuid.Parse = %q, want the adapter to win", merged.Literals["uuid"].Parse)
	}
	if merged.Literals["decimal"].Suffix != "m" {
		t.Errorf("literals merge: decimal.Suffix = %q, want the base entry preserved", merged.Literals["decimal"].Suffix)
	}
	if merged.ArrayFormat != "IList<%s>" {
		t.Errorf("array format = %q, want the base value when the adapter declares none", merged.ArrayFormat)
	}
	if merged.NullableFormat != " | null" {
		t.Errorf("nullable format = %q, want the adapter's declared value", merged.NullableFormat)
	}
	if !merged.EnumNullable {
		t.Error("enum_nullable should OR-merge across the chain")
	}
	if mergeLiterals(nil, nil) != nil {
		t.Error("merging two empty literal maps should yield nil")
	}
}
