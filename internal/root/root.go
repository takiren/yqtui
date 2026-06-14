// Package root はアプリ全体の bubbletea モデルを提供します。
//
// 上部に入力バー、下部を左右に分割（左：補完候補、右：プレビュー）した3領域
// 構成で、端末サイズの変化に追従します。入力バーの式は yq で評価して右ペインに
// プレビューします。評価はキー入力ごとに UI スレッドをブロックしないよう、
// デバウンスしたうえで tea.Cmd として非同期に実行します（#30）。
//
// 同じ非同期コマンドの中で、左ペインに出す補完候補も併せて計算します（#10）。
// 入力中の末尾トークンに応じて候補をインクリメンタルに絞り込み、TAB で確定すると
// 入力バーへパス断片を追記します。↑↓ / Ctrl+N,P で候補を移動できます。
package root

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/takiren/yqtui/internal/input"
	"github.com/takiren/yqtui/internal/yq"
)

// evalDebounce は連続入力中の無駄な評価を抑えるための待ち時間です。最後の
// キー入力からこの時間が経過してから評価を開始します。
const evalDebounce = 80 * time.Millisecond

// resultMsg は非同期処理の結果です。右ペイン用のプレビュー（out/err）と、左ペイン
// 用の補完候補（cands/replaceAt）を一度に運びます。両者は同じクエリ・同じ世代から
// 計算されるため1メッセージにまとめています。gen はどの入力世代に対する結果かを
// 表し、最新世代でなければ（より新しい入力があれば）破棄されます。
type resultMsg struct {
	gen       uint64
	out       string
	err       error
	cands     []string // 絞り込み済みの補完候補（表示順）
	replaceAt int      // 確定時に query[:replaceAt] を残してそこへ候補を挿入する
}

// debounceMsg はデバウンス満了の通知です。gen が最新ならその時点のクエリで
// 評価を開始します。
type debounceMsg struct{ gen uint64 }

// Model はアプリ全体の状態を保持します。
type Model struct {
	source  input.Source
	width   int
	height  int
	query   string // 入力バーの内容（編集の本実装は後続Issue）
	gen     uint64 // 入力世代。クエリが変わるたびに増える。古い評価結果の破棄に使う
	preview string // 直近で評価に成功した結果YAML
	evalErr error  // 直近の評価エラー。nil なら preview は最新

	cands     []string // 現在の補完候補（絞り込み済み）
	selected  int      // ハイライト中の候補インデックス
	replaceAt int      // 確定時に query[:replaceAt] を残して候補を挿入する位置

	scroll int // プレビューの先頭に表示する行オフセット（viewport の縦スクロール量）
}

// previewLines は現在のプレビューを行配列に分解して返します。スクロール量の計算と
// 描画の両方で同じ分解を使うため、ヘルパにまとめています。
func (m Model) previewLines() []string {
	return strings.Split(strings.TrimRight(m.preview, "\n"), "\n")
}

// previewViewportH はプレビュー本文（ヘッダ2行を除いた領域）の表示可能行数を返します。
// レイアウトの bodyOuterH（プレビューボックスの外形高さ・枠込み）から、枠線2行と
// ヘッダ＋空行の2行を差し引いた内寸です。
func previewViewportH(bodyOuterH int) int {
	h := bodyOuterH - 2 /* 枠線 */ - 2 /* ヘッダ＋空行 */
	if h < 1 {
		return 1
	}
	return h
}

// maxScroll はプレビューが viewH 行に収まらないときの、スクロール可能な最大行数を
// 返します（最終行が表示窓の末尾に来る位置）。収まる場合は 0 です。
func maxScroll(totalLines, viewH int) int {
	if totalLines <= viewH {
		return 0
	}
	return totalLines - viewH
}

// clampScroll はスクロール量を [0, maxScroll] に収めて返します。内容更新や端末
// リサイズで表示可能行数や総行数が変わっても、窓が末尾を超えないようにします。
func clampScroll(scroll, totalLines, viewH int) int {
	if scroll < 0 {
		return 0
	}
	if m := maxScroll(totalLines, viewH); scroll > m {
		return m
	}
	return scroll
}

// bodyOuterH は現在の端末サイズからプレビューボックスの外形高さ（枠込み）を求めます。
// View() のレイアウト計算と、キー入力時のスクロール量クランプで同じ値を使うため
// 切り出しています。
func (m Model) bodyOuterH() int {
	const (
		barOuterH = 3
		footerH   = 1
	)
	return m.height - barOuterH - footerH
}

// NewRootModel は読み込み済みの入力からモデルを生成します。
func NewRootModel(source input.Source) Model {
	return Model{source: source}
}

