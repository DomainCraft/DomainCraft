package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/DomainCraft/DomainCraft/internal/ir"
	"github.com/DomainCraft/DomainCraft/pkg/logger"

	"gopkg.in/yaml.v3"
)

// sourceConfig is one bridge directory plus its parsed bridge.yaml. A composed
// renderer holds several of these, ordered base-first, with the adapter last.
type sourceConfig struct {
	dir    string
	config BridgeConfig
}

// Renderer reads bridge.yaml and renders files from IR.
//
// A renderer may be composed from a chain of bridges: the optional base bridges
// (declared via `extends:`) render first, then the adapter on top. Base
// templates, helper templates and type_mappings are all merged; the adapter's
// values win on conflicts.
type Renderer struct {
	sources []sourceConfig // ordered base-first; last entry is the adapter
	log     *logger.Logger
	// migration is the abstract schema-migration plan (set via SetMigration).
	migration *ir.MigrationPlan
	// seedData is the normalized explicit seed (set via SetSeedData).
	seedData *ir.SeedDataset
	// mockData is the deterministic mock seed (set via SetMockData).
	mockData *ir.SeedDataset
}

// config returns the topmost (adapter) bridge config, which owns the effective
// output_dir, name, delimiters and migration commands.
func (r *Renderer) config() BridgeConfig {
	if len(r.sources) == 0 {
		return BridgeConfig{}
	}
	return r.sources[len(r.sources)-1].config
}

// adapterDir returns the directory of the topmost (adapter) bridge.
func (r *Renderer) adapterDir() string {
	if len(r.sources) == 0 {
		return ""
	}
	return r.sources[len(r.sources)-1].dir
}

// loadSource loads a single bridge directory (its bridge.yaml).
func loadSource(bridgePath string) (sourceConfig, error) {
	bridgeDir, configPath := resolveBridgePath(bridgePath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return sourceConfig{}, fmt.Errorf("read bridge config: %w", err)
	}

	var config BridgeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return sourceConfig{}, fmt.Errorf("parse bridge config: %w", err)
	}
	if config.OutputDir == "" {
		config.OutputDir = "generated"
	}
	return sourceConfig{dir: bridgeDir, config: config}, nil
}

// New loads a single (leaf) bridge.
func New(bridgePath string, log *logger.Logger) (*Renderer, error) {
	src, err := loadSource(bridgePath)
	if err != nil {
		return nil, err
	}
	return &Renderer{sources: []sourceConfig{src}, log: log}, nil
}

// Extends reports the base bridge this bridge composes on top of (a path,
// registry ID or owner/repo shorthand), or "" when it is a leaf bridge.
func (r *Renderer) Extends() string {
	return r.config().Extends
}

// AttachBases prepends resolved base bridge directories (outermost base first)
// under this bridge. Bases render before the adapter, and the adapter's
// type_mappings/helpers override the bases' on conflicts.
func (r *Renderer) AttachBases(basePaths []string) error {
	bases := make([]sourceConfig, 0, len(basePaths)+len(r.sources))
	for _, p := range basePaths {
		src, err := loadSource(p)
		if err != nil {
			return err
		}
		bases = append(bases, src)
	}
	r.sources = append(bases, r.sources...)
	return nil
}

// MigrationConfig exposes the bridge's declared database-migration commands
// (from bridge.yaml manifest), used by `domaincraft generate --migrate`.
func (r *Renderer) MigrationConfig() *MigrationConfig {
	return r.config().Migrations
}

// SetMigration attaches the core-computed schema migration plan to this renderer.
func (r *Renderer) SetMigration(plan *ir.MigrationPlan) { r.migration = plan }

// SetSeedData attaches the core-normalized explicit seed to this renderer.
func (r *Renderer) SetSeedData(ds *ir.SeedDataset) { r.seedData = ds }

// SetMockData attaches the core-generated mock seed to this renderer.
func (r *Renderer) SetMockData(ds *ir.SeedDataset) { r.mockData = ds }

// delimiters returns the configured template delimiters, defaulting to ["{{", "}}"].
func (r *Renderer) delimiters() (string, string) {
	if len(r.config().Delimiters) >= 2 {
		return r.config().Delimiters[0], r.config().Delimiters[1]
	}
	return "{{", "}}"
}

// applyDelimiters sets custom delimiters on a template if configured.
func (r *Renderer) applyDelimiters(t *template.Template) *template.Template {
	left, right := r.delimiters()
	if left != "{{" || right != "}}" {
		return t.Delims(left, right)
	}
	return t
}

func resolveBridgePath(bridgePath string) (string, string) {
	info, err := os.Stat(bridgePath)
	if err != nil {
		// Path does not exist or is not accessible — treat as file path.
		return filepath.Dir(bridgePath), bridgePath
	}
	if info.IsDir() {
		return bridgePath, filepath.Join(bridgePath, "bridge.yaml")
	}
	return filepath.Dir(bridgePath), bridgePath
}