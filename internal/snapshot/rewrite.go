package snapshot

// Content-rewrite helpers used by the migration engine in --prune mode. When an
// entity, a field or the project (root namespace) is renamed, the developer-owned
// (overwrite: false) files that reference the old identifier are not regenerated.
// Rather than leaving a compile error, these functions rewrite the old identifier
// to the new one across the common casing forms (Pascal, camel, UPPER and, for
// entities, singular + plural). The logic is language-agnostic: it operates on
// plain identifier tokens, so a "ProductService" in C#, a "product_service" in
// Python or a bare `product` in TS are all rewritten by the same rules.

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/fsutil"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

// rewriteCasings replaces each old identifier with new inside content. Variants
// are applied longest-first (so a plural form like `Products` is rewritten as a
// whole before its singular head `Product` could consume it), then
// alphabetically, keeping output deterministic.
func rewriteCasings(content []byte, variants map[string]string) []byte {
	if len(variants) == 0 {
		return content
	}
	keys := make([]string, 0, len(variants))
	for k := range variants {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	out := content
	for _, old := range keys {
		new := variants[old]
		if old == "" || old == new {
			continue
		}
		out = replaceToken(out, []byte(old), []byte(new))
	}
	return out
}

// replaceToken replaces every structural occurrence of the exact identifier old
// with new. An occurrence begins at an identifier boundary (not preceded by an
// identifier character) and stops before any identifier character that is a
// lowercase ASCII letter. This lets `ProductService` become `ItemService` while
// both `myProduct` (inner) and `Productivity` (lowercase continuation) survive.
func replaceToken(content, old, new []byte) []byte {
	if len(old) == 0 {
		return content
	}
	out := make([]byte, 0, len(content))
	i := 0
	for i < len(content) {
		rel := bytes.Index(content[i:], old)
		if rel < 0 {
			out = append(out, content[i:]...)
			break
		}
		idx := i + rel
		out = append(out, content[i:idx]...)
		if tokenOK(content, idx, old) {
			out = append(out, new...)
		} else {
			out = append(out, old...)
		}
		i = idx + len(old)
	}
	return out
}

// tokenOK reports whether content[idx:idx+len(old)] is a standalone identifier
// occurrence. An occurrence must begin at a word boundary (possibly after a
// single uppercase marker/interface prefix such as `I` in `IProductService`, so
// that `myProduct` stays untouched) and its following rune, when it is an
// identifier character, must not be a lowercase ASCII letter (so `Productivity`
// survives).
func tokenOK(content []byte, idx int, old []byte) bool {
	if idx > 0 {
		prev := content[idx-1]
		// A lone uppercase prefix letter counts as a boundary (I, X, Z prefixes).
		prefixOK := prev >= 'A' && prev <= 'Z' && (idx < 2 || !isIdentChar(content[idx-2]))
		if isIdentChar(prev) && !prefixOK {
			return false
		}
	}
	j := idx + len(old)
	if j < len(content) && isIdentChar(content[j]) {
		if content[j] >= 'a' && content[j] <= 'z' {
			return false
		}
	}
	return true
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// entityCasings builds the casing-variant table for an entity rename.
func entityCasings(oldName, newName string) map[string]string {
	oldPascal := textutil.PascalCase(oldName)
	newPascal := textutil.PascalCase(newName)
	return map[string]string{
		oldPascal:                                       newPascal,                                       // Product -> Item
		textutil.CamelCase(oldName):                     textutil.CamelCase(newName),                     // product -> item
		strings.ToUpper(oldPascal):                      strings.ToUpper(newPascal),                      // PRODUCT -> ITEM
		textutil.Pluralize(oldPascal):                   textutil.Pluralize(newPascal),                   // Products -> Items
		textutil.CamelCase(textutil.Pluralize(oldName)): textutil.CamelCase(textutil.Pluralize(newName)), // products -> items
		strings.ToUpper(textutil.Pluralize(oldPascal)):  strings.ToUpper(textutil.Pluralize(newPascal)),  // PRODUCTS -> ITEMS
	}
}

// fieldCasings builds the casing-variant table for a field rename.
func fieldCasings(oldField, newField string) map[string]string {
	oldPascal := textutil.PascalCase(oldField)
	newPascal := textutil.PascalCase(newField)
	return map[string]string{
		oldPascal:                    newPascal,                    // Title -> Name
		textutil.CamelCase(oldField): textutil.CamelCase(newField), // title -> name
		strings.ToUpper(oldPascal):   strings.ToUpper(newPascal),   // TITLE -> NAME
	}
}

// namespaceCasings builds the casing-variant table for a root-namespace rename.
func namespaceCasings(oldNs, newNs string) map[string]string {
	return map[string]string{
		oldNs:                     newNs,
		textutil.CamelCase(oldNs): textutil.CamelCase(newNs),
		strings.ToUpper(oldNs):    strings.ToUpper(newNs),
	}
}

// RewriteEntityContent rewrites identifier references to an old entity name back
// to the new one (Pascal / camel / UPPER / plural forms).
func RewriteEntityContent(content []byte, oldName, newName string) []byte {
	return rewriteCasings(content, entityCasings(oldName, newName))
}

// RewriteFieldContent rewrites identifier references to an old field back to the
// new one (Pascal / camel / UPPER forms).
func RewriteFieldContent(content []byte, oldField, newField string) []byte {
	return rewriteCasings(content, fieldCasings(oldField, newField))
}

// RewriteNamespaceContent rewrites the old root-namespace identifier to the new one.
func RewriteNamespaceContent(content []byte, oldNs, newNs string) []byte {
	return rewriteCasings(content, namespaceCasings(oldNs, newNs))
}

// EntityTransform returns a transform rewriting an old entity identifier to its
// new name. It is the func([]byte) shape used to build compound transforms.
func EntityTransform(oldName, newName string) func([]byte) []byte {
	return func(b []byte) []byte { return RewriteEntityContent(b, oldName, newName) }
}

// NamespaceTransform returns a transform rewriting the old root namespace.
func NamespaceTransform(n *NamespaceMismatch) func([]byte) []byte {
	if n == nil {
		return func(b []byte) []byte { return b }
	}
	oldNs, newNs := n.OldNamespace, n.NewNamespace
	return func(b []byte) []byte { return RewriteNamespaceContent(b, oldNs, newNs) }
}

// FieldTransform returns a transform rewriting a renamed field's identifiers.
func FieldTransform(fr FieldRename) func([]byte) []byte {
	oldField, newField := fr.OldField, fr.NewField
	return func(b []byte) []byte { return RewriteFieldContent(b, oldField, newField) }
}

// RenameTransforms builds the content transforms to apply after an entity rename
// oldName -> newName: the entity identifiers themselves, the project namespace
// (when the project was renamed) and any field renames declared for that entity.
func RenameTransforms(oldName, newName string, d *Diff) []func([]byte) []byte {
	transforms := []func([]byte) []byte{EntityTransform(oldName, newName)}
	if d == nil {
		return transforms
	}
	if d.NamespaceRename != nil {
		transforms = append(transforms, NamespaceTransform(d.NamespaceRename))
	}
	for i := range d.FieldRenames {
		fr := d.FieldRenames[i]
		if fr.Entity != newName {
			continue
		}
		transforms = append(transforms, FieldTransform(fr))
	}
	return transforms
}

// RewriteFile applies content transforms to the file at relPath (relative to
// outputDir), writing it back atomically only when something changed. It returns
// whether a write occurred. A missing file is not an error.
func RewriteFile(outputDir, relPath string, transforms ...func([]byte) []byte) (bool, error) {
	if len(transforms) == 0 {
		return false, nil
	}
	abs := filepath.Join(outputDir, filepath.FromSlash(relPath))
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	out := data
	for _, t := range transforms {
		out = t(out)
	}
	if string(out) == string(data) {
		return false, nil
	}
	if err := fsutil.AtomicWrite(abs, out); err != nil {
		return false, err
	}
	return true, nil
}
