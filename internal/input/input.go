// Package input resolves the YAML data yqtui operates on and arranges a
// keyboard input source for the TUI.
//
// Data can come from a file argument (`yqtui file.yaml`) or from a pipe
// (`cat file.yaml | yqtui`). When the data arrives via a pipe, os.Stdin is
// already consumed and is no longer a terminal, so /dev/tty is opened to drive
// the interactive UI.
package input

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

// ErrNoInput is returned when neither a file argument nor piped stdin is
// available, i.e. yqtui was run interactively with no data to view.
var ErrNoInput = errors.New("no input provided")

// Source is the resolved YAML input to operate on.
type Source struct {
	Data []byte // raw YAML bytes
	Name string // human-readable origin: a file path or "<stdin>"
}

// Load resolves the YAML input. If args contains a path, that file is read.
// Otherwise, if stdin is piped or redirected (not a terminal), stdin is read.
// If neither is available, ErrNoInput is returned.
func Load(args []string, stdin *os.File) (Source, error) {
	if len(args) > 0 {
		path := args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return Source{}, fmt.Errorf("reading %s: %w", path, err)
		}
		return Source{Data: data, Name: path}, nil
	}

	if !isTerminal(stdin) {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return Source{}, fmt.Errorf("reading stdin: %w", err)
		}
		return Source{Data: data, Name: "<stdin>"}, nil
	}

	return Source{}, ErrNoInput
}

// TUIInput returns the reader that should drive the terminal UI. When stdin is
// a terminal it is used directly. When stdin was piped (not a terminal), it
// cannot carry keystrokes, so /dev/tty is opened instead.
//
// The returned io.Closer is non-nil only when a new file was opened; the
// caller is responsible for closing it.
func TUIInput(stdin *os.File) (io.Reader, io.Closer, error) {
	if isTerminal(stdin) {
		return stdin, nil, nil
	}

	tty, err := os.Open("/dev/tty")
	if err != nil {
		return nil, nil, fmt.Errorf("opening /dev/tty for interactive input: %w", err)
	}
	return tty, tty, nil
}

// isTerminal reports whether f is an interactive terminal (tty), as opposed to
// a pipe, regular file, or non-tty character device such as /dev/null.
func isTerminal(f *os.File) bool {
	return f != nil && term.IsTerminal(f.Fd())
}
