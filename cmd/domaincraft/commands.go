package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DomainCraft/DomainCraft/internal/bridge"
	"github.com/DomainCraft/DomainCraft/internal/interactive"
	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/renderer"
	"github.com/DomainCraft/DomainCraft/internal/snapshot"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/internal/validator"
	"github.com/DomainCraft/DomainCraft/pkg/logger"

	"github.com/spf13/cobra"
)

var (
	domainFile     string
	bridgePath     string
	outputDir      string
	nonInteractive bool
	adminBridge    string // --admin [bridge-id]; empty = not requested
	prune          bool   // --prune: apply migration cleanup without prompting
)

func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "domaincraft",
		Short: "DomainCraft CLI",
		Long:  "DomainCraft CLI — domain-driven code generator.\nParse domain.yaml, validate it, and generate production-ready code via pluggable bridges.",
	}

	rootCmd.PersistentFlags().StringVarP(&domainFile, "domain", "d", "domain.yaml", "path to domain.yaml")
	rootCmd.PersistentFlags().StringVarP(&bridgePath, "bridge", "b", "", "bridge ID, path, or owner/repo (interactive if omitted)")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "generated", "output directory")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "disable interactive prompts (requires all flags)")

	rootCmd.AddCommand(newNewCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newBridgesCmd())
	return rootCmd
}

// --- new / init ---

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "new",
		Aliases: []string{"init"},
		Short:   "Create a new project (interactive wizard)",
		Long:    "Create a new domain.yaml with an interactive wizard.\nIn non-interactive mode (--non-interactive), all options must be provided via flags.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if nonInteractive || !interactive.IsTerminal() {
				return runNonInteractiveNew(cmd)
			}
			return runInteractiveNew(cmd)
		},
	}

	cmd.Flags().String("name", "", "project name")
	cmd.Flags().String("version", "1.0.0", "project version")
	cmd.Flags().String("database", specmeta.Databases[0], "database type ("+strings.Join(specmeta.Databases, ", ")+")")
	cmd.Flags().String("auth", specmeta.AuthTypes[len(specmeta.AuthTypes)-1], "authentication type ("+strings.Join(specmeta.AuthTypes, ", ")+")")
	cmd.Flags().String("api-style", specmeta.APIStyles[0], "API style ("+strings.Join(specmeta.APIStyles, ", ")+")")

	return cmd
}

func runInteractiveNew(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	if name == "" {
		name, err = interactive.PromptProjectName()
		if err != nil {
			return err
		}
	}

	version, err := cmd.Flags().GetString("version")
	if err != nil {
		return err
	}

	database, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("database") {
		database, err = interactive.PromptDatabase()
		if err != nil {
			return err
		}
	}

	auth, err := cmd.Flags().GetString("auth")
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("auth") {
		auth, err = interactive.PromptAuth()
		if err != nil {
			return err
		}
	}

	apiStyle, err := cmd.Flags().GetString("api-style")
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("api-style") {
		apiStyle, err = interactive.PromptAPIStyle()
		if err != nil {
			return err
		}
	}

	resolved, bridgeName, err := resolveBridgeInteractive()
	if err != nil {
		return err
	}
	bridgePath = resolved
	if bridgeName != "" {
		fmt.Fprintf(out, "Bridge: %s (cached at %s)\n", bridgeName, resolved)
	}

	if err := scaffoldDomainYAML("domain.yaml", name, version, database, auth, apiStyle); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nCreated domain.yaml\n")
	fmt.Fprintf(out, "  Project:    %s\n", name)
	fmt.Fprintf(out, "  Database:   %s\n", database)
	fmt.Fprintf(out, "  Auth:       %s\n", auth)
	fmt.Fprintf(out, "  API style:  %s\n", apiStyle)
	fmt.Fprintf(out, "\nNext steps:\n")
	fmt.Fprintf(out, "  1. Edit domain.yaml to define your entities\n")
	fmt.Fprintf(out, "  2. Run 'domaincraft generate' to generate code\n")

	return nil
}

func runNonInteractiveNew(cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = "Sample App"
	}

	version, _ := cmd.Flags().GetString("version")
	database, _ := cmd.Flags().GetString("database")
	auth, _ := cmd.Flags().GetString("auth")
	apiStyle, _ := cmd.Flags().GetString("api-style")

	// Validate flag values against specmeta.
	if !specmeta.IsDatabase(database) {
		return fmt.Errorf("unknown database %q; allowed: %s", database, strings.Join(specmeta.Databases, ", "))
	}
	if !specmeta.IsAuthType(auth) {
		return fmt.Errorf("unknown auth type %q; allowed: %s", auth, strings.Join(specmeta.AuthTypes, ", "))
	}
	if !specmeta.IsAPIStyle(apiStyle) {
		return fmt.Errorf("unknown api style %q; allowed: %s", apiStyle, strings.Join(specmeta.APIStyles, ", "))
	}

	if err := scaffoldDomainYAML("domain.yaml", name, version, database, auth, apiStyle); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created domain.yaml (project: %s)\n", name)
	return nil
}

