package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

func init() {
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

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate domain.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := loadSchema(domainFile)
			if err != nil {
				return err
			}

			validationErrors := validator.New(schema).Validate()
			if len(validationErrors) > 0 {
				for _, validationError := range validationErrors {
					fmt.Fprintln(cmd.OutOrStdout(), validationError.Error())
				}
				return fmt.Errorf("validation failed with %d error(s)", len(validationErrors))
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
			schema, err := loadSchema(domainFile)
			if err != nil {
				return err
			}

			validationErrors := validator.New(schema).Validate()
			if len(validationErrors) > 0 {
				for _, validationError := range validationErrors {
					fmt.Fprintln(cmd.OutOrStdout(), validationError.Error())
				}
				return fmt.Errorf("validation failed with %d error(s)", len(validationErrors))
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

func downloadAndExtractZip(zipURL, targetDir string) error {
	if zipURL == "" {
		return fmt.Errorf("no download url configured")
	}

	response, err := http.Get(zipURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("download bridge: unexpected status %s", response.Status)
	}

	tempFile, err := os.CreateTemp("", "domaincraft-bridge-*.zip")
	if err != nil {
		return err
	}
	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath)
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, response.Body); err != nil {
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	return extractZipArchive(tempFilePath, targetDir)
}

func extractZipArchive(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		relativePath := stripArchiveRoot(file.Name)
		if relativePath == "" {
			continue
		}

		outputPath := filepath.Join(targetDir, filepath.FromSlash(relativePath))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(outputPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}

		src, err := file.Open()
		if err != nil {
			return err
		}

		dst, err := os.Create(outputPath)
		if err != nil {
			src.Close()
			return err
		}

		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			return err
		}

		if err := dst.Close(); err != nil {
			src.Close()
			return err
		}
		if err := src.Close(); err != nil {
			return err
		}
	}

	return nil
}

func stripArchiveRoot(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	return filepath.ToSlash(filepath.Join(parts[1:]...))
}

func copyDir(sourceDir, targetDir string) error {
	if _, err := os.Stat(sourceDir); err != nil {
		return err
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}

		outputPath := filepath.Join(targetDir, relativePath)
		if info.IsDir() {
			return os.MkdirAll(outputPath, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}

		return os.WriteFile(outputPath, data, info.Mode())
	})
}
