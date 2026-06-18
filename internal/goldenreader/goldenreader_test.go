package goldenreader

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestReadGolden_rejectsLeavingFeaturesDirectory(t *testing.T) {
	t.Parallel()

	_, err := ReadGolden(filepath.Join("testdata", "..", "..", "..", "go.mod"))
	if err == nil {
		t.Fatal("want error")
	}

	if !errors.Is(err, ErrFixturePath) {
		t.Fatalf("want errors.Is(err, ErrFixturePath): %v", err)
	}
}

func TestReadGolden_readsCommittedInput(t *testing.T) {
	t.Parallel()

	data, err := ReadGolden(filepath.Join("testdata", "inputs", "three-lines.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("want non-empty fixture bytes")
	}
}
