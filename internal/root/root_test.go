package root

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/takiren/yqtui/internal/input"
)

// arrayYAML は配列を含む補完テスト用のサンプル。
const arrayYAML = `
name: demo
spec:
  replicas: 3
  containers:
    - name: web
      image: nginx
    - name: sidecar
      image: envoy
`

// ready はモデルを 80x24 で初期化し、初回の補完候補計算まで完了させて返す。
func ready(t *testing.T, data string) Model {
	t.Helper()
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte(data)})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	return pump(t, m, m.Init())
}

// press は1つのキー入力を与え、更新後のモデルと返却コマンドを返す。
func press(t *testing.T, m Model, code rune) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(tea.KeyPressMsg{Code: code})
	return updated.(Model), cmd
}

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

// typeQuery はモデルへ s の各文字をキー入力として与え、最後のキー入力が返す
// コマンド（デバウンス評価）を併せて返す。
func typeQuery(t *testing.T, m Model, s string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, r := range s {
		var updated tea.Model
		updated, cmd = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	return m, cmd
}

// pump はコマンドが返すメッセージをモデルに与え続け、評価パイプライン
// （デバウンス→非同期評価）を完了させる。Tick はテスト内で実時間だけ待つ。
func pump(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		updated, next := m.Update(msg)
		m = updated.(Model)
		cmd = next
	}
	return m
}

// 空入力ではドキュメント全体がプレビューされること。
func TestPreview_EmptyQueryShowsWholeDocument(t *testing.T) {
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\nb: 2\n")})
	m = pump(t, m, m.Init())
	for _, want := range []string{"a: 1", "b: 2"} {
		if !strings.Contains(m.preview, want) {
			t.Errorf("空入力のプレビューに %q が含まれていない:\n%s", want, m.preview)
		}
	}
}

// 入力した式が評価され、その結果がプレビューに反映されること。
func TestPreview_EvaluatesExpression(t *testing.T) {
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\nb: 2\n")})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	m, cmd := typeQuery(t, m, ".a")
	m = pump(t, m, cmd)
	if strings.TrimSpace(m.preview) != "1" {
		t.Errorf("式 .a のプレビュー = %q, want %q", strings.TrimSpace(m.preview), "1")
	}
	if !strings.Contains(m.View().Content, "1") {
		t.Error("評価結果が右ペインに表示されていない")
	}
}

// 無効な式では直前の有効なプレビューを保持し、エラー状態を記録すること。
func TestPreview_InvalidExpressionKeepsLastPreview(t *testing.T) {
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\n")})

	m, cmd := typeQuery(t, m, ".a")
	m = pump(t, m, cmd)
	prev := m.preview
	if strings.TrimSpace(prev) != "1" {
		t.Fatalf("前提: 有効な式のプレビュー = %q, want %q", strings.TrimSpace(prev), "1")
	}

	m, cmd = typeQuery(t, m, "[") // ".a[" は無効な式
	m = pump(t, m, cmd)
	if m.evalErr == nil {
		t.Error("無効な式では evalErr が設定されるべき")
	}
	if m.preview != prev {
		t.Errorf("無効な式でプレビューが変化した: %q, want %q", m.preview, prev)
	}
}

// 古い世代の評価結果は、より新しい入力があれば破棄されること。
func TestPreview_StaleResultIsDropped(t *testing.T) {
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\nb: 2\n")})

	// ".b" を入力すると gen は最新（2）へ進む。
	m, cmd := typeQuery(t, m, ".b")

	// それより古い世代（gen=1）の結果が遅れて届いても採用しない。
	updated, _ := m.Update(resultMsg{gen: 1, out: "stale\n"})
	m = updated.(Model)
	if strings.Contains(m.preview, "stale") {
		t.Errorf("古い世代の結果が採用された: %q", m.preview)
	}

	// 最新世代の評価を完了させると ".b" の結果になること。
	m = pump(t, m, cmd)
	if strings.TrimSpace(m.preview) != "2" {
		t.Errorf("最新クエリ .b のプレビュー = %q, want %q", strings.TrimSpace(m.preview), "2")
	}
}

