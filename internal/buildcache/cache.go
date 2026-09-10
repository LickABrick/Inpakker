package buildcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LickABrick/inpakker/internal/atomicfile"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"os"
	"path/filepath"
)

const (
	FileName      = "build-cache.json"
	currentSchema = 1
)

type Entry struct {
	Fingerprint string   `json:"fingerprint"`
	Artifacts   []string `json:"artifacts"`
}

type Cache struct {
	Schema int              `json:"schemaVersion"`
	Apps   map[string]Entry `json:"apps"`
}

// Get returns a copy of the cached entry for an application. It is intended
// for read-only status inspection; callers cannot mutate the cache through the
// returned value.
func (c *Cache) Get(key string) (Entry, bool) {
	entry, ok := c.Apps[key]
	entry.Artifacts = append([]string(nil), entry.Artifacts...)
	return entry, ok
}

func Load(workspaceRoot string) (*Cache, error) {
	path := filepath.Join(workspaceRoot, FileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read build cache: %w", err)
	}
	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("read build cache %q: %w (remove the file to rebuild it)", path, err)
	}
	if cache.Schema != currentSchema || cache.Apps == nil {
		return nil, fmt.Errorf("unsupported build cache schemaVersion %d", cache.Schema)
	}
	return &cache, nil
}

func (c *Cache) Current(key, fingerprint, appPath string) bool {
	entry, ok := c.Apps[key]
	if !ok || entry.Fingerprint != fingerprint || len(entry.Artifacts) == 0 {
		return false
	}
	for _, artifact := range entry.Artifacts {
		if !pathutil.ValidRelative(artifact) {
			return false
		}
		path := filepath.Join(appPath, filepath.FromSlash(artifact))
		if err := pathutil.Within(appPath, path); err != nil {
			return false
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func (c *Cache) Set(key, fingerprint, appPath string, artifacts []string) error {
	relative := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		path, err := filepath.Rel(appPath, artifact)
		if err != nil {
			return fmt.Errorf("record build artifact: %w", err)
		}
		relative = append(relative, filepath.ToSlash(path))
	}
	c.Apps[key] = Entry{Fingerprint: fingerprint, Artifacts: relative}
	return nil
}

func (c *Cache) Save(stateDirectory string) error {
	return atomicfile.JSON(filepath.Join(stateDirectory, FileName), c, 0600)
}

func empty() *Cache {
	return &Cache{Schema: currentSchema, Apps: make(map[string]Entry)}
}
