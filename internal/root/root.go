package root

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

type rootModel struct {
	choices  []string
	cursor   int
	selected map[int]struct{}
}

func NewRootModel() rootModel {
	return rootModel{
		choices:  []string{"model.yaml", "controller.yaml", "service.yaml"},
		selected: make(map[int]struct{}),
	}
}

func (r rootModel) Init() tea.Cmd {
	return nil
}

func (r rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyPressMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return r, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if r.cursor > 0 {
				r.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if r.cursor < len(r.choices)-1 {
				r.cursor++
			}

		// The "enter" key and the space bar toggle the selected state
		// for the item that the cursor is pointing at.
		case "enter", "space":
			_, ok := r.selected[r.cursor]
			if ok {
				delete(r.selected, r.cursor)
			} else {
				r.selected[r.cursor] = struct{}{}
			}
		}
	}
	return r, nil
}
func (r rootModel) View() tea.View {

	s := "What should we buy at the market?\n\n"

	for i, choice := range r.choices {

		cursor := " "
		if r.cursor == i {
			cursor = ">"
		}

		checked := " "
		if _, ok := r.selected[i]; ok {
			checked = "x"
		}

		s += fmt.Sprintf("%s %s %s\n", cursor, checked, choice)
	}

	s += "\nPress space to toggle a checkbox, up/down arrows to move, q or Ctrl+C to quit.\n"
	return tea.NewView(s)
}