func scaffoldDomainYAML(path, name, version, database, auth, apiStyle string) error {
	var authBlock string
	if auth == "none" {
		authBlock = "auth:\n  type: none"
	} else {
		authBlock = fmt.Sprintf(`auth:
  type: %s
  roles: [Admin, User]
  endpoints:
    login: true
    register: true
    me: true`, auth)
	}

	content := fmt.Sprintf(`project:
  name: %s
  version: %s

database: %s
%s
api_style: %s

entities:
  User:
    features: [audit]
    fields:
      id: uuid [primary]
      email: string [required, unique, email]
      name: string [required]
      password: string [required, hidden]
`, name, version, database, authBlock, apiStyle)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists — remove it first or choose a different output directory", path)
		}
		return err
	}
	_, writeErr := f.Write([]byte(content))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// --- validate ---

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate domain.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New()
			log.SetWriter(cmd.OutOrStdout())
			log.Info("Validating %s", domainFile)
			schema, err := loadAndValidate(cmd.OutOrStdout())
			if err != nil {
				return err
			}
			log.Success("Schema valid (%d entities)", len(schema.Entities))
			return nil
		},
	}
}

// --- generate ---

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from domain.yaml",
		Long:  "Parse domain.yaml, build IR, and render code via the selected bridge.\nIf --bridge is omitted, an interactive selection menu is shown.\nUse --admin to also generate an admin panel (optionally specify a bridge ID).",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New()
			log.SetWriter(cmd.OutOrStdout())

			log.Info("Parsing %s", domainFile)
			schema, err := loadAndValidate(cmd.OutOrStdout())
			if err != nil {
				return err
			}
			log.Success("Schema valid (%d entities)", len(schema.Entities))

			resolvedPath, bridgeName, err := resolveBridgeInteractive()
			if err != nil {
				return err
			}
			bridgePath = resolvedPath

			log.Info("Building IR")
			irProject, err := ir.Build(schema)
			if err != nil {
				return err
			}

			// --- Schema snapshot / migration diff ---
			snapPath := snapshot.SnapshotPath(outputDir)
			oldSnapshot, err := snapshot.Load(snapPath)
			if err != nil {
				return fmt.Errorf("load schema snapshot: %w", err)
			}
			migrationDiff := snapshot.ComputeDiff(oldSnapshot, irProject, outputDir)

			log.Info("Rendering via %s", bridgePath)
			rendererInstance, err := renderer.New(bridgePath, log)
			if err != nil {
				return err
			}

			writtenFiles, manifest, err := rendererInstance.Render(irProject, outputDir)
			if err != nil {
				return err
			}

			log.Success("Generated %d file(s) into %s", len(writtenFiles), outputDir)

			// --- Admin panel generation ---
			if adminBridge == "" && !cmd.Flags().Changed("admin") && nonInteractive {
				// In non-interactive mode, skip admin prompt — admin not requested.
			} else if adminBridge == "" && !cmd.Flags().Changed("admin") && interactive.IsTerminal() {
				generate, _ := interactive.PromptGenerateAdmin()
				if generate {
					adminBridge = "admin-refine"
				}
			}
			if adminBridge != "" {
				adminManifest, err := generateAdminPanel(irProject, log)
				if err != nil {
					return err
				}
				manifest = append(manifest, adminManifest...)
			}

			// --- Migration actions: clean up orphaned / renamed files ---
			// Files written this run must never be touched by cleanup.
			fresh := make(map[string]bool)
			for _, f := range manifest {
				if f.Written {
					fresh[f.Path] = true
				}
			}
			applied, err := applyMigrationActions(migrationDiff, fresh, log, cmd)
			if err != nil {
				return err
			}

			// --- Smart warning for type changes (manual refactoring report) ---
			if report := migrationDiff.TypeChangeReport(); report != "" {
				log.Warn("ACTION REQUIRED — manual refactoring may be needed")
				fmt.Fprint(cmd.OutOrStdout(), report)
			}

			// --- Persist the new snapshot ---
			if !applied {
				// Cleanup was deferred (non-interactive without --prune). Keep the
				// previous snapshot so a later --prune run can still find the
				// orphaned/renamed files.
				log.Info("Snapshot kept — cleanup pending (re-run with --prune to finish)")
				return nil
			}
			bridgeID := bridgePath
			if bridgeName != "" {
				bridgeID = bridgeName
			}
			if err := snapshot.Save(snapPath, snapshot.New(bridgeID, irProject, manifest)); err != nil {
				return fmt.Errorf("save schema snapshot: %w", err)
			}

			return nil
		},
	}

	// --admin [bridge-id] — optional value, defaults to "admin-refine" when flag is present without value.
	cmd.Flags().StringVar(&adminBridge, "admin", "", "generate admin panel (optionally specify bridge ID, default: admin-refine)")
	// --prune — automatically delete/rename orphaned files without prompting (CI).
	cmd.Flags().BoolVar(&prune, "prune", false, "automatically remove/rename orphaned files detected by the migration engine (no prompts)")

	return cmd
}

