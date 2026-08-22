// Package templatefuncs provides the small set of template functions that the
// core exposes to bridge templates. Keeping this set small and stdlib-backed
// reduces the dependency graph while preserving the exact call semantics
// (including reversed argument order for contains/hasPrefix and sep-first
// join) that the templates rely on.
package templatefuncs

import (
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// FuncMap returns the template functions registered by the core on top of
// text/template's built-ins. These are the functions the shipped bridges
// actually call; anything else should be a hard render error rather than a
// silently missing function.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"add":       Add,
		"any":       Any,
		"append":    Append,
		"atoi":      Atoi,
		"base":      path.Base,
		"camelcase": CamelCase,
		"contains":  Contains,
		"default":   Default,
		"dict":      Dict,
		"fkName":    textutil.FKName,
		"hasPrefix": HasPrefix,
		"humanize":  Humanize,
		"join":      Join,
		"kebabcase": textutil.KebabCase,
		"kindIs":    KindIs,
		"kindOf":    KindOf,
		"list":      List,
		"lower":     strings.ToLower,
		"lowercase": strings.ToLower,
		"now":       time.Now,
		"pascalcase": PascalCase,
		"pluralize": textutil.Pluralize,
		"set":       Set,
		"snakecase": textutil.SnakeCase,
		"splitList": SplitList,
		"title":     strings.Title,
		"toString":  ToString,
		"upper":     strings.ToUpper,
		"uppercase": strings.ToUpper,
	}
}

// Add sums all arguments as int64.
func Add(i ...interface{}) int64 {
	var a int64
	for _, b := range i {
		a += toInt64(b)
	}
	return a
}

// Any reports whether any argument is non-empty.
func Any(v ...interface{}) bool {
	for _, val := range v {
		if !empty(val) {
			return true
		}
	}
	return false
}

// Atoi parses s as an integer, returning 0 on error.
func Atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// Append appends v to a slice (any slice kind) and returns the result as
// []interface{}. It panics on a non-slice, which text/template surfaces as a
// render error.
func Append(list interface{}, v interface{}) []interface{} {
	val := reflect.ValueOf(list)
	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		l := val.Len()
		nl := make([]interface{}, l)
		for i := range l {
			nl[i] = val.Index(i).Interface()
		}
		return append(nl, v)
	default:
		panic("append: not a slice")
	}
}

// Contains reports whether str contains substr. Note the reversed argument
// order (substr first) that templates rely on when piping a value in.
func Contains(substr, str string) bool {
	return strings.Contains(str, substr)
}

// PascalCase converts v to PascalCase via textutil. It coerces any value to a
// string first (stringArg) so bridges can pipe the IR's named string types
// (e.g. FilterOp) without tripping over Go's nominal typing.
func PascalCase(v any) string {
	return textutil.PascalCase(stringArg(v))
}

// CamelCase converts v to camelCase via textutil (same coercion as PascalCase).
func CamelCase(v any) string {
	return textutil.CamelCase(stringArg(v))
}

// Humanize splits an identifier into words ("createdAt" -> "created At").
func Humanize(name string) string {
	return strings.Join(textutil.SplitIdentifier(name), " ")
}

// stringArg coerces a template argument to its string form, mirroring the
// renderer helper it replaced: strings pass through, fmt.Stringer uses
// String(), everything else is formatted with fmt.Sprint.
func stringArg(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

// Default returns d unless the first given value is empty (zero values are
// considered unset).
func Default(d interface{}, given ...interface{}) interface{} {
	if empty(given) || empty(given[0]) {
		return d
	}
	return given[0]
}

// Dict builds a map[string]interface{} from key/value pairs. An odd trailing
// key is assigned an empty string.
func Dict(v ...interface{}) map[string]interface{} {
	dict := make(map[string]interface{}, len(v)/2+1)
	for i := 0; i < len(v); i += 2 {
		key := ToString(v[i])
		if i+1 >= len(v) {
			dict[key] = ""
			continue
		}
		dict[key] = v[i+1]
	}
	return dict
}

// HasPrefix reports whether str starts with prefix. Note the reversed
// argument order (prefix first) that templates rely on when piping a value in.
func HasPrefix(prefix, str string) bool {
	return strings.HasPrefix(str, prefix)
}

// Join joins the elements of v (a []string, []interface{}, or any slice) with
// sep (sep comes first when called as a pipeline).
func Join(sep string, v interface{}) string {
	return strings.Join(strslice(v), sep)
}

// KindIs reports whether src's reflect.Kind equals target (e.g. "slice").
func KindIs(target string, src interface{}) bool {
	return target == KindOf(src)
}

// List returns its arguments as a []interface{}. With no arguments it yields
// an empty slice, which is the common "start a list, then append" idiom in
// templates.
func List(v ...interface{}) []interface{} {
	return v
}

// KindOf returns the reflect.Kind name of src (e.g. "string", "slice").
func KindOf(src interface{}) string {
	return reflect.ValueOf(src).Kind().String()
}

// Set assigns value under key in d and returns d.
func Set(d map[string]interface{}, key string, value interface{}) map[string]interface{} {
	d[key] = value
	return d
}

// SplitList splits orig by sep into a []string (the indexable counterpart to
// the deprecated `split` map).
func SplitList(sep, orig string) []string {
	return strings.Split(orig, sep)
}

// ToString renders v as a string (strings pass through, fmt.Stringer values
// use String(), everything else is fmt.Sprint'ed).
func ToString(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// empty reports whether given holds the zero value for its type (the same
// semantics text/template's isTrue uses).
func empty(given interface{}) bool {
	g := reflect.ValueOf(given)
	if !g.IsValid() {
		return true
	}
	switch g.Kind() {
	default:
		return g.IsNil()
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return g.Len() == 0
	case reflect.Bool:
		return !g.Bool()
	case reflect.Complex64, reflect.Complex128:
		return g.Complex() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return g.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return g.Float() == 0
	case reflect.Struct:
		return false
	}
}

// strslice converts v into []string (used by Join).
func strslice(v interface{}) []string {
	switch v := v.(type) {
	case []string:
		return v
	case []interface{}:
		b := make([]string, 0, len(v))
		for _, s := range v {
			if s != nil {
				b = append(b, ToString(s))
			}
		}
		return b
	default:
		val := reflect.ValueOf(v)
		switch val.Kind() {
		case reflect.Array, reflect.Slice:
			l := val.Len()
			b := make([]string, 0, l)
			for i := range l {
				value := val.Index(i).Interface()
				if value != nil {
					b = append(b, ToString(value))
				}
			}
			return b
		default:
			if v == nil {
				return []string{}
			}
			return []string{ToString(v)}
		}
	}
}

// toInt64 coerces v to int64 for Add. It covers the numeric and string forms
// templates realistically pass; anything else yields 0.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	case float32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}
