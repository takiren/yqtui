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

	ttyIn, inCloser, err := input.TUIInput(os.Stdin)
	if err != nil {
		return err
	}
	if inCloser != nil {
		defer func() {
			if err := inCloser.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "error: closing input:", err)
			}
		}()
	}

	// 描画は /dev/tty に分離し、stdout は確定した式の出力（fzf 風）専用にする。
	ttyOut, outCloser, err := input.TUIOutput()
	if err != nil {
		return err
	}
	defer func() {
		if err := outCloser.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "error: closing display:", err)
		}
	}()

	p := tea.NewProgram(r.NewRootModel(src), tea.WithInput(ttyIn), tea.WithOutput(ttyOut))
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("running program: %w", err)
	}

	// Enter で確定したときだけ、組み立てた式を stdout へ出力する（中断時は無出力）。
	if m, ok := final.(r.Model); ok {
		if expr, confirmed := m.Result(); confirmed {
			fmt.Fprintln(os.Stdout, expr)
		}
	}
	return nil
}
