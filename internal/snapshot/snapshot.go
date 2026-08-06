// Package snapshot implements the schema snapshot / migration engine.
//
// After a successful `domaincraft generate` the current state of the domain
// (the IR) is persisted to `<output>/.domaincraft/snapshot.json`. On the next
// run the new domain.yaml is compared against that snapshot and a Diff is
// computed: deleted entities, renamed entities (via the `old_name` hint), and
// field type changes. The CLI uses the diff to offer cleanup of orphaned files
// (delete/rename) and to warn about custom files (overwrite: false) that may
// have been broken by a type change.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/renderer"
	"github.com/DomainCraft/DomainCraft/pkg/textutil"
)

const (
	// FormatVersion is bumped when the on-disk snapshot shape changes.
	FormatVersion = 1

	// DirName is the hidden directory storing DomainCraft state inside the output dir.
	DirName = ".domaincraft"

	// FileName is the snapshot file name inside DirName.
	FileName = "snapshot.json"
)

// EntityState records the shape of one entity at snapshot time so that later
// runs can diff it against the current IR.
type EntityState struct {
	OldName string            `json:"old_name,omitempty"`
	Fields  map[string]string `json:"fields"` // field name -> IR database type
	// FieldOldNames records the field-level `old_name` rename hints (previous
	// field name) declared in the CURRENT model, so the next diff can recognize
	// a safe column rename instead of a destructive drop + add.
	FieldOldNames map[string]string `json:"field_old_names,omitempty"` // field name -> previous field name
}

// Snapshot is the persisted history of the domain model for one output dir.
type Snapshot struct {
	FormatVersion int                     `json:"format_version"`
	Bridge        string                  `json:"bridge"`
	CreatedAt     string                  `json:"created_at"`
	ProjectNamespace string               `json:"project_namespace,omitempty"` // pascal-cased project name used as the C#/other root namespace
	Files         []renderer.RenderedFile `json:"files"`
	Entities      map[string]EntityState  `json:"entities"`
}

// SnapshotPath returns the absolute path of the snapshot for an output dir.
func SnapshotPath(outputDir string) string {
	return filepath.Join(outputDir, DirName, FileName)
}

// Load reads a snapshot from path. It returns (nil, nil) when the snapshot
// does not exist yet (first generation run).
func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read snapshot %s: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse snapshot %s: %w", path, err)
	}
	return &snap, nil
}

