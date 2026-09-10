// Package atomicfile writes complete files before making them visible.
package atomicfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func JSON(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	return Write(path, append(data, '\n'), mode)
}

func Write(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".inpakker-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	if err := Replace(f.Name(), path); err != nil {
		return fmt.Errorf("replace %q: %w", path, err)
	}
	return nil
}
