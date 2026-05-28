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
	cmd.Flags().String("database", "postgresql", "database type (postgresql, mysql, sqlite, mssql, mongodb)")
	cmd.Flags().String("auth", "none", "authentication type (jwt, none)")
	cmd.Flags().String("api-style", "rest", "API style (rest, graphql, grpc)")

	return cmd
}

func runInteractiveNew(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		var err error
		name, err = interactive.PromptProjectName()
		if err != nil {
			return err
		}
	}

	version, _ := cmd.Flags().GetString("version")

	database, _ := cmd.Flags().GetString("database")
	if !cmd.Flags().Changed("database") {
		var err error
		database, err = interactive.PromptDatabase()
		if err != nil {
			return err
		}
	}

	auth, _ := cmd.Flags().GetString("auth")
	if !cmd.Flags().Changed("auth") {
		var err error
		auth, err = interactive.PromptAuth()
		if err != nil {
			return err
		}
	}

	apiStyle, _ := cmd.Flags().GetString("api-style")
	if !cmd.Flags().Changed("api-style") {
		var err error
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

	if err := scaffoldDomainYAML("domain.yaml", name, version, database, auth, apiStyle); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created domain.yaml (project: %s)\n", name)
	return nil
}

func scaffoldDomainYAML(path, name, version, database, auth, apiStyle string) error {
	content := fmt.Sprintf(`project:
  name: %s
  version: %s

database: %s
auth: %s
api_style: %s

entities:
  User:
    features: [audit]
    fields:
      id: uuid [primary]
      email: string [required, unique, email]
      name: string [required]
`, name, version, database, auth, apiStyle)

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

			resolvedPath, _, err := resolveBridgeInteractive()
			if err != nil {
				return err
			}
			bridgePath = resolvedPath

			log.Info("Building IR")
			irProject, err := ir.NewBuilder().Build(schema)
			if err != nil {
				return err
			}

			log.Info("Rendering via %s", bridgePath)
			rendererInstance, err := renderer.New(bridgePath, log)
			if err != nil {
				return err
			}

			writtenFiles, err := rendererInstance.Render(irProject, outputDir)
			if err != nil {
				return err
			}

			log.Success("Generated %d file(s) into %s", len(writtenFiles), outputDir)

			// --- Admin panel generation ---
			if adminBridge == "" && !cmd.Flags().Changed("admin") && interactive.IsTerminal() {
				generate, _ := interactive.PromptGenerateAdmin()
				if generate {
					adminBridge = "admin-refine"
				}
			}
			if adminBridge != "" {
				if err := generateAdminPanel(irProject, log); err != nil {
					return err
				}
			}

			return nil
		},
	}

	// --admin [bridge-id] — optional value, defaults to "admin-refine" when flag is present without value.
	cmd.Flags().StringVar(&adminBridge, "admin", "", "generate admin panel (optionally specify bridge ID, default: admin-refine)")

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
	registry := bridge.Default()
	resolver := bridge.NewResolver(registry)

	if bridgePath != "" {
		resolved, err := resolver.Resolve(bridgePath)
		return resolved, "", err
	}

	if nonInteractive || !interactive.IsTerminal() {
		return "", "", fmt.Errorf("--bridge is required in non-interactive mode")
	}

	entry, err := interactive.SelectBridge(registry)
	if err != nil {
		return "", "", err
	}

	resolved, err := resolver.Resolve(entry.ID)
	if err != nil {
		return "", "", fmt.Errorf("resolve bridge %q: %w", entry.ID, err)
	}

	return resolved, entry.Name, nil
}

func generateAdminPanel(irProject *ir.IRProject, log *logger.Logger) error {
	registry := bridge.Default()
	resolver := bridge.NewResolver(registry)

	adminID := adminBridge
	if adminID == "" {
		adminID = "admin-refine"
	}

	adminPath, err := resolver.Resolve(adminID)
	if err != nil {
		return fmt.Errorf("resolve admin bridge %q: %w", adminID, err)
	}

	log.Info("Rendering admin panel via %s", adminID)
	adminRenderer, err := renderer.New(adminPath, log)
	if err != nil {
		return err
	}

	adminFiles, err := adminRenderer.Render(irProject, outputDir)
	if err != nil {
		return err
	}

	log.Success("Generated %d admin file(s)", len(adminFiles))
	return nil
}

func loadAndValidate(out io.Writer) (*parser.ParsedSchema, error) {
	schema, err := loadSchema(domainFile)
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

func loadSchema(path string) (*parser.ParsedSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read domain file: %w", err)
	}
	return parser.ParseYAML(data)
}
