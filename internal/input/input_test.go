package input

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FromFileArgument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.yaml")
	want := []byte("name: demo\n")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}

	src, err := Load([]string{path}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(src.Data) != string(want) {
		t.Errorf("data = %q, want %q", src.Data, want)
	}
	if src.Name != path {
		t.Errorf("name = %q, want %q", src.Name, path)
	}
}

func TestLoad_MissingFileErrors(t *testing.T) {
	_, err := Load([]string{filepath.Join(t.TempDir(), "nope.yaml")}, nil)
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestLoad_FromPipedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	want := "spec:\n  replicas: 3\n"
	go func() {
		_, _ = w.WriteString(want)
		_ = w.Close()
	}()

	src, err := Load(nil, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(src.Data) != want {
		t.Errorf("data = %q, want %q", src.Data, want)
	}
	if src.Name != "<stdin>" {
		t.Errorf("name = %q, want %q", src.Name, "<stdin>")
	}
}

func TestLoad_EmptyPipedStdin(t *testing.T) {
	// すぐに閉じたパイプは端末ではないため、Load はそれを読み、ErrNoInput では
	// なく空データを返す。（ErrNoInput には stdin が本物の tty である必要があり、
	// テストで移植性よく再現できないため、ここではパイプの挙動を確認する。）
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	defer func() {
		err := r.Close()
		if err != nil {
			t.Errorf("error closing input pipe: %v", err)
		}
	}()

	// Reading an empty closed pipe yields empty data, not ErrNoInput.
	src, err := Load(nil, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(src.Data) != 0 {
		t.Errorf("expected empty data, got %q", src.Data)
	}
}

func TestErrNoInput_IsSentinel(t *testing.T) {
	if !errors.Is(ErrNoInput, ErrNoInput) {
		t.Fatal("ErrNoInput should match itself via errors.Is")
	}
}
