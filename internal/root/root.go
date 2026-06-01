// Package root はアプリ全体の bubbletea モデルを提供します。
//
// この段階（#6）ではレイアウトの「枠」だけを用意します。上部に入力バー、
// 下部を左右に分割（左：補完候補、右：プレビュー）した3領域構成で、
// 端末サイズの変化に追従します。式の評価・補完・プレビューの中身は後続の
// Issue で実装します。
package root

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/takiren/yqtui/internal/input"
)

// Model はアプリ全体の状態を保持します。
type Model struct {
	source input.Source
	width  int
	height int
	query  string // 入力バーの内容（編集の本実装は後続Issue）
}

// NewRootModel は読み込み済みの入力からモデルを生成します。
func NewRootModel(source input.Source) Model {
	return Model{source: source}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		// 中断
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "backspace":
			if r := []rune(m.query); len(r) > 0 {
				m.query = string(r[:len(r)-1])
			}

		default:
			// 1文字のキーは入力バーへ追記する（暫定。本格的な行編集は後続）。
			if s := msg.String(); len([]rune(s)) == 1 {
				m.query += s
			}
		}
	}
	return m, nil
}

var (
	borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	titleStyle  = lipgloss.NewStyle().Bold(true)
	hintStyle   = lipgloss.NewStyle().Faint(true)
	footerStyle = lipgloss.NewStyle().Faint(true)
)

func (m Model) View() tea.View {
	if m.width < 20 || m.height < 8 {
		// 最初のサイズ通知前、または端末が極端に小さい場合。
		v := tea.NewView("読み込み中…（端末を広げてください）")
		v.AltScreen = true
		return v
	}

	const (
		barOuterH = 3 // 入力バー（枠込み）
		footerH   = 1
	)
	bodyOuterH := m.height - barOuterH - footerH
	leftOuterW := m.width / 3
	rightOuterW := m.width - leftOuterW

	// 入力バー。lipgloss v2 の Width/Height は枠線を含む総サイズを指す。
	// （本物のカーソル表示は後続Issueで対応する）
	bar := borderStyle.Width(m.width).Height(barOuterH).
		Render("> " + m.query)

	// 下段：左（補完候補）／右（プレビュー）
	left := borderStyle.Width(leftOuterW).Height(bodyOuterH).Render(
		titleStyle.Render("補完候補") + "\n\n" +
			hintStyle.Render("(#7 以降で実装)"),
	)
	right := borderStyle.Width(rightOuterW).Height(bodyOuterH).Render(
		titleStyle.Render("プレビュー") + "\n" +
			hintStyle.Render(fmt.Sprintf("%s (%d bytes)", m.source.Name, len(m.source.Data))) + "\n\n" +
			hintStyle.Render("(ライブ評価は #8 以降で実装)"),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := footerStyle.Render("Ctrl+C / Esc: 終了")

	content := strings.Join([]string{bar, body, footer}, "\n")
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