// --- bridges ---

func newBridgesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bridges",
		Short: "List available bridges",
		Long:  "Show all known bridges with their cache status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := bridge.Default()
			out := cmd.OutOrStdout()

			entries := registry.All()
			if len(entries) == 0 {
				fmt.Fprintln(out, "No bridges registered.")
				return nil
			}

			fmt.Fprintf(out, "%-20s %-10s %-10s %s\n", "ID", "Language", "Status", "Description")
			fmt.Fprintf(out, "%-20s %-10s %-10s %s\n",
				strings.Repeat("-", 20),
				strings.Repeat("-", 10),
				strings.Repeat("-", 10),
				strings.Repeat("-", 40),
			)

			for _, e := range entries {
				status := "remote"
				if bridge.IsCached(e) {
					status = "cached"
				}
				fmt.Fprintf(out, "%-20s %-10s %-10s %s\n", e.ID, e.Language, status, e.Description)
			}

			return nil
		},
	}
}

// --- helpers ---

// resolveBridgeInteractive resolves the bridge from the --bridge flag, or
// prompts the user interactively. Returns (path, displayName, error).
func resolveBridgeInteractive() (string, string, error) {
	resolver := bridge.NewResolver(bridge.Default())

	if bridgePath != "" {
		resolved, err := resolver.Resolve(bridgePath)
		return resolved, "", err
	}

	if nonInteractive || !interactive.IsTerminal() {
		return "", "", fmt.Errorf("--bridge is required in non-interactive mode")
	}

	entry, err := interactive.SelectBridge(bridge.Default())
	if err != nil {
		return "", "", err
	}

	resolved, err := resolver.Resolve(entry.ID)
	if err != nil {
		return "", "", fmt.Errorf("resolve bridge %q: %w", entry.ID, err)
	}

	return resolved, entry.Name, nil
}

func generateAdminPanel(irProject *ir.IRProject, log *logger.Logger) ([]renderer.RenderedFile, error) {
	resolver := bridge.NewResolver(bridge.Default())

	adminID := adminBridge
	if adminID == "" {
		adminID = "admin-refine"
	}

	adminPath, err := resolver.Resolve(adminID)
	if err != nil {
		return nil, fmt.Errorf("resolve admin bridge %q: %w", adminID, err)
	}

	log.Info("Rendering admin panel via %s", adminID)
	adminRenderer, err := renderer.New(adminPath, log)
	if err != nil {
		return nil, err
	}

	adminFiles, adminManifest, err := adminRenderer.Render(irProject, outputDir)
	if err != nil {
		return nil, err
	}

	log.Success("Generated %d admin file(s)", len(adminFiles))
	return adminManifest, nil
}

// applyMigrationActions handles orphaned file cleanup for entities that were
// removed or renamed since the last generation. In interactive mode the user
// is prompted before touching custom (overwrite: false) files. With --prune
// everything is done automatically; otherwise (non-interactive) a warning is
// printed and nothing is modified. Files freshly written this run (fresh) are
// never touched.
//
// It returns whether all actions were applied. When cleanup was deferred
// (non-interactive without --prune) it returns false so the caller keeps the
// previous snapshot — otherwise orphaned files would be forgotten and a later
// --prune run could no longer find them.
func applyMigrationActions(diff *snapshot.Diff, fresh map[string]bool, log *logger.Logger, cmd *cobra.Command) (bool, error) {
	if diff == nil || diff.IsEmpty() {
		return true, nil
	}
	out := cmd.OutOrStdout()

	applied := true
	for _, del := range diff.Deleted {
		if len(del.Files) == 0 {
			continue
		}
		done, err := handleDeletedEntity(del, fresh, log, out)
		if err != nil {
			return false, err
		}
		applied = applied && done
	}

	for _, ren := range diff.Renamed {
		if len(ren.Files) == 0 {
			continue
		}
		done, err := handleRename(ren, fresh, log, out)
		if err != nil {
			return false, err
		}
		applied = applied && done
	}
	return applied, nil
}

