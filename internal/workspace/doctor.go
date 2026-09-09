package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CheckStatus string

const (
	CheckOK      CheckStatus = "ok"
	CheckWarning CheckStatus = "warning"
	CheckFailure CheckStatus = "failure"
)

type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail"`
}

func Diagnose(root string) []Check {
	ws, err := Open(root)
	if err != nil {
		return []Check{{Name: "Workspace configuration", Status: CheckFailure, Detail: err.Error()}}
	}
	checks := []Check{{Name: "Workspace configuration", Status: CheckOK, Detail: filepath.Join(ws.Root, ConfigFile)}}
	if info, err := os.Stat(ws.AppsDir()); err != nil || !info.IsDir() {
		checks = append(checks, Check{Name: "Applications directory", Status: CheckFailure, Detail: errorDetail(err, "path is not a directory")})
	} else {
		checks = append(checks, Check{Name: "Applications directory", Status: CheckOK, Detail: ws.AppsDir()})
	}
	utility := ws.Config.IntuneWinAppUtil
	if utility == "" {
		utility = ws.Config.IntuneWinAppUtilPath
	}
	checks = append(checks, executableCheck("Packaging utility", utility, false))
	checks = append(checks, executableCheck("Decoder", ws.Config.DecoderPath, true))
	apps, err := ws.List()
	if err != nil {
		checks = append(checks, Check{Name: "Applications", Status: CheckFailure, Detail: err.Error()})
		return checks
	}
	if len(apps) == 0 {
		checks = append(checks, Check{Name: "Applications", Status: CheckWarning, Detail: "no applications found"})
		return checks
	}
	invalid := 0
	for _, app := range apps {
		if app.Status != "valid" {
			invalid++
		}
	}
	if invalid > 0 {
		checks = append(checks, Check{Name: "Applications", Status: CheckFailure, Detail: fmt.Sprintf("%d of %d invalid", invalid, len(apps))})
	} else {
		checks = append(checks, Check{Name: "Applications", Status: CheckOK, Detail: fmt.Sprintf("%d valid", len(apps))})
	}
	return checks
}

func executableCheck(name, path string, optional bool) Check {
	if strings.TrimSpace(path) == "" {
		status := CheckFailure
		detail := "not configured"
		if optional {
			status, detail = CheckWarning, "not configured; unpack is unavailable"
		}
		return Check{Name: name, Status: status, Detail: detail}
	}
	if err := EnsureFile(path, strings.ToLower(name)); err != nil {
		status := CheckFailure
		if optional {
			status = CheckWarning
		}
		return Check{Name: name, Status: status, Detail: err.Error()}
	}
	return Check{Name: name, Status: CheckOK, Detail: path}
}

func errorDetail(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if errors.Is(err, os.ErrNotExist) {
		return "directory does not exist"
	}
	return err.Error()
}
