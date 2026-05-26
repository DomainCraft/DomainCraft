package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommandCreatesDomainYAML(t *testing.T) {
	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	cmd := newRootCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v\noutput: %s", err, output.String())
	}

	mustExist(t, filepath.Join(workDir, "domain.yaml"))

	// Check that domain.yaml is a valid YAML file with basic structure
	content, err := os.ReadFile(filepath.Join(workDir, "domain.yaml"))
	if err != nil {
		t.Fatalf("failed to read domain.yaml: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "project:") {
		t.Fatalf("domain.yaml should contain 'project:' section")
	}
	if !strings.Contains(contentStr, "database:") {
		t.Fatalf("domain.yaml should contain 'database:' section")
	}
	if !strings.Contains(contentStr, "entities:") {
		t.Fatalf("domain.yaml should contain 'entities:' section")
	}
}

func TestValidateCommandSucceeds(t *testing.T) {
	workDir := t.TempDir()
	domainPath := filepath.Join(workDir, "domain.yaml")
	if err := os.WriteFile(domainPath, []byte(`project:
  name: Validate App

database: postgresql
entities:
  User:
    fields:
      id: uuid [primary]
      email: string [required]
`), 0o644); err != nil {
		t.Fatalf("write domain: %v", err)
	}

	cmd := newRootCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"validate", "--domain", domainPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate command failed: %v\noutput: %s", err, output.String())
	}
	if !strings.Contains(output.String(), "Schema valid") {
		t.Fatalf("expected 'Schema valid' in output, got: %s", output.String())
	}
}

func TestGenerateCommandWithoutBridgeShowsError(t *testing.T) {
	workDir := t.TempDir()
	domainPath := filepath.Join(workDir, "domain.yaml")
	bridgeDir := filepath.Join(workDir, "bridges", "nonexistent")
	outputDirPath := filepath.Join(workDir, "generated")

	if err := os.WriteFile(domainPath, []byte(`project:
  name: Generate App
  version: 1.0.0

database: postgresql
entities:
  User:
    fields:
      id: uuid [primary]
      email: string [required]
`), 0o644); err != nil {
		t.Fatalf("write domain: %v", err)
	}

	cmd := newRootCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"generate", "--domain", domainPath, "--bridge", bridgeDir, "--output", outputDirPath})

	// Should fail because bridge doesn't exist
	if err := cmd.Execute(); err == nil {
		t.Fatalf("generate command should fail with nonexistent bridge")
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