// Save persists the snapshot to path, creating the parent directory.
func Save(path string, snap *Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// New builds a fresh snapshot from the current IR and the renderer file manifest.
func New(bridge string, project *ir.IRProject, files []renderer.RenderedFile) *Snapshot {
	entities := make(map[string]EntityState, len(project.Entities))
	for _, e := range project.Entities {
		fields := make(map[string]string, len(e.Fields))
		fieldOldNames := make(map[string]string)
		for _, f := range e.Fields {
			fields[f.Name] = f.DatabaseType
			if f.OldName != "" {
				fieldOldNames[f.Name] = f.OldName
			}
		}
		entities[e.Name] = EntityState{OldName: e.OldName, Fields: fields, FieldOldNames: fieldOldNames}
	}
	return &Snapshot{
		FormatVersion:    FormatVersion,
		Bridge:           bridge,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ProjectNamespace: textutil.PascalCase(project.Name),
		Files:            files,
		Entities:         entities,
	}
}

// FileRef identifies an orphaned file relative to the output dir.
type FileRef struct {
	Path   string // relative path (forward slashes)
	Custom bool   // true when the file was generated with overwrite: false (developer-owned)
}

// DeletedEntity reports an entity that existed in the previous snapshot but is
// gone from the current domain model.
type DeletedEntity struct {
	Name  string
	Files []FileRef // entity-scoped files (relative to output dir) that still exist on disk
}

// Rename reports an entity renamed via the `old_name` hint.
type Rename struct {
	OldName string
	NewName string
	Files   []FileRef // entity-scoped files (relative to output dir) generated under the old name
}

// TypeChange reports a field whose database type changed between runs.
type TypeChange struct {
	Entity      string // current entity name
	Field       string
	OldType     string
	NewType     string
	CustomFiles []string // custom (overwrite: false) file paths for the entity that still exist
}

// FieldRename reports a field renamed via the field-level `old_name` modifier.
// A safe column rename (RenameColumn) preserves the data on a real database,
// whereas a DropColumn + AddColumn would destroy it.
type FieldRename struct {
	Entity   string // current entity name
	OldField string // previous field name (the old_name hint)
	NewField string // current field name
	OldColumn string // previous snake_case DB column
	NewColumn string // current snake_case DB column
	OldType  string
	NewType  string
	CustomFiles []string // custom (overwrite: false) file paths for the entity that still exist
}

// NamespaceMismatch reports custom (overwrite: false) files that still reference
// the previous project's root namespace after the project was renamed. The bridge
// regenerates only the generated (overwrite: true) files with the new namespace;
// developer-owned files keep their old `namespace OldProject.*` and would no longer
// compile. The files listed still exist on disk and contain the old namespace token.
type NamespaceMismatch struct {
	OldNamespace string   // previous pascal-cased project name
	NewNamespace string   // current pascal-cased project name
	Files        []string // relative paths of custom files still containing the old namespace
}

// Diff is the computed difference between the old snapshot and the current IR.
type Diff struct {
	Deleted       []DeletedEntity
	Renamed       []Rename
	TypeChanges   []TypeChange
	FieldRenames  []FieldRename
	NamespaceRename *NamespaceMismatch
}

// IsEmpty reports whether the diff contains nothing actionable.
func (d *Diff) IsEmpty() bool {
	return len(d.Deleted) == 0 && len(d.Renamed) == 0 && len(d.TypeChanges) == 0 && len(d.FieldRenames) == 0 && d.NamespaceRename == nil
}

// ComputeDiff compares an old snapshot against the current IR project.
// File lists are filtered to those that still exist on disk under outputDir.
func ComputeDiff(old *Snapshot, project *ir.IRProject, outputDir string) *Diff {
	diff := &Diff{}
	if old == nil || project == nil {
		return diff
	}

	newEntities := make(map[string]ir.IREntity, len(project.Entities))
	renamedByOld := make(map[string]string) // old name -> new name
	for _, e := range project.Entities {
		newEntities[e.Name] = e
		if e.OldName != "" {
			renamedByOld[e.OldName] = e.Name
		}
	}

	// Group old entity-scoped files by entity name.
	filesByEntity := make(map[string][]renderer.RenderedFile)
	for _, f := range old.Files {
		if f.Entity != "" {
			filesByEntity[f.Entity] = append(filesByEntity[f.Entity], f)
		}
	}

	// Renamed entities (declared via old_name).
	for oldName, newName := range renamedByOld {
		if _, ok := old.Entities[oldName]; !ok {
			continue
		}
		files := existingFileRefs(outputDir, filesByEntity[oldName])
		diff.Renamed = append(diff.Renamed, Rename{OldName: oldName, NewName: newName, Files: files})
	}
	sort.Slice(diff.Renamed, func(i, j int) bool { return diff.Renamed[i].OldName < diff.Renamed[j].OldName })

	// Deleted entities (in the snapshot, not in the current model, not renamed).
	for oldName := range old.Entities {
		if _, ok := newEntities[oldName]; ok {
			continue
		}
		if _, renamed := renamedByOld[oldName]; renamed {
			continue
		}
		files := existingFileRefs(outputDir, filesByEntity[oldName])
		diff.Deleted = append(diff.Deleted, DeletedEntity{Name: oldName, Files: files})
	}
	sort.Slice(diff.Deleted, func(i, j int) bool { return diff.Deleted[i].Name < diff.Deleted[j].Name })

	// Field type changes for entities that still exist (possibly under a new name).
	for oldName, oldState := range old.Entities {
		newName := oldName
		if mapped, ok := renamedByOld[oldName]; ok {
			newName = mapped
		}
		newEntity, ok := newEntities[newName]
		if !ok {
			continue
		}
		newFields := make(map[string]string, len(newEntity.Fields))
		for _, f := range newEntity.Fields {
			newFields[f.Name] = f.DatabaseType
		}
		customFiles := existingFilePaths(outputDir, customEntityFiles(old.Files, oldName))
		for fieldName, oldType := range oldState.Fields {
			newType, ok := newFields[fieldName]
			if !ok || oldType == newType {
				continue
			}
			diff.TypeChanges = append(diff.TypeChanges, TypeChange{
				Entity:      newName,
				Field:       fieldName,
				OldType:     oldType,
				NewType:     newType,
				CustomFiles: customFiles,
			})
		}
	}
	sort.Slice(diff.TypeChanges, func(i, j int) bool {
		if diff.TypeChanges[i].Entity != diff.TypeChanges[j].Entity {
			return diff.TypeChanges[i].Entity < diff.TypeChanges[j].Entity
		}
		return diff.TypeChanges[i].Field < diff.TypeChanges[j].Field
	})

	// Field renames (via the field-level `old_name` modifier). Detected when the
	// current field declares a previous name that exists in the old snapshot and
	// the current field name did not exist there (a true rename, not a shadow).
	// A safe RenameColumn preserves the data — a Drop+Add would destroy it.
	for oldName, oldState := range old.Entities {
		newName := oldName
		if mapped, ok := renamedByOld[oldName]; ok {
			newName = mapped
		}
		newEntity, ok := newEntities[newName]
		if !ok {
			continue
		}
		customFiles := existingFilePaths(outputDir, customEntityFiles(old.Files, oldName))
		for _, f := range newEntity.Fields {
			if f.OldName == "" {
				continue
			}
			oldFieldType, wasPresent := oldState.Fields[f.OldName]
			if !wasPresent {
				continue
			}
			// Ambiguous: the new field name already existed in the previous model,
			// so this is not a clean rename (two different columns would be involved).
			if _, already := oldState.Fields[f.Name]; already {
				continue
			}
			diff.FieldRenames = append(diff.FieldRenames, FieldRename{
				Entity:      newName,
				OldField:    f.OldName,
				NewField:    f.Name,
				OldColumn:   f.OldDatabaseColumnName,
				NewColumn:   f.DatabaseColumnName,
				OldType:     oldFieldType,
				NewType:     f.DatabaseType,
				CustomFiles: customFiles,
			})
		}
	}
	sort.Slice(diff.FieldRenames, func(i, j int) bool {
		if diff.FieldRenames[i].Entity != diff.FieldRenames[j].Entity {
			return diff.FieldRenames[i].Entity < diff.FieldRenames[j].Entity
		}
		return diff.FieldRenames[i].OldField < diff.FieldRenames[j].OldField
	})

	// Project rename: custom (overwrite: false) files carry the old root namespace
	// because they are scaffolded once. Detect them by scanning their content so the
	// developer gets an actionable list instead of a compile error later.
	currentNamespace := textutil.PascalCase(project.Name)
	if old.ProjectNamespace != "" && old.ProjectNamespace != currentNamespace {
		diff.NamespaceRename = detectNamespaceMismatch(outputDir, old, old.ProjectNamespace, currentNamespace)
	}

	return diff
}

// detectNamespaceMismatch finds custom files from the old snapshot (still present
// on disk) whose content references the old root namespace.
func detectNamespaceMismatch(outputDir string, old *Snapshot, oldNamespace, newNamespace string) *NamespaceMismatch {
	var affected []string
	for _, f := range old.Files {
		if !f.Custom {
			continue
		}
		abs := filepath.Join(outputDir, filepath.FromSlash(f.Path))
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		if referencesNamespace(string(data), oldNamespace) {
			affected = append(affected, f.Path)
		}
	}
	if len(affected) == 0 {
		return nil
	}
	sort.Strings(affected)
	return &NamespaceMismatch{OldNamespace: oldNamespace, NewNamespace: newNamespace, Files: affected}
}

// referencesNamespace reports whether s declares the given root namespace, e.g.
// `namespace EscrowPay...` or `namespace EscrowPay;`.
func referencesNamespace(s, namespace string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "namespace ") {
			continue
		}
		rest := strings.TrimPrefix(line, "namespace ")
		rest = strings.TrimSuffix(rest, ";")
		rest = strings.TrimSpace(rest)
		if rest == namespace || strings.HasPrefix(rest, namespace+".") {
			return true
		}
	}
	return false
}

