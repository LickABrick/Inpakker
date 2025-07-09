package cacheutil

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const cacheFileName = ".inpakker-cache.json"

type Cache struct {
	Apps map[string]string `json:"apps"` // map[appPath]hash
	mu   sync.Mutex
}

func Load(workspaceRoot string) (*Cache, error) {
	path := filepath.Join(workspaceRoot, cacheFileName)

	c := &Cache{Apps: map[string]string{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil // return empty cache
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &c.Apps); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Cache) Save(workspaceRoot string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := filepath.Join(workspaceRoot, cacheFileName)
	data, err := json.MarshalIndent(c.Apps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Cache) Get(appPath string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash, ok := c.Apps[appPath]
	return hash, ok
}

func (c *Cache) Set(appPath, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Apps[appPath] = hash
}