// Init は起動直後に、空入力としてドキュメント全体をデバウンスなしで評価します。
func (m Model) Init() tea.Cmd {
	return evalCmd(m.gen, m.query, m.source.Data)
}

// exprFor は入力バーの内容を yq 式へ変換します。空入力は恒等式 "." とみなし、
// ドキュメント全体を表示します。
func exprFor(query string) string {
	if strings.TrimSpace(query) == "" {
		return "."
	}
	return query
}

// scheduleEval は現在の世代の評価を、デバウンス後に発火させるコマンドを返します。
func (m Model) scheduleEval() tea.Cmd {
	gen := m.gen
	return tea.Tick(evalDebounce, func(time.Time) tea.Msg {
		return debounceMsg{gen: gen}
	})
}

// evalCmd は yq 評価と補完候補の計算を UI スレッドの外（コマンドのゴルーチン）で
// まとめて実行し、結果を resultMsg として返します。どちらも YAML のパースを伴う
// ため、入力ごとに同期実行せずデバウンス後に非同期で走らせます。
func evalCmd(gen uint64, query string, data []byte) tea.Cmd {
	return func() tea.Msg {
		out, err := yq.Evaluate(exprFor(query), data)
		cands, at := completionsFor(query, data)
		return resultMsg{gen: gen, out: out, err: err, cands: cands, replaceAt: at}
	}
}

