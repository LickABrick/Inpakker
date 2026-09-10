package buildcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	FileName      = ".inpakker-cache.json"
	currentSchema = 1
)

type Entry struct {
	Fingerprint string   `json:"fingerprint"`
	Artifacts   []string `json:"artifacts"`
}

type Cache struct {
	Schema int              `json:"schema"`
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
		return empty(), nil
	}
	return &cache, nil
}

func (c *Cache) Current(key, fingerprint, appPath string) bool {
	entry, ok := c.Apps[key]
	if !ok || entry.Fingerprint != fingerprint || len(entry.Artifacts) == 0 {
		return false
	}
	for _, artifact := range entry.Artifacts {
		info, err := os.Stat(filepath.Join(appPath, filepath.FromSlash(artifact)))
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

func (c *Cache) Save(workspaceRoot string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode build cache: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(workspaceRoot, FileName)
	temporary, err := os.CreateTemp(workspaceRoot, ".inpakker-cache-*")
	if err != nil {
		return fmt.Errorf("create temporary build cache: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o644); err == nil {
		_, err = temporary.Write(data)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write build cache: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace build cache: %w", err)
	}
	return nil
}

func empty() *Cache {
	return &Cache{Schema: currentSchema, Apps: make(map[string]Entry)}
}
