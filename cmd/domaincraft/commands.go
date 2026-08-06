package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	addonsFlag     string // --addons "dapr,pulsar": comma-separated infrastructure accelerators
	migrateFlag    bool   // --migrate: run the bridge's database-migration commands after generation
	updateBridges  bool   // --update-bridges: download newer bridge versions before generating
	checkUpdates   bool   // bridges --check-updates: contact remotes and report outdated cached bridges

	// version is stamped at build time via -ldflags "-X main.version=vX.Y.Z".
	version = "dev"
)

func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "domaincraft",
		Short:   "DomainCraft CLI",
		Long:    "DomainCraft CLI — domain-driven code generator.\nParse domain.yaml, validate it, and generate production-ready code via pluggable bridges.",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Make the root runnable so Cobra exposes its --version flag, while
			// still showing the help text when invoked without a subcommand.
			return cmd.Help()
		},
	}

	rootCmd.PersistentFlags().StringVarP(&domainFile, "domain", "d", "domain.yaml", "path to domain.yaml")
	rootCmd.PersistentFlags().StringVarP(&bridgePath, "bridge", "b", "", "bridge ID, path, or owner/repo (interactive if omitted)")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "generated", "output directory")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "disable interactive prompts (requires all flags)")
	rootCmd.PersistentFlags().StringVar(&addonsFlag, "addons", "", "comma-separated infrastructure addons to enable (e.g. dapr)")
	rootCmd.PersistentFlags().BoolVar(&updateBridges, "update-bridges", false, "check cached bridges for newer versions and download them before generating (no prompts)")

	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newBridgesCmd())
	return rootCmd
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

			resolvedPath, bridgeName, err := resolveBridgeInteractive(log)
			if err != nil {
				return err
			}
			bridgePath = resolvedPath

			log.Info("Building IR")
			irProject, err := ir.Build(schema)
			if err != nil {
				return err
			}
			applyAddons(irProject)

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
					adminBridge = "admin-alpine"
				}
			}
			if adminBridge != "" {
				adminManifest, err := generateAdminPanel(irProject, log)
				if err != nil {
					return err
				}
				manifest = append(manifest, adminManifest...)
			}

			// --- Database migrations ---
			// Optionally run the bridge's declared migration commands so the
			// developer never has to think about SQL schemas. Gated on --migrate.
			if migrateFlag && rendererInstance.MigrationConfig() != nil && rendererInstance.MigrationConfig().Enabled {
				if err := runDatabaseMigrations(rendererInstance.MigrationConfig(), outputDir, cmd, log); err != nil {
					return err
				}
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

			// --- Smart warning for field renames (data-preserving column rename) ---
			if report := migrationDiff.FieldRenameReport(); report != "" {
				log.Warn("ACTION REQUIRED — fields were renamed; apply a safe column rename to avoid data loss")
				fmt.Fprint(cmd.OutOrStdout(), report)
			}

			// --- Project-rename warning: custom files with a stale root namespace ---
			if report := migrationDiff.NamespaceRenameReport(); report != "" {
				log.Warn("ACTION REQUIRED — project was renamed; custom files reference the old namespace")
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

	// --admin [bridge-id] — optional value, defaults to "admin-alpine" when flag is present without value.
	cmd.Flags().StringVar(&adminBridge, "admin", "", "generate admin panel (optionally specify bridge ID, default: admin-alpine)")
	// --prune — automatically delete/rename orphaned files without prompting (CI).
	cmd.Flags().BoolVar(&prune, "prune", false, "automatically remove/rename orphaned files detected by the migration engine (no prompts)")
	// --migrate — after generation, run the bridge's declared database-migration commands.
	cmd.Flags().BoolVar(&migrateFlag, "migrate", false, "run the bridge's database-migration commands after generation (e.g. dotnet ef database update)")

	return cmd
}

// --- bridges ---

func newBridgesCmd() *cobra.Command {
	cmd := &cobra.Command{
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

			fmt.Fprintf(out, "%-20s %-10s %-26s %s\n", "ID", "Language", "Status", "Description")
			fmt.Fprintf(out, "%-20s %-10s %-26s %s\n",
				strings.Repeat("-", 20),
				strings.Repeat("-", 10),
				strings.Repeat("-", 26),
				strings.Repeat("-", 40),
			)

			for _, e := range entries {
				status := "remote"
				if bridge.IsCached(e) {
					status = "cached"
					if checkUpdates {
						if u, err := bridge.CheckForUpdate(e, 0); err == nil && u != nil {
							status = "update available " + versionDelta(u)
						}
					}
				}
				fmt.Fprintf(out, "%-20s %-10s %-26s %s\n", e.ID, e.Language, status, e.Description)
			}

			if checkUpdates {
				fmt.Fprintln(out, "\nHint: run `domaincraft generate --update-bridges` to download available updates.")
			}
			return nil
		},
	}

	// --check-updates — contact each cached bridge's remote and report whether a
	// newer version is available (no modification, just status).
	cmd.Flags().BoolVar(&checkUpdates, "check-updates", false, "contact remotes and report whether cached bridges are outdated")

	return cmd
}

