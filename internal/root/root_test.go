package root

import (
	"fmt"
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

// --- stdout出力で終了（#14） ---

// Enter で式を確定し、QuitMsg を返すこと。確定した式は Result で取り出せること。
func TestUpdate_EnterConfirmsAndQuits(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".spec")
	m = pump(t, m, cmd)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("enter はコマンドを返すべき")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("enter は QuitMsg を返すべき, got %T", cmd())
	}
	expr, confirmed := m.Result()
	if !confirmed {
		t.Error("enter 後は confirmed であるべき")
	}
	if expr != ".spec" {
		t.Errorf("確定した式 = %q, want %q", expr, ".spec")
	}
}

// 空入力で Enter すると恒等式 "." を出力すること。
func TestUpdate_EnterEmptyQueryOutputsIdentity(t *testing.T) {
	m := ready(t, arrayYAML)
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	expr, confirmed := updated.(Model).Result()
	if !confirmed || expr != "." {
		t.Errorf("空入力の確定 = (%q, %v), want (\".\", true)", expr, confirmed)
	}
}

// 中断（Esc/Ctrl+C）では確定せず、何も出力しないこと。
func TestUpdate_AbortDoesNotConfirm(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".spec")
	m = pump(t, m, cmd)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, confirmed := updated.(Model).Result(); confirmed {
		t.Error("中断では confirmed であってはならない")
	}
}

// --- エラー/該当なし処理（#16） ---

// 無効な式のときフッターにエラー要旨が表示され、プレビューは保持されること。
func TestView_ErrorShowsHintAndKeepsPreview(t *testing.T) {
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\n")})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	m, cmd := typeQuery(t, m, ".a")
	m = pump(t, m, cmd)

	m, cmd = typeQuery(t, m, "[") // ".a[" は無効な式
	m = pump(t, m, cmd)
	if m.evalErr == nil {
		t.Fatal("前提: 無効な式で evalErr が設定されるべき")
	}

	content := m.View().Content
	if !strings.Contains(content, "✗") {
		t.Error("エラー時はプロンプト/フッターに ✗ インジケータを出すべき")
	}
	// 直前の有効なプレビュー（1）は描画され続ける。
	if !strings.Contains(content, "1") {
		t.Error("エラー時も直前のプレビューを保持すべき")
	}
}

// 式は有効だが結果が空のときは「該当なし」を表示すること。
func TestView_EmptyResultShowsNoMatch(t *testing.T) {
	m := NewRootModel(input.Source{Name: "demo.yaml", Data: []byte("a: 1\n")})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	m, cmd := typeQuery(t, m, ".[] | select(false)") // 有効だがマッチなし
	m = pump(t, m, cmd)
	if m.evalErr != nil {
		t.Fatalf("前提: 有効な式なので evalErr は nil のはず: %v", m.evalErr)
	}
	if !strings.Contains(m.View().Content, "該当なし") {
		t.Error("結果が空のときは「該当なし」を表示すべき")
	}
}

// oneLine は改行・連続空白を1つの空白に畳むこと。
func TestOneLine_CollapsesWhitespace(t *testing.T) {
	if got := oneLine("bad expression\n  could not find"); got != "bad expression could not find" {
		t.Errorf("oneLine = %q", got)
	}
}

// --- yq式のOSC52コピー（#15） ---

// Ctrl+Y で式コピーのコマンドが返り、フィードバックがフッターに出ること。
func TestCopy_CtrlYCopiesExprAndShowsNotice(t *testing.T) {
	m := ready(t, arrayYAML)
	m, cmd := typeQuery(t, m, ".spec")
	m = pump(t, m, cmd)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Ctrl+Y は OSC52 コピーのコマンドを返すべき")
	}
	if !strings.Contains(m.notice, ".spec") {
		t.Errorf("コピーした式がフィードバックに含まれない: %q", m.notice)
	}
	if !strings.Contains(m.View().Content, "コピー") {
		t.Error("コピー成功のフィードバックがフッターに表示されていない")
	}
}

// 空入力での Ctrl+Y は恒等式 "." をコピーすること。
func TestCopy_EmptyQueryCopiesIdentity(t *testing.T) {
	m := ready(t, arrayYAML)
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Ctrl+Y はコマンドを返すべき")
	}
	if !strings.Contains(m.notice, ".") {
		t.Errorf("空入力では恒等式 . をコピーすべき: %q", m.notice)
	}
}

// フィードバックは次のキー入力で消えること。
func TestCopy_NoticeClearsOnNextKey(t *testing.T) {
	m := ready(t, arrayYAML)
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	m = updated.(Model)
	if m.notice == "" {
		t.Fatal("前提: Ctrl+Y でフィードバックが設定されること")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(Model)
	if m.notice != "" {
		t.Errorf("次のキー入力でフィードバックが消えるべき: %q", m.notice)
	}
}

// --- プレビューの viewport スクロール（#13） ---

// maxScroll / clampScroll の境界が想定どおりであること。
func TestScroll_ClampBounds(t *testing.T) {
	if got := maxScroll(5, 10); got != 0 {
		t.Errorf("収まる場合の maxScroll = %d, want 0", got)
	}
	if got := maxScroll(30, 10); got != 20 {
		t.Errorf("あふれる場合の maxScroll = %d, want 20", got)
	}
	if got := clampScroll(-5, 30, 10); got != 0 {
		t.Errorf("負のスクロールは 0 に丸めるべき, got %d", got)
	}
	if got := clampScroll(100, 30, 10); got != 20 {
		t.Errorf("過大なスクロールは maxScroll に丸めるべき, got %d", got)
	}
}

// longPreview は n 行のプレビューを持つ、ウィンドウサイズ済みのモデルを返す。
func longPreview(t *testing.T, n int) Model {
	t.Helper()
	m := sized(t, 80, 24)
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i)
	}
	m.preview = strings.Join(lines, "\n")
	return m
}

