package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yemaka/internal/config"
)

type Profile struct {
	Name                string
	Root                string
	Database            string
	Sessions            string
	Skills              string
	Extensions          string
	GeneratedExtensions string
	RAG                 string
	Logs                string
	Notifications       string
	Snapshots           string
	Evals               string
	Permissions         string
	ConfigPath          string
}

func Init(cfg *config.Config) (*Profile, error) {
	if cfg == nil {
		return nil, fmt.Errorf("profile init requires config")
	}

	supportDir, err := config.SupportDir()
	if err != nil {
		return nil, err
	}

	root := filepath.Join(supportDir, "profiles", cfg.App.Profile)
	generatedExtensions := config.ExpandPath(cfg.Extensions.GeneratedDir)
	if generatedExtensions == "" {
		generatedExtensions = filepath.Join(root, "extensions", "generated")
		cfg.Extensions.GeneratedDir = generatedExtensions
	}
	if !isChildPath(root, generatedExtensions) {
		generatedExtensions = filepath.Join(root, "extensions", "generated")
		cfg.Extensions.GeneratedDir = generatedExtensions
	}
	profile := &Profile{
		Name:                cfg.App.Profile,
		Root:                root,
		Database:            config.ExpandPath(cfg.Memory.Database),
		Sessions:            filepath.Join(root, "sessions"),
		Skills:              filepath.Join(root, "skills"),
		Extensions:          filepath.Join(root, "extensions"),
		GeneratedExtensions: generatedExtensions,
		RAG:                 filepath.Join(root, "rag"),
		Logs:                filepath.Join(root, "logs"),
		Notifications:       filepath.Join(root, "notifications"),
		Snapshots:           filepath.Join(root, "snapshots"),
		Evals:               filepath.Join(root, "evals"),
		Permissions:         filepath.Join(root, "permissions"),
		ConfigPath:          cfg.Path,
	}
	if profile.Database == "" {
		profile.Database = filepath.Join(root, "memory.sqlite")
		cfg.Memory.Database = profile.Database
	}

	for _, dir := range []string{
		profile.Root,
		filepath.Dir(profile.Database),
		profile.Sessions,
		profile.Skills,
		profile.Extensions,
		profile.GeneratedExtensions,
		profile.RAG,
		profile.Logs,
		profile.Notifications,
		profile.Snapshots,
		profile.Evals,
		profile.Permissions,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create profile directory %s: %w", dir, err)
		}
	}

	return profile, nil
}

func isChildPath(root string, child string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, childAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