// --- helpers ---

// resolveBridgeInteractive resolves the bridge from the --bridge flag, or
// prompts the user interactively. Returns (path, displayName, error).
func resolveBridgeInteractive(log *logger.Logger) (string, string, error) {
	resolver := bridge.NewResolver(bridge.Default()).WithEnsureOptions(bridgeEnsureOptions(log))

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

// bridgeEnsureOptions builds the bridge caching/update policy from the CLI
// flags and terminal state:
//   - --update-bridges consents to downloading newer versions (CI-safe);
//   - otherwise, an interactive terminal is prompted when an update is found;
//   - in non-interactive mode the cached copy is kept and a warning is logged.
func bridgeEnsureOptions(log *logger.Logger) *bridge.EnsureOptions {
	opts := &bridge.EnsureOptions{
		Force: updateBridges,
		Warn:  log.Warn,
	}

	switch {
	case updateBridges:
		// The flag itself is the consent — update without prompting.
		opts.ConfirmUpdate = func(entry bridge.RegistryEntry, u *bridge.Update) (bool, error) {
			log.Info("Updating bridge %s %s", entry.ID, versionDelta(u))
			return true, nil
		}
	case interactive.IsTerminal():
		opts.ConfirmUpdate = func(entry bridge.RegistryEntry, u *bridge.Update) (bool, error) {
			return interactive.PromptBridgeUpdate(entry, u)
		}
	default:
		// Non-interactive without the flag: keep the cached copy; the bridge
		// package logs a warning pointing at --update-bridges.
		opts.ConfirmUpdate = nil
	}
	return opts
}

// versionDelta renders a "vX" hint for a detected update, or "" when the local
// version is unknown.
func versionDelta(u *bridge.Update) string {
	if u == nil || u.LocalVersion == "" {
		return ""
	}
	return fmt.Sprintf("(v%s available)", u.LocalVersion)
}

// generateAdminPanel renders the optional admin panel bridge into the output
// directory and returns the extra file manifest entries.
func generateAdminPanel(irProject *ir.IRProject, log *logger.Logger) ([]renderer.RenderedFile, error) {
	resolver := bridge.NewResolver(bridge.Default()).WithEnsureOptions(bridgeEnsureOptions(log))

	adminID := adminBridge
	if adminID == "" {
		adminID = "admin-alpine"
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

// runDatabaseMigrations executes the bridge's declared database-migration
// commands in order, from the generated output directory. Commands are shell
// lines invoked through the platform shell so bridges can rely on standard
// tooling (e.g. `dotnet ef ...`). A failing command aborts the run.
func runDatabaseMigrations(cfg *renderer.MigrationConfig, outDir string, cmd *cobra.Command, log *logger.Logger) error {
	if cfg == nil || !cfg.Enabled || len(cfg.Commands) == 0 {
		return nil
	}
	out := cmd.OutOrStdout()
	for i, command := range cfg.Commands {
		log.Info("Migration %d/%d: %s", i+1, len(cfg.Commands), command)
		var argv []string
		var shell string
		if runtime.GOOS == "windows" {
			shell = "cmd"
			argv = []string{"/C", command}
		} else {
			shell = "sh"
			argv = []string{"-c", command}
		}
		c := exec.Command(shell, argv...)
		c.Dir = outDir
		c.Stdout = out
		c.Stderr = out
		if err := c.Run(); err != nil {
			log.Error("Migration failed: %s", err)
			return fmt.Errorf("database migration %d failed: %w", i+1, err)
		}
	}
	log.Success("Database migrations applied")
	return nil
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
			newRel := snapshot.RenameRelPath(f.Path, ren.OldName, ren.NewName)
			if fresh[newRel] {
				// The destination was scaffolded this run by the custom
				// (overwrite: false) template. Replace it with the developer's
				// existing custom file so custom hooks survive the rename.
				if err := os.Remove(filepath.Join(outputDir, filepath.FromSlash(newRel))); err != nil {
					log.Warn("could not replace scaffolded %s: %v", newRel, err)
					continue
				}
			}
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

// applyAddons parses the --addons flag into the IR project's Addons set,
// validating each name against specmeta. Unknown addons are silently ignored so
// bridges that don't know a given addon simply render their non-addon templates.
func applyAddons(project *ir.IRProject) {
	if strings.TrimSpace(addonsFlag) == "" {
		return
	}
	if project.Addons == nil {
		project.Addons = make(map[string]bool)
	}
	for _, addon := range strings.Split(addonsFlag, ",") {
		addon = strings.TrimSpace(addon)
		if addon == "" {
			continue
		}
		if specmeta.IsAddon(addon) {
			project.Addons[addon] = true
		}
	}
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