// NamespaceRenameReport returns a human-readable report for the project-namespace
// mismatch in the diff, or an empty string when there is none.
func (d *Diff) NamespaceRenameReport() string {
	n := d.NamespaceRename
	if n == nil || len(n.Files) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You renamed the project from %q to %q. Developer-owned custom files\n", n.OldNamespace, n.NewNamespace)
	b.WriteString("(overwrite: false) still declare the old root namespace and will no longer compile:\n\n")
	for i, f := range n.Files {
		fmt.Fprintf(&b, "%d. %s\n", i+1, f)
	}
	b.WriteString("\nReplace `namespace "+n.OldNamespace+".` with `namespace "+n.NewNamespace+".` in each file ")
	b.WriteString("(or run a find-and-replace across your project).\n")
	return b.String()
}

// customEntityFiles returns the relative paths of custom (overwrite: false)
// files that were generated for the given entity in the old snapshot.
func customEntityFiles(files []renderer.RenderedFile, entityName string) []string {
	var paths []string
	for _, f := range files {
		if f.Entity == entityName && f.Custom {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// existingFileRefs filters old entity-scoped files to those that still exist
// on disk, preserving whether each is a custom (overwrite: false) file.
func existingFileRefs(outputDir string, files []renderer.RenderedFile) []FileRef {
	var refs []FileRef
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(f.Path))); err != nil {
			continue
		}
		refs = append(refs, FileRef{Path: f.Path, Custom: f.Custom})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Path < refs[j].Path })
	return refs
}

