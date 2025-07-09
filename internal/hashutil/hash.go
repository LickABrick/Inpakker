package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/LickABrick/inpakker/types"
)

type CacheEntry struct {
	Hash string `json:"hash"`
}

type Cache struct {
	Apps map[string]CacheEntry `json:"apps"`
}

const CacheFile = ".inpakker-cache.json"

// ComputeAppHash returns a SHA-256 hash of relevant files including filenames.
func ComputeAppHash(appPath string, cfg *types.AppConfig) (string, error) {
	h := sha256.New()

	// Include app.config.json in hash
	if err := hashFileWithName(h, filepath.Join(appPath, "app.config.json"), "app.config.json"); err != nil {
		return "", fmt.Errorf("hashing app config: %w", err)
	}

	// Include setup file
	setupPath := filepath.Join(appPath, cfg.Source, cfg.SetupFile)
	setupRel, _ := filepath.Rel(appPath, setupPath)
	if err := hashFileWithName(h, setupPath, setupRel); err != nil {
		return "", fmt.Errorf("hashing setup file: %w", err)
	}

	// Walk source dir
	sourcePath := filepath.Join(appPath, cfg.Source)
	var files []string
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking source: %w", err)
	}

	sort.Strings(files)

	for _, file := range files {
		rel, _ := filepath.Rel(appPath, file)
		if err := hashFileWithName(h, file, rel); err != nil {
			return "", fmt.Errorf("hashing file %s: %w", file, err)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFileWithName(w io.Writer, path, relPath string) error {
	_, _ = w.Write([]byte(relPath)) // include file name
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}

func LoadCache(cachePath string) (Cache, error) {
	var cache Cache
	cache.Apps = make(map[string]CacheEntry)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil // empty cache
		}
		return cache, err
	}
	err = json.Unmarshal(data, &cache)
	return cache, err
}

func SaveCache(cachePath string, cache Cache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}