// handleDeletedEntity deletes generated files automatically and asks (or warns)
// about custom files of a removed entity. Returns false when cleanup was
// deferred (non-interactive without --prune).
func handleDeletedEntity(del snapshot.DeletedEntity, fresh map[string]bool, log *logger.Logger, out io.Writer) (bool, error) {
	selected := make(map[string]bool)

	if prune {
		for _, f := range del.Files {
			selected[f.Path] = true
		}
	} else if interactive.IsTerminal() {
		choices := make([]interactive.FileChoice, len(del.Files))
		for i, f := range del.Files {
			choices[i] = interactive.FileChoice{Path: f.Path, Label: fileLabel(f)}
		}
		picked, err := interactive.PromptDeleteFiles(del.Name, choices)
		if err != nil {
			return false, err
		}
		for _, p := range picked {
			selected[p] = true
		}
	} else {
		log.Warn("Entity %q was removed from domain.yaml — orphaned files kept (re-run with --prune to delete):", del.Name)
		for _, f := range del.Files {
			fmt.Fprintf(out, "    - %s%s\n", f.Path, customMark(f))
		}
		return false, nil
	}

	for _, f := range del.Files {
		if !selected[f.Path] || fresh[f.Path] {
			continue
		}
		if err := snapshot.DeleteFile(outputDir, f.Path); err != nil {
			log.Warn("could not delete %s: %v", f.Path, err)
			continue
		}
		fmt.Fprintf(out, "  ▸ deleted %s\n", f.Path)
	}
	return true, nil
}

// handleRename renames custom files after a prompt and removes stale generated
// files (they have been regenerated under the new entity name). Returns false
// when cleanup was deferred (non-interactive without --prune).
func handleRename(ren snapshot.Rename, fresh map[string]bool, log *logger.Logger, out io.Writer) (bool, error) {
	selected := make(map[string]bool)

	if prune {
		for _, f := range ren.Files {
			selected[f.Path] = true
		}
	} else if interactive.IsTerminal() {
		choices := make([]interactive.FileChoice, len(ren.Files))
		for i, f := range ren.Files {
			choices[i] = interactive.FileChoice{Path: f.Path, Label: fileLabel(f)}
		}
		picked, err := interactive.PromptRenameFiles(ren.OldName, ren.NewName, choices)
		if err != nil {
			return false, err
		}
		for _, p := range picked {
			selected[p] = true
		}
	} else {
		log.Warn("Entity %q was renamed to %q — files kept (re-run with --prune to rename):", ren.OldName, ren.NewName)
		for _, f := range ren.Files {
			fmt.Fprintf(out, "    - %s -> %s\n", f.Path, snapshot.RenameRelPath(f.Path, ren.OldName, ren.NewName))
		}
		return false, nil
	}

	for _, f := range ren.Files {
		if !selected[f.Path] || fresh[f.Path] {
			continue
		}
		if f.Custom {
			newRel, renamed, err := snapshot.RenameEntityFile(outputDir, f.Path, ren.OldName, ren.NewName)
			if err != nil {
				log.Warn("could not rename %s: %v", f.Path, err)
				continue
			}
			if renamed {
				fmt.Fprintf(out, "  ▸ renamed %s -> %s\n", f.Path, newRel)
			}
			continue
		}
		// Generated files have been regenerated under the new name — remove the stale copy.
		if err := snapshot.DeleteFile(outputDir, f.Path); err != nil {
			log.Warn("could not delete stale generated file %s: %v", f.Path, err)
			continue
		}
		fmt.Fprintf(out, "  ▸ removed stale generated file %s\n", f.Path)
	}
	return true, nil
}

// fileLabel renders a file choice label, annotating developer-owned files.
func fileLabel(f snapshot.FileRef) string {
	return f.Path + customMark(f)
}

// customMark returns a marker suffix for custom (overwrite: false) files.
func customMark(f snapshot.FileRef) string {
	if f.Custom {
		return "  [custom]"
	}
	return ""
}

func loadAndValidate(out io.Writer) (*parser.ParsedSchema, error) {
	data, err := os.ReadFile(domainFile)
	if err != nil {
		return nil, fmt.Errorf("read domain file: %w", err)
	}
	schema, err := parser.ParseYAML(data)
	if err != nil {
		return nil, err
	}

	allErrors := validator.New(schema).Validate()
	var hardErrors []validator.ValidationError
	for _, e := range allErrors {
		if e.Warning {
			fmt.Fprintf(out, "⚠ %s\n", e.Error())
		} else {
			hardErrors = append(hardErrors, e)
			fmt.Fprintln(out, e.Error())
		}
	}

	if len(hardErrors) > 0 {
		return nil, fmt.Errorf("validation failed with %d error(s)", len(hardErrors))
	}

	return schema, nil
}