// existingFilePaths filters relative paths to those that still exist on disk.
func existingFilePaths(outputDir string, relPaths []string) []string {
	var result []string
	for _, rel := range relPaths {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); err == nil {
			result = append(result, rel)
		}
	}
	sort.Strings(result)
	return result
}

// RenameRelPath maps an entity-scoped file path to its path after the entity
// rename oldName -> newName. Both the plural and the singular forms of the old
// name are replaced (plural first, since it is more specific).
func RenameRelPath(relPath, oldName, newName string) string {
	out := strings.ReplaceAll(relPath, textutil.Pluralize(oldName), textutil.Pluralize(newName))
	out = strings.ReplaceAll(out, oldName, newName)
	return out
}

// DeleteFile removes an entity-scoped file (path relative to outputDir).
func DeleteFile(outputDir, relPath string) error {
	abs := filepath.Join(outputDir, filepath.FromSlash(relPath))
	if err := os.Remove(abs); err != nil {
		return fmt.Errorf("remove %s: %w", relPath, err)
	}
	return nil
}

// RenameEntityFile renames an entity-scoped file after the entity rename
// oldName -> newName. It returns the new relative path and whether the file was
// actually renamed. When the destination already exists (e.g. it was generated
// this run or created manually) the file is left untouched.
func RenameEntityFile(outputDir, relPath, oldName, newName string) (newRel string, renamed bool, err error) {
	newRel = RenameRelPath(relPath, oldName, newName)
	if newRel == relPath {
		return newRel, false, nil
	}
	abs := filepath.Join(outputDir, filepath.FromSlash(relPath))
	newAbs := filepath.Join(outputDir, filepath.FromSlash(newRel))
	if _, err := os.Stat(newAbs); err == nil {
		return newRel, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil {
		return newRel, false, fmt.Errorf("create parent dir for %s: %w", newRel, err)
	}
	if err := os.Rename(abs, newAbs); err != nil {
		return newRel, false, fmt.Errorf("rename %s to %s: %w", relPath, newRel, err)
	}
	return newRel, true, nil
}

// FieldRenameReport returns a human-readable report for the field renames in
// the diff, or an empty string when there are none. It tells the developer that
// a safe column rename is required — a DropColumn + AddColumn would destroy the
// existing data. Column names are bridge-agnostic snake_case; each bridge turns
// a field's `old_name` into its own rename statement (see the C# bridge, which
// generates migrationBuilder.RenameColumn guidance).
func (d *Diff) FieldRenameReport() string {
	if len(d.FieldRenames) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("You renamed fields in domain.yaml. Keep the existing data by renaming the DB\n")
	b.WriteString("column instead of dropping it and adding a new one (that would erase the data):\n\n")
	for i, fr := range d.FieldRenames {
		fmt.Fprintf(&b, "%d. Entity: %s — field %s renamed to %s\n", i+1, fr.Entity, fr.OldField, fr.NewField)
		fmt.Fprintf(&b, "   Column:   %s -> %s\n", fr.OldColumn, fr.NewColumn)
		if fr.OldType != fr.NewType {
			fmt.Fprintf(&b, "   Types:    %s -> %s (was a rename AND a type change)\n", fr.OldType, fr.NewType)
		}
		if len(fr.CustomFiles) > 0 {
			b.WriteString("   Files that may reference the old name:\n")
			for _, f := range fr.CustomFiles {
				fmt.Fprintf(&b, "   - %s\n", f)
			}
		}
	}
	b.WriteString("\nThe generated bridge code emits the exact rename statement for each renamed column;\n")
	b.WriteString("apply it as a single migration before regenerating schema-dependent data.\n")
	return b.String()
}

// TypeChangeReport returns a human-readable refactoring report for the type
// changes in the diff.
func (d *Diff) TypeChangeReport() string {
	if len(d.TypeChanges) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("You changed data types in domain.yaml. Custom files (overwrite: false)\n")
	b.WriteString("that reference these fields may have broken and need manual refactoring:\n\n")
	for i, tc := range d.TypeChanges {
		fmt.Fprintf(&b, "%d. Entity: %s (Field: %s)\n", i+1, tc.Entity, tc.Field)
		fmt.Fprintf(&b, "   Change: %s -> %s\n", tc.OldType, tc.NewType)
		if len(tc.CustomFiles) > 0 {
			b.WriteString("   Files to check:\n")
			for _, f := range tc.CustomFiles {
				fmt.Fprintf(&b, "   - %s\n", f)
			}
		} else {
			b.WriteString("   (no custom files recorded — verify generated code in your IDE)\n")
		}
	}
	b.WriteString("\nTip: open the project in your IDE and check for compilation errors.\n")
	return b.String()
}
