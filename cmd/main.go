package main

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	r "github.com/takiren/yqtui/internal/root"
)

func main() {
	p := tea.NewProgram(r.NewRootModel())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		return
	}
}
