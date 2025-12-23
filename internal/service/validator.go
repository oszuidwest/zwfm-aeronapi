// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
)

// validateBackup validates backup file integrity based on format.
func validateBackup(ctx context.Context, filePath, format, pgRestorePath string) error {
	if format == "custom" {
		return validateWithPgRestore(ctx, filePath, pgRestorePath)
	}
	return validatePlainSQL(ctx, filePath)
}

func validateWithPgRestore(ctx context.Context, filePath, pgRestorePath string) error {
	cmd := exec.CommandContext(ctx, pgRestorePath, "--list", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = err.Error()
		}
		return types.NewOperationError("backup validatie", fmt.Errorf("bestand is corrupt of onleesbaar: %s", errMsg))
	}
	return nil
}

func validatePlainSQL(ctx context.Context, filePath string) (returnErr error) {
	file, err := os.Open(filePath)
	if err != nil {
		return types.NewOperationError("backup validatie", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = types.NewOperationError("backup validatie", closeErr)
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return types.NewOperationError("backup validatie", err)
	}
	if stat.Size() == 0 {
		return types.NewOperationError("backup validatie", errors.New("bestand is leeg"))
	}

	// Check header (first 1KB)
	header := make([]byte, 1024)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return types.NewOperationError("backup validatie", err)
	}
	if !strings.Contains(string(header[:n]), "-- PostgreSQL database dump") {
		return types.NewOperationError("backup validatie", errors.New("ongeldige SQL dump - header ontbreekt"))
	}

	// Check footer (last 1KB)
	if stat.Size() > 1024 {
		if _, err = file.Seek(-1024, io.SeekEnd); err != nil {
			return types.NewOperationError("backup validatie", err)
		}
	} else {
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return types.NewOperationError("backup validatie", err)
		}
	}

	footer := make([]byte, 1024)
	n, err = file.Read(footer)
	if err != nil && err != io.EOF {
		return types.NewOperationError("backup validatie", err)
	}
	if !strings.Contains(string(footer[:n]), "-- PostgreSQL database dump complete") {
		return types.NewOperationError("backup validatie", errors.New("SQL dump is incompleet - footer ontbreekt"))
	}

	return nil
}