// pressKey は msg.String() が want と一致することを確認してから Update に渡す。
// キー構築の取り違えで「何もしないキー」を送って誤って成功するのを防ぐ。
func pressKey(t *testing.T, m Model, msg tea.KeyPressMsg, want string) Model {
	t.Helper()
	if got := msg.String(); got != want {
		t.Fatalf("キー構築の取り違え: String() = %q, want %q", got, want)
	}
	updated, _ := m.Update(msg)
	return updated.(Model)
}

// Ctrl+J / Ctrl+K で1行ずつスクロールし、上端・下端でクランプされること。
func TestScroll_LineByLine(t *testing.T) {
	m := longPreview(t, 60) // 表示窓よりはるかに長い
	viewH := previewViewportH(m.bodyOuterH())

	m = pressKey(t, m, tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}, "ctrl+j")
	if m.scroll != 1 {
		t.Errorf("Ctrl+J 1回で scroll = %d, want 1", m.scroll)
	}
	m = pressKey(t, m, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}, "ctrl+k")
	if m.scroll != 0 {
		t.Errorf("Ctrl+K で戻して scroll = %d, want 0", m.scroll)
	}
	// 上端でさらに Ctrl+K しても 0 のまま。
	m = pressKey(t, m, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}, "ctrl+k")
	if m.scroll != 0 {
		t.Errorf("上端クランプ: scroll = %d, want 0", m.scroll)
	}
	// 下端を超えてもクランプされること。
	for range 100 {
		m = pressKey(t, m, tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}, "ctrl+j")
	}
	if want := maxScroll(60, viewH); m.scroll != want {
		t.Errorf("下端クランプ: scroll = %d, want %d", m.scroll, want)
	}
}

// PgDn / PgUp で表示窓ぶんだけページスクロールすること。
func TestScroll_PageUpDown(t *testing.T) {
	m := longPreview(t, 60)
	viewH := previewViewportH(m.bodyOuterH())

	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown}, "pgdown")
	if m.scroll != viewH {
		t.Errorf("PgDn 1回で scroll = %d, want %d", m.scroll, viewH)
	}
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp}, "pgup")
	if m.scroll != 0 {
		t.Errorf("PgUp で戻して scroll = %d, want 0", m.scroll)
	}
}

// 短いプレビュー（表示窓に収まる）はスクロールしないこと。
func TestScroll_ShortPreviewDoesNotScroll(t *testing.T) {
	m := longPreview(t, 3)
	m = pressKey(t, m, tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}, "ctrl+j")
	if m.scroll != 0 {
		t.Errorf("収まる内容では scroll = %d, want 0", m.scroll)
	}
}

// 内容が更新されたらスクロール位置が先頭へリセットされること。
func TestScroll_ResetsOnContentChange(t *testing.T) {
	m := longPreview(t, 60)
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown}, "pgdown")
	if m.scroll == 0 {
		t.Fatal("前提: スクロール済みであること")
	}
	// 別内容の評価結果が最新世代で届くと先頭へ戻る。
	updated, _ := m.Update(resultMsg{gen: m.gen, out: "new-content\n"})
	m = updated.(Model)
	if m.scroll != 0 {
		t.Errorf("内容更新後の scroll = %d, want 0", m.scroll)
	}
}

// スクロール位置から表示窓ぶんの行だけが描画され、窓外の行は出ないこと。
func TestView_ScrollsPreviewWindow(t *testing.T) {
	m := longPreview(t, 60)
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown}, "pgdown")

	content := m.View().Content
	if strings.Contains(content, "line-00") {
		t.Error("スクロール後も先頭行 line-00 が表示されている")
	}
	viewH := previewViewportH(m.bodyOuterH())
	want := fmt.Sprintf("line-%02d", viewH) // 窓の先頭に来るはずの行
	if !strings.Contains(content, want) {
		t.Errorf("スクロール後に %q が表示されていない", want)
	}
}

// 長いプレビューをスクロールしてもレイアウトが縦横にあふれないこと。
func TestView_ScrolledPreviewDoesNotOverflow(t *testing.T) {
	m := longPreview(t, 200)
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown}, "pgdown")
	c := m.View().Content
	if h := lipgloss.Height(c); h != 24 {
		t.Errorf("縦あふれ: 高さ=%d, want 24", h)
	}
	if w := lipgloss.Width(c); w != 80 {
		t.Errorf("横あふれ: 幅=%d, want 80", w)
	}
}
