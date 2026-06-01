package root

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/takiren/yqtui/internal/input"
)

type rootModel struct {
	source   input.Source
	choices  []string
	cursor   int
	selected map[int]struct{}
}

func NewRootModel(source input.Source) rootModel {
	return rootModel{
		source:   source,
		choices:  []string{"model.yaml", "controller.yaml", "service.yaml"},
		selected: make(map[int]struct{}),
	}
}

func (r rootModel) Init() tea.Cmd {
	return nil
}

func (r rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// キー入力かどうか
	case tea.KeyPressMsg:

		// 実際に押されたキーで分岐する
		switch msg.String() {

		// プログラムを終了するキー
		case "ctrl+c", "q":
			return r, tea.Quit

		// 「up」「k」でカーソルを上に移動
		case "up", "k":
			if r.cursor > 0 {
				r.cursor--
			}

		// 「down」「j」でカーソルを下に移動
		case "down", "j":
			if r.cursor < len(r.choices)-1 {
				r.cursor++
			}

		// 「enter」「space」でカーソル位置の項目の選択状態を切り替える
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

	s := fmt.Sprintf("yqtui — %s (%d bytes)\n\n", r.source.Name, len(r.source.Data))

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

	s += "\nspaceで選択切り替え、上下矢印で移動、qまたはCtrl+Cで終了します。\n"
	return tea.NewView(s)
}
