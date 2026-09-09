package unpacker

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type decoderRunner struct {
	entries map[string]string
}

func (r decoderRunner) Run(_ context.Context, _ string, args []string, _, _ io.Writer) error {
	archivePath := strings.TrimSuffix(args[0], filepath.Ext(args[0])) + ".decoded.zip"
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	for name, contents := range r.entries {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			return createErr
		}
		if _, writeErr := entry.Write([]byte(contents)); writeErr != nil {
			return writeErr
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return file.Close()
}

func TestUnpackStagesDecoderAndExtractsArchive(t *testing.T) {
	root := t.TempDir()
	decoder := filepath.Join(root, "IntuneWinAppUtilDecoder.exe")
	packagePath := filepath.Join(root, "example.intunewin")
	writeTestFile(t, decoder, "decoder")
	writeTestFile(t, packagePath, "package")

	service := Service{DecoderPath: decoder, Runner: decoderRunner{entries: map[string]string{"IntuneWinPackage/metadata.txt": "decoded"}}}
	if err := service.Validate(); err != nil {
		t.Fatalf("Validate returned %v", err)
	}
	result := service.Unpack(context.Background(), packagePath, "", false, io.Discard, io.Discard)
	if result.Err != nil {
		t.Fatalf("Unpack returned %v", result.Err)
	}
	contents, err := os.ReadFile(filepath.Join(result.Destination, "IntuneWinPackage", "metadata.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(contents) != "decoded" {
		t.Fatalf("extracted contents = %q", contents)
	}
}

func TestUnpackRefusesExistingDestinationWithoutForce(t *testing.T) {
	root := t.TempDir()
	decoder := filepath.Join(root, "decoder.exe")
	packagePath := filepath.Join(root, "example.intunewin")
	destination := filepath.Join(root, "existing")
	writeTestFile(t, decoder, "decoder")
	writeTestFile(t, packagePath, "package")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	service := Service{DecoderPath: decoder, Runner: decoderRunner{}}
	result := service.Unpack(context.Background(), packagePath, destination, false, io.Discard, io.Discard)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "--force") {
		t.Fatalf("Unpack error = %v, want --force guidance", result.Err)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	for _, entryName := range []string{"../outside.txt", `..\outside.txt`, "/absolute.txt", `C:\absolute.txt`} {
		t.Run(entryName, func(t *testing.T) {
			root := t.TempDir()
			archivePath := filepath.Join(root, "unsafe.zip")
			file, err := os.Create(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			entry, err := writer.Create(entryName)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write([]byte("unsafe")); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}

			err = extractZip(archivePath, filepath.Join(root, "output"))
			if err == nil || !strings.Contains(err.Error(), "unsafe path") {
				t.Fatalf("extractZip error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
				t.Fatalf("outside file exists or stat failed unexpectedly: %v", err)
			}
		})
	}
}

func TestValidateDestinationRejectsFilesystemRoot(t *testing.T) {
	if err := validateDestination(string(filepath.Separator), filepath.Join(t.TempDir(), "package.intunewin")); err == nil {
		t.Fatal("validateDestination accepted filesystem root")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