// isWordChar はパスのキー名やインデックスを構成する文字（補完の絞り込みトークンと
// みなす文字）かを判定します。
func isWordChar(b byte) bool {
	return b == '_' || b == '-' ||
		('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

// completionsFor は現在のクエリに対する補完候補を、絞り込んだ状態で返します。
// あわせて、確定時に候補を挿入する位置 replaceAt（query 内のバイト位置）も返します。
//
// まずクエリ全体がマップ/配列に解決するなら、その直下候補を末尾に追記する形で
// 提示します（例: ".spec" まで打つと replicas/template/... が出る）。解決しない
// （入力途中）の場合は、末尾の編集中トークンを切り出し、親ノードの候補をそのトークン
// で前方一致絞り込みします（例: ".spec.con" で containers に絞る）。
func completionsFor(query string, data []byte) (cands []string, replaceAt int) {
	if ck, err := yq.ChildKeys(query, data); err == nil && len(ck) > 0 {
		return ck, len(query)
	}

	evalPath, prefix, at := splitForCompletion(query)
	ck, err := yq.ChildKeys(evalPath, data)
	if err != nil {
		return nil, len(query)
	}
	lower := strings.ToLower(prefix)
	for _, c := range ck {
		if strings.HasPrefix(strings.ToLower(c), lower) {
			cands = append(cands, c)
		}
	}
	return cands, at
}

// splitForCompletion はクエリを「候補を列挙する対象パス evalPath」「絞り込みトークン
// prefix」「確定時の挿入位置 replaceAt」へ分解します。末尾の語トークンを prefix と
// して切り出し、その直前の区切り文字（"." や "["）で対象パスを決めます。
func splitForCompletion(query string) (evalPath, prefix string, replaceAt int) {
	i := len(query)
	for i > 0 && isWordChar(query[i-1]) {
		i--
	}
	prefix = query[i:]
	head := query[:i]

	switch {
	case strings.HasSuffix(head, "["):
		// 配列インデックスの入力途中（例: ".spec.containers[0"）。配列ノードを
		// 対象に "[0]"/"[]" を候補化し、"[" ごと置換できるよう位置を戻す。
		evalPath = strings.TrimSuffix(head, "[")
		prefix = "[" + prefix
		replaceAt = i - 1
	case strings.HasSuffix(head, "."):
		// マップキーの入力途中（例: ".spec.con"）。"." の前を対象にする。
		evalPath = strings.TrimSuffix(head, ".")
		replaceAt = i
	default:
		// 先頭からの語（例: "spec"）など。そのまま対象にする。
		evalPath = head
		replaceAt = i
	}
	return evalPath, prefix, replaceAt
}

// applyCompletion は選択中の候補をクエリへ挿入した結果を返します。配列断片
// （"[" 始まり）はそのまま連結し、マップキーは直前に "." が無ければ補って連結
// します。
func applyCompletion(query string, replaceAt int, candidate string) string {
	if replaceAt < 0 || replaceAt > len(query) {
		replaceAt = len(query)
	}
	left := query[:replaceAt]
	if strings.HasPrefix(candidate, "[") || strings.HasSuffix(left, ".") {
		return left + candidate
	}
	return left + "." + candidate
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case debounceMsg:
		// 満了後に新しい入力が無ければ（最新世代なら）評価を開始する。
		if msg.gen == m.gen {
			return m, evalCmd(m.gen, m.query, m.source.Data)
		}

	case resultMsg:
		// 古い世代の結果は破棄し、最新クエリの結果だけを採用する。プレビューが
		// 失敗しても直前の有効なプレビューを保持し、エラー状態だけを記録する。
		// 補完候補は最新の絞り込み結果へ差し替え、選択を先頭へ戻す。
		if msg.gen == m.gen {
			if msg.err != nil {
				m.evalErr = msg.err
			} else {
				m.evalErr = nil
				// プレビュー本文が変わったらスクロール位置を先頭へ戻す。
				// 別のクエリへ切り替えれば内容は別物になるため、前の位置を引き
				// 継ぐ意味が薄く、混乱を避けて先頭表示が分かりやすいと判断した。
				// 同じ内容で再評価されただけのときは位置を保つ。
				if msg.out != m.preview {
					m.preview = msg.out
					m.scroll = 0
				}
			}
			m.cands = msg.cands
			m.replaceAt = msg.replaceAt
			m.selected = 0
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		// 中断
		case "ctrl+c", "esc":
			return m, tea.Quit

		// 候補移動（上）。候補が無ければ何もしない。
		case "up", "ctrl+p":
			if n := len(m.cands); n > 0 {
				m.selected = (m.selected - 1 + n) % n
			}
			return m, nil

		// 候補移動（下）。
		case "down", "ctrl+n":
			if n := len(m.cands); n > 0 {
				m.selected = (m.selected + 1) % n
			}
			return m, nil

		// プレビュー（右ペイン）の縦スクロール。
		//
		// キーバインドの両立について：入力バーは常にフォーカスを持ち、印字可能キーは
		// すべて yq クエリへ追記される（msg.Text 方針, CLAUDE.md 参照）。そのため
		// 素の j/k をスクロールに割り当てると ".jobs" のようなクエリが打てなくなる。
		// 完了条件の「j/k」はクエリ編集を壊さない Ctrl+J / Ctrl+K に割り当てて満たす
		// （Ctrl 付きキーは msg.Text が空で、クエリには流れない）。あわせて、矢印と
		// 競合しない PgUp/PgDn をページスクロールに割り当てる。
		case "ctrl+k":
			m.scroll = clampScroll(m.scroll-1, len(m.previewLines()), previewViewportH(m.bodyOuterH()))
			return m, nil
		case "ctrl+j":
			m.scroll = clampScroll(m.scroll+1, len(m.previewLines()), previewViewportH(m.bodyOuterH()))
			return m, nil
		case "pgup":
			viewH := previewViewportH(m.bodyOuterH())
			m.scroll = clampScroll(m.scroll-viewH, len(m.previewLines()), viewH)
			return m, nil
		case "pgdown":
			viewH := previewViewportH(m.bodyOuterH())
			m.scroll = clampScroll(m.scroll+viewH, len(m.previewLines()), viewH)
			return m, nil

		// 補完の確定。選択中の候補を入力バーへ追記する。
		case "tab":
			if m.selected < len(m.cands) {
				m.query = applyCompletion(m.query, m.replaceAt, m.cands[m.selected])
				m.gen++
				return m, m.scheduleEval()
			}
			return m, nil

		case "backspace":
			if r := []rune(m.query); len(r) > 0 {
				m.query = string(r[:len(r)-1])
				m.gen++
				return m, m.scheduleEval()
			}

		default:
			// 印字可能なキーは入力バーへ追記する。msg.Text には実際に入力された
			// 文字列（スペースを含む）が入り、矢印など非印字キーでは空になる。
			// String() のルーン数で判定するとスペース等を取りこぼすため Text を使う。
			// （暫定。本格的な行編集は後続Issue）
			if msg.Text != "" {
				m.query += msg.Text
				m.gen++
				return m, m.scheduleEval()
			}
		}
	}
	return m, nil
}

var (
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	titleStyle    = lipgloss.NewStyle().Bold(true)
	hintStyle     = lipgloss.NewStyle().Faint(true)
	footerStyle   = lipgloss.NewStyle().Faint(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true)
)

// renderCandidates は補完候補リストを、選択中の候補をハイライトして描画します。
// maxLines に収まらない場合は、選択中の候補が常に見えるよう表示範囲を窓送りします。
func renderCandidates(cands []string, selected, maxLines int) string {
	if len(cands) == 0 {
		return hintStyle.Render("(候補なし)")
	}
	if maxLines < 1 {
		maxLines = 1
	}

	// 選択位置が表示窓に入るよう先頭をずらす。
	start := 0
	if selected >= maxLines {
		start = selected - maxLines + 1
	}
	end := min(start+maxLines, len(cands))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if i == selected {
			lines = append(lines, selectedStyle.Render("▌"+cands[i]))
		} else {
			lines = append(lines, " "+cands[i])
		}
	}
	return strings.Join(lines, "\n")
}

