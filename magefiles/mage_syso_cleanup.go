//go:build mage
// +build mage

// Mage release cleanup helpers keep generated linker inputs out of committed state.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// cleanupWindowsSysoInputs removes root-level resource_windows_*.syso after linking.
func cleanupWindowsSysoInputs() error {
	root, err := absInRoot(".")
	if err != nil {
		return err
	}
	matches, err := rootWindowsSysoInputs(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, p := range matches {
		if remErr := fileOps.RemoveAll(p); remErr != nil {
			return fmt.Errorf("remove %s: %w", p, remErr)
		}
	}
	return nil
}

func rootWindowsSysoInputs(root string) ([]string, error) {
	var matches []string
	rootCanon := filepath.ToSlash(filepath.Clean(root))
	walkErr := fileOps.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() || filepath.ToSlash(filepath.Clean(filepath.Dir(path))) != rootCanon {
			return nil
		}
		matched, matchErr := filepath.Match("resource_windows_*.syso", filepath.Base(path))
		if matchErr != nil {
			return fmt.Errorf("match resource_windows_*.syso: %w", matchErr)
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})

	return matches, walkErr
}