// 起動直後はルート直下のキーが補完候補として並ぶこと。
func TestCompletion_ShowsRootKeysInitially(t *testing.T) {
	m := ready(t, arrayYAML)
	want := []string{"name", "spec"}
	if !reflect.DeepEqual(m.cands, want) {
		t.Errorf("初期候補 = %v, want %v", m.cands, want)
	}
}

// 入力した接頭辞で候補が絞り込まれること。
func TestCompletion_FiltersByPrefix(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".sp")
	m = pump(t, m, cmd)
	want := []string{"spec"}
	if !reflect.DeepEqual(m.cands, want) {
		t.Errorf(".sp の候補 = %v, want %v", m.cands, want)
	}
}

// 解決可能なパスまで打つと、その直下キーが候補になること。
func TestCompletion_ShowsChildKeysOfResolvedPath(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".spec")
	m = pump(t, m, cmd)
	want := []string{"replicas", "containers"}
	if !reflect.DeepEqual(m.cands, want) {
		t.Errorf(".spec の候補 = %v, want %v", m.cands, want)
	}
}

// ↑↓で選択が循環して移動すること。
func TestCompletion_Navigation(t *testing.T) {
	m := ready(t, arrayYAML) // 候補は name, spec の2件
	if m.selected != 0 {
		t.Fatalf("初期選択 = %d, want 0", m.selected)
	}
	m, _ = press(t, m, tea.KeyDown)
	if m.selected != 1 {
		t.Errorf("down 後の選択 = %d, want 1", m.selected)
	}
	m, _ = press(t, m, tea.KeyDown) // 末尾から先頭へ循環
	if m.selected != 0 {
		t.Errorf("循環後の選択 = %d, want 0", m.selected)
	}
	m, _ = press(t, m, tea.KeyUp) // 先頭から末尾へ循環
	if m.selected != 1 {
		t.Errorf("up 循環後の選択 = %d, want 1", m.selected)
	}
}

// TAB で選択中の候補が入力バーへ追記されること。
func TestCompletion_TabAppendsPath(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".sp")
	m = pump(t, m, cmd)

	m, cmd = press(t, m, tea.KeyTab)
	if m.query != ".spec" {
		t.Errorf("TAB 確定後の query = %q, want %q", m.query, ".spec")
	}
	// 確定後は新しいノードの直下キーへ候補が更新される。
	m = pump(t, m, cmd)
	want := []string{"replicas", "containers"}
	if !reflect.DeepEqual(m.cands, want) {
		t.Errorf("確定後の候補 = %v, want %v", m.cands, want)
	}
}

// 配列ノードではインデックスと [] ワイルドカードが候補になること。
func TestCompletion_ArrayIndicesAndWildcard(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".spec.containers")
	m = pump(t, m, cmd)
	want := []string{"[0]", "[1]", "[]"}
	if !reflect.DeepEqual(m.cands, want) {
		t.Errorf("配列の候補 = %v, want %v", m.cands, want)
	}
}

// 補完を繰り返して .spec.containers[].image のような反復パスが組めること。
func TestCompletion_BuildsIterationPath(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".spec.containers")
	m = pump(t, m, cmd) // 候補: [0] [1] []

	// "[]"（3件目）を選んで確定。
	m, _ = press(t, m, tea.KeyDown)
	m, _ = press(t, m, tea.KeyDown)
	m, cmd = press(t, m, tea.KeyTab)
	if m.query != ".spec.containers[]" {
		t.Fatalf("ワイルドカード確定後 = %q, want %q", m.query, ".spec.containers[]")
	}
	m = pump(t, m, cmd) // 候補: name image

	// "image"（2件目）を選んで確定。
	m, _ = press(t, m, tea.KeyDown)
	m, _ = press(t, m, tea.KeyTab)
	if m.query != ".spec.containers[].image" {
		t.Errorf("反復パス = %q, want %q", m.query, ".spec.containers[].image")
	}
}

// 候補リストが左ペインに描画され、選択中の候補が強調されること。
func TestView_ShowsCandidateList(t *testing.T) {
	m := ready(t, arrayYAML)
	content := m.View().Content
	if !strings.Contains(content, "▌") {
		t.Error("選択中候補のマーカー（▌）が描画されていない")
	}
	if !strings.Contains(content, "spec") {
		t.Error("候補 spec が描画されていない")
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
