package templatefuncs

import (
	"reflect"
	"testing"

	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

func TestContainsAndHasPrefixArgOrder(t *testing.T) {
	// The reversed argument order (substr first) must be preserved.
	if !Contains("ell", "hello") {
		t.Error("Contains('ell', 'hello') = false, want true")
	}
	if !HasPrefix("he", "hello") {
		t.Error("HasPrefix('he', 'hello') = false, want true")
	}
}

func TestAdd(t *testing.T) {
	got := Add(1, 2, 3)
	if got != 6 {
		t.Errorf("Add(1,2,3) = %d, want 6", got)
	}
}

func TestAtoi(t *testing.T) {
	if got := Atoi("42"); got != 42 {
		t.Errorf("Atoi('42') = %d, want 42", got)
	}
	if got := Atoi("nope"); got != 0 {
		t.Errorf("Atoi('nope') = %d, want 0", got)
	}
}

func TestDefault(t *testing.T) {
	if got := Default("fallback", ""); got != "fallback" {
		t.Errorf("Default(fallback, empty) = %v, want fallback", got)
	}
	if got := Default("fallback", 0); got != "fallback" {
		t.Errorf("Default(fallback, 0) = %v, want fallback", got)
	}
	if got := Default("fallback", "value"); got != "value" {
		t.Errorf("Default(fallback, value) = %v, want value", got)
	}
}

func TestDict(t *testing.T) {
	got := Dict("a", 1, "b", "two")
	want := map[string]interface{}{"a": 1, "b": "two"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Dict = %v, want %v", got, want)
	}
	// Odd trailing key gets empty string.
	got = Dict("k")
	if !reflect.DeepEqual(got, map[string]interface{}{"k": ""}) {
		t.Errorf("Dict(k) = %v, want {k: ''}", got)
	}
}

func TestSet(t *testing.T) {
	d := map[string]interface{}{}
	got := Set(d, "x", 7)
	if got["x"] != 7 {
		t.Errorf("Set = %v, want x=7", got)
	}
}

func TestListAppend(t *testing.T) {
	l := List()
	if len(l) != 0 {
		t.Errorf("List() = %v, want empty", l)
	}
	l = Append(l, "a")
	l = Append(l, 2)
	if !reflect.DeepEqual(l, []interface{}{"a", 2}) {
		t.Errorf("Append chain = %v, want [a 2]", l)
	}
	// Append also works on typed slices (converted via reflection).
	if got := Append([]string{"x", "y"}, "z"); !reflect.DeepEqual(got, []interface{}{"x", "y", "z"}) {
		t.Errorf("Append([]string) = %v, want [x y z]", got)
	}
}

func TestSplitList(t *testing.T) {
	got := SplitList(" ", "a b c")
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("SplitList = %v, want [a b c]", got)
	}
}

func TestJoin(t *testing.T) {
	if got := Join(",", []string{"a", "b"}); got != "a,b" {
		t.Errorf("Join = %q, want a,b", got)
	}
	if got := Join("-", []interface{}{1, 2, 3}); got != "1-2-3" {
		t.Errorf("Join([]interface{}) = %q, want 1-2-3", got)
	}
	if got := Join(",", nil); got != "" {
		t.Errorf("Join(nil) = %q, want empty", got)
	}
}

func TestKindOf(t *testing.T) {
	if got := KindOf("x"); got != "string" {
		t.Errorf("KindOf('x') = %q, want string", got)
	}
	if got := KindOf([]int{1}); got != "slice" {
		t.Errorf("KindOf([]int{1}) = %q, want slice", got)
	}
}

func TestToString(t *testing.T) {
	if got := ToString("s"); got != "s" {
		t.Errorf("ToString(string) = %q, want s", got)
	}
	if got := ToString(42); got != "42" {
		t.Errorf("ToString(42) = %q, want 42", got)
	}
}

func TestAny(t *testing.T) {
	if Any("", 0, false, nil) {
		t.Error("Any(all empty) = true, want false")
	}
	if !Any("", "x") {
		t.Error("Any('', 'x') = false, want true")
	}
}

func TestFuncMapContainsAllExpected(t *testing.T) {
	fm := FuncMap()
	expected := []string{
		"add", "any", "append", "atoi", "base", "camelcase", "contains",
		"default", "dict", "fkName", "hasPrefix", "humanize", "join",
		"kebabcase", "kindIs", "kindOf", "list", "lower", "lowercase",
		"now", "pascalcase", "pluralize", "set", "snakecase", "splitList",
		"title", "toString", "upper", "uppercase",
	}
	for _, name := range expected {
		if _, ok := fm[name]; !ok {
			t.Errorf("FuncMap missing %q", name)
		}
	}
}

func TestPascalCamelCase(t *testing.T) {
	if got := PascalCase("first_name"); got != "FirstName" {
		t.Errorf("PascalCase(first_name) = %q, want FirstName", got)
	}
	if got := CamelCase("FirstName"); got != "firstName" {
		t.Errorf("CamelCase(FirstName) = %q, want firstName", got)
	}
	// Acronyms must stay split: HTTPPort → HttpPort, not Httpport.
	if got := PascalCase("HTTPPort"); got != "HttpPort" {
		t.Errorf("PascalCase(HTTPPort) = %q, want HttpPort", got)
	}
}

func TestHumanizePluralizeFKName(t *testing.T) {
	if got := Humanize("createdAt"); got != "created At" {
		t.Errorf("Humanize(createdAt) = %q, want 'created At'", got)
	}
	if got := textutil.Pluralize("category"); got != "categories" {
		t.Errorf("pluralize(category) = %q, want categories", got)
	}
	if got := textutil.FKName("Category"); got != "CategoryId" {
		t.Errorf("fkName(Category) = %q, want CategoryId", got)
	}
	if got := textutil.FKName("CategoryId"); got != "CategoryId" {
		t.Errorf("fkName(CategoryId) = %q, want CategoryId", got)
	}
}

func TestSnakeKebabCase(t *testing.T) {
	fm := FuncMap()
	snake := fm["snakecase"].(func(string) string)
	kebab := fm["kebabcase"].(func(string) string)
	cases := []struct {
		in, snake, kebab string
	}{
		{"FirstName", "first_name", "first-name"},
		{"HTTPServer", "http_server", "http-server"},
		{"NoHTTPS", "no_https", "no-https"},
		{"GO_PATH", "go_path", "go-path"},
		{"GO PATH", "go_path", "go-path"},
		{"GO-PATH", "go_path", "go-path"},
		{"http2xx", "http_2xx", "http-2xx"},
		{"HTTP20xOK", "http_20x_ok", "http-20x-ok"},
		{"Duration2m3s", "duration_2m3s", "duration-2m3s"},
		{"Bld4Floor3rd", "bld4_floor_3rd", "bld4-floor-3rd"},
		{"IPv4Address", "i_pv4_address", "i-pv4-address"},
	}
	for _, c := range cases {
		if got := snake(c.in); got != c.snake {
			t.Errorf("snakecase(%q) = %q, want %q", c.in, got, c.snake)
		}
		if got := kebab(c.in); got != c.kebab {
			t.Errorf("kebabcase(%q) = %q, want %q", c.in, got, c.kebab)
		}
	}
}
