package root

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/takiren/yqtui/internal/input"
)

// sized はモデルを生成し、指定サイズの WindowSizeMsg を与えた状態で返す。
func sized(t *testing.T, w, h int) Model {
	t.Helper()
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\n")})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model)
}

func TestView_RendersThreeRegions(t *testing.T) {
	v := sized(t, 80, 24).View()
	if !v.AltScreen {
		t.Error("AltScreen が有効になっていない")
	}
	for _, want := range []string{"補完候補", "プレビュー", "demo.yaml", "Ctrl+C"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("ビューに %q が含まれていない", want)
		}
	}
}

func TestView_RespondsToResize(t *testing.T) {
	for _, w := range []int{80, 40, 120} {
		got := lipgloss.Width(sized(t, w, 24).View().Content)
		if got != w {
			t.Errorf("幅 %d: 描画幅 = %d, want %d", w, got, w)
		}
	}
}

func TestView_SmallTerminalShowsPlaceholder(t *testing.T) {
	if !strings.Contains(sized(t, 10, 4).View().Content, "読み込み中") {
		t.Error("極端に小さい端末ではプレースホルダを表示すべき")
	}
}

// 長いクエリを入力しても入力バーが縦に伸びず、全体が端末サイズに収まること。
func TestView_LongQueryDoesNotOverflow(t *testing.T) {
	m := sized(t, 40, 14)
	for _, r := range strings.Repeat("abcdefghij.", 6) {
		updated, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	c := m.View().Content
	if h := lipgloss.Height(c); h != 14 {
		t.Errorf("長いクエリで縦あふれ: 高さ=%d, want 14", h)
	}
	if w := lipgloss.Width(c); w != 40 {
		t.Errorf("幅=%d, want 40", w)
	}
}

// 長いソース名でもプレビューペインが縦に伸びず、全体が端末サイズに収まること。
func TestView_LongSourceNameDoesNotOverflow(t *testing.T) {
	m := NewRootModel(input.Source{Name: strings.Repeat("very/long/path/", 8) + "f.yaml"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 14})
	if h := lipgloss.Height(updated.(Model).View().Content); h != 14 {
		t.Errorf("長い名前で縦あふれ: 高さ=%d, want 14", h)
	}
}

func TestUpdate_TypingAndBackspace(t *testing.T) {
	m := sized(t, 80, 24)
	for _, r := range "ab" {
		updated, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	if m.query != "ab" {
		t.Errorf("query = %q, want %q", m.query, "ab")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updated.(Model)
	if m.query != "a" {
		t.Errorf("backspace 後の query = %q, want %q", m.query, "a")
	}
}

// スペースキー（String() が "space" を返す）でも入力バーへ文字が追記されること。
// yq 式はスペースを多用するため、取りこぼすと式を入力できない。
func TestUpdate_TypesSpace(t *testing.T) {
	m := sized(t, 80, 24)
	for _, k := range []tea.KeyPressMsg{
		{Code: 'a', Text: "a"},
		{Code: tea.KeySpace, Text: " "},
		{Code: 'b', Text: "b"},
	} {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	if m.query != "a b" {
		t.Errorf("query = %q, want %q", m.query, "a b")
	}
}

func TestUpdate_EscQuits(t *testing.T) {
	m := sized(t, 80, 24)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc はコマンドを返すべき")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("esc は QuitMsg を返すべき, got %T", cmd())
	}
}
