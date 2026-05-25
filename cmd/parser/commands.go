package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"domaincraft/internal/ir"
	"domaincraft/internal/parser"
	"domaincraft/internal/renderer"
	"domaincraft/internal/validator"

	"github.com/spf13/cobra"
)

var (
	domainFile string
	bridgePath string
	outputDir  string
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
		Long:  "DomainCraft CLI parses domain.yaml, validates it, builds IR, and renders templates.",
	}

	rootCmd.PersistentFlags().StringVarP(&domainFile, "domain", "d", "domain.yaml", "path to domain.yaml")
	rootCmd.PersistentFlags().StringVarP(&bridgePath, "bridge", "b", "bridges/csharp", "path to bridge directory or bridge.yaml")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "generated", "output directory")

	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newInitCmd())
	return rootCmd
}

func loadAndValidate(out io.Writer) (*parser.ParsedSchema, error) {
	schema, err := loadSchema(domainFile)
	if err != nil {
		return nil, err
	}

	validationErrors := validator.New(schema).Validate()
	if len(validationErrors) > 0 {
		for _, validationError := range validationErrors {
			fmt.Fprintln(out, validationError.Error())
		}
		return nil, fmt.Errorf("validation failed with %d error(s)", len(validationErrors))
	}

	return schema, nil
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate domain.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadAndValidate(cmd.OutOrStdout()); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Validation successful")
			return nil
		},
	}
}

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate code from domain.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := loadAndValidate(cmd.OutOrStdout())
			if err != nil {
				return err
			}

			irProject, err := ir.NewBuilder().Build(schema)
			if err != nil {
				return err
			}

			rendererInstance, err := renderer.New(bridgePath)
			if err != nil {
				return err
			}

			writtenFiles, err := rendererInstance.Render(irProject, outputDir)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Generated %d file(s) into %s\n", len(writtenFiles), outputDir)
			for _, filePath := range writtenFiles {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", filePath)
			}
			return nil
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a starter domain.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := scaffoldDomainFile("domain.yaml"); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created domain.yaml. Use --bridge flag to specify your bridge repository URL or local path.\n")
			return nil
		},
	}
}

func loadSchema(path string) (*parser.ParsedSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read domain file: %w", err)
	}
	return parser.ParseYAML(data)
}

func scaffoldDomainFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	content := `project:
  name: Sample App
  version: 1.0.0

database: postgresql
auth: none
api_style: rest

entities:
  User:
    features: [audit]
    fields:
      id: uuid [primary]
      email: string [required, unique, email]
      name: string [required]
`

	return os.WriteFile(path, []byte(content), 0o644)
}
