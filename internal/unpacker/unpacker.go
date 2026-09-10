package unpacker

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/workspace"
)

const DecoderProjectURL = "https://github.com/okieselbach/Intune/tree/master/IntuneWinAppUtilDecoder"

type Service struct {
	DecoderPath string
	Runner      process.Runner
}

type Result struct {
	PackagePath string
	Destination string
	Err         error
}

func (s Service) Validate() error {
	if strings.TrimSpace(s.DecoderPath) == "" {
		return fmt.Errorf("Package decoder is not configured; use 'inpakker tools install decoder' or obtain it from %s", DecoderProjectURL)
	}
	if s.Runner == nil {
		return errors.New("process runner is required")
	}
	return workspace.EnsureFile(s.DecoderPath, "decoder")
}

func (s Service) Unpack(ctx context.Context, packagePath, destination string, force bool, stdout, stderr io.Writer) Result {
	result := Result{PackagePath: packagePath, Destination: destination}
	absPackage, err := filepath.Abs(packagePath)
	if err != nil {
		result.Err = fmt.Errorf("resolve package path: %w", err)
		return result
	}
	if err := workspace.EnsureFile(absPackage, "package"); err != nil {
		result.Err = err
		return result
	}
	if !strings.EqualFold(filepath.Ext(absPackage), ".intunewin") {
		result.Err = errors.New("package must have an .intunewin extension")
		return result
	}

	if destination == "" {
		name := strings.TrimSuffix(filepath.Base(absPackage), filepath.Ext(absPackage))
		destination = filepath.Join(filepath.Dir(absPackage), "decoded", name)
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		result.Err = fmt.Errorf("resolve destination: %w", err)
		return result
	}
	if err := validateDestination(absDestination, absPackage); err != nil {
		result.Err = err
		return result
	}
	result.PackagePath, result.Destination = absPackage, absDestination

	if info, statErr := os.Stat(absDestination); statErr == nil {
		if !force {
			result.Err = fmt.Errorf("destination %q already exists; use --force to replace it", absDestination)
			return result
		}
		if !info.IsDir() {
			result.Err = fmt.Errorf("destination %q is not a directory", absDestination)
			return result
		}
	} else if !os.IsNotExist(statErr) {
		result.Err = fmt.Errorf("check destination: %w", statErr)
		return result
	}

	workDir, err := os.MkdirTemp("", "inpakker-unpack-*")
	if err != nil {
		result.Err = fmt.Errorf("create temporary directory: %w", err)
		return result
	}
	defer os.RemoveAll(workDir)

	stagedPackage := filepath.Join(workDir, filepath.Base(absPackage))
	if err := copyFile(absPackage, stagedPackage); err != nil {
		result.Err = fmt.Errorf("stage package: %w", err)
		return result
	}
	if err := s.Runner.Run(ctx, s.DecoderPath, []string{stagedPackage, "/s"}, stdout, stderr); err != nil {
		result.Err = fmt.Errorf("run decoder: %w", err)
		return result
	}
	decodedZip := strings.TrimSuffix(stagedPackage, filepath.Ext(stagedPackage)) + ".decoded.zip"
	if err := workspace.EnsureFile(decodedZip, "decoded archive"); err != nil {
		result.Err = err
		return result
	}

	if err := os.MkdirAll(filepath.Dir(absDestination), 0o755); err != nil {
		result.Err = fmt.Errorf("create destination parent: %w", err)
		return result
	}
	stagingDestination, err := os.MkdirTemp(filepath.Dir(absDestination), ".inpakker-extract-*")
	if err != nil {
		result.Err = fmt.Errorf("create extraction directory: %w", err)
		return result
	}
	defer os.RemoveAll(stagingDestination)
	if err := extractZip(decodedZip, stagingDestination); err != nil {
		result.Err = fmt.Errorf("extract decoded archive: %w", err)
		return result
	}
	if force {
		if err := os.RemoveAll(absDestination); err != nil {
			result.Err = fmt.Errorf("replace destination: %w", err)
			return result
		}
	}
	if err := os.Rename(stagingDestination, absDestination); err != nil {
		result.Err = fmt.Errorf("finalize destination: %w", err)
		return result
	}
	return result
}

func validateDestination(path, packagePath string) error {
	clean := filepath.Clean(path)
	if clean == filepath.Dir(clean) {
		return errors.New("destination cannot be a filesystem root")
	}
	workingDirectory, err := os.Getwd()
	if err == nil && samePath(clean, workingDirectory) {
		return errors.New("destination cannot be the current working directory")
	}
	if samePath(clean, filepath.Dir(packagePath)) {
		return errors.New("destination cannot be the package directory")
	}
	return nil
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func extractZip(zipPath, destination string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !pathutil.IsSafeRelative(file.Name) {
			return fmt.Errorf("archive entry has an unsafe path: %q", file.Name)
		}
		target := filepath.Join(destination, file.Name)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive entry escapes destination: %q", file.Name)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive entry is a symbolic link: %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.Mode().Perm())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		inputErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if inputErr != nil {
			return inputErr
		}
	}
	return nil
}