// scrollIndicator はプレビューがスクロール可能なとき、ヘッダ末尾に出す控えめな
// 進捗表示（例: " [12-30/120]"）を返します。全体が収まっているときは空文字です。
func scrollIndicator(scroll, totalLines, viewH int) string {
	if maxScroll(totalLines, viewH) == 0 {
		return ""
	}
	end := min(scroll+viewH, totalLines)
	return fmt.Sprintf(" [%d-%d/%d]", scroll+1, end, totalLines)
}

func (m Model) View() tea.View {
	if m.width < 20 || m.height < 8 {
		// 最初のサイズ通知前、または端末が極端に小さい場合。
		v := tea.NewView("読み込み中…（端末を広げてください）")
		v.AltScreen = true
		return v
	}

	const barOuterH = 3 // 入力バー（枠込み）
	bodyOuterH := m.bodyOuterH()
	leftOuterW := m.width / 3
	rightOuterW := m.width - leftOuterW

	// 入力バー。クエリが内幅を超えたら末尾を表示（横スクロール）し、
	// バーが縦に伸びてレイアウトが崩れるのを防ぐ。
	// 式が無効なときはプロンプト記号を変えて状態を控えめに示す。
	// （本物のカーソル表示は後続Issueで対応する）
	prompt := "> "
	if m.evalErr != nil {
		prompt = "✗ "
	}
	bar := box(m.width, barOuterH, fitInputBar(prompt, m.query, m.width-2))

	// 下段：左（補完候補）／右（プレビュー）
	// タイトル＋空行で2行使うので、候補に充てられる内幅の高さはその分減らす。
	left := box(leftOuterW, bodyOuterH,
		titleStyle.Render("補完候補")+"\n\n"+
			renderCandidates(m.cands, m.selected, bodyOuterH-4),
	)
	// プレビューは viewport として、スクロール位置から表示可能行数ぶんだけ切り出す。
	// クランプは描画時にも行い、リサイズで表示可能行数が変わって窓が末尾を超えた
	// 場合でも空白だけのページにならないようにする（scroll 自体は次のキー入力で
	// 追従して補正される）。
	viewH := previewViewportH(bodyOuterH)
	lines := m.previewLines()
	scroll := clampScroll(m.scroll, len(lines), viewH)
	end := min(scroll+viewH, len(lines))
	visible := strings.Join(lines[scroll:end], "\n")
	// 切り出した可視範囲だけをキー・値・型で色分けして読みやすくする（#12）。
	// NO_COLOR や色非対応端末では colorEnabled が false を返し、プレーンテキストへ
	// フォールバックする。色付けは表示文字にのみ付与し見た目幅を変えないため、
	// 行分割・スクロール後に適用しても枠あふれやスクロール量の計算には影響しない。
	visible = highlightYAML(visible, colorEnabled())

	rightHeader := titleStyle.Render("プレビュー") + " " +
		hintStyle.Render(fmt.Sprintf("%s (%d bytes)%s",
			m.source.Name, len(m.source.Data), scrollIndicator(scroll, len(lines), viewH)))
	right := box(rightOuterW, bodyOuterH, rightHeader+"\n\n"+visible)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	// フッターは幅上限を持たないため、狭い端末でも全体幅を超えないよう末尾を切り詰める。
	footer := footerStyle.Render(ansi.Truncate("Ctrl+C / Esc: 終了   Ctrl+J/K・PgUp/PgDn: プレビュー スクロール", m.width, "…"))

	content := strings.Join([]string{bar, body, footer}, "\n")
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// box は内容を outerW×outerH（枠線込み）のボックスに描画する。MaxWidth/MaxHeight
// で上限も固定し、内容が長くても枠からあふれてレイアウトを崩さないようにする。
func box(outerW, outerH int, content string) string {
	return borderStyle.
		Width(outerW).Height(outerH).
		MaxWidth(outerW).MaxHeight(outerH).
		Render(content)
}

// fitInputBar はプロンプト付きの入力行を innerWidth に収める。収まらない場合は
// 末尾（入力中の箇所）が見えるよう先頭を切り詰め、"…" を前置する。
func fitInputBar(prompt, query string, innerWidth int) string {
	text := prompt + query
	if innerWidth <= 0 || lipgloss.Width(text) <= innerWidth {
		return text
	}
	// 先頭から (超過分 + "…"の幅1) セルを除去して innerWidth に収める。
	cut := lipgloss.Width(text) - innerWidth + 1
	return ansi.TruncateLeft(text, cut, "…")
}
