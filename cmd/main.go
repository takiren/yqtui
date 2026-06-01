package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/takiren/yqtui/internal/input"
	r "github.com/takiren/yqtui/internal/root"
)

const usage = `yqtui — yq によるYAML検索ビューワー

使い方:
  yqtui <file.yaml>
  <command> | yqtui`

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, input.ErrNoInput) {
			fmt.Fprintln(os.Stderr, usage)
		} else {
			fmt.Fprintln(os.Stderr, "yqtui:", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	src, err := input.Load(args, os.Stdin)
	if err != nil {
		return err
	}

	ttyIn, closer, err := input.TUIInput(os.Stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	p := tea.NewProgram(r.NewRootModel(src), tea.WithInput(ttyIn))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running program: %w", err)
	}
	return nil
}
