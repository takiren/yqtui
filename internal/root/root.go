// Package root はアプリ全体の bubbletea モデルを提供します。
//
// 上部に入力バー、下部を左右に分割（左：補完候補、右：プレビュー）した3領域
// 構成で、端末サイズの変化に追従します。入力バーの式は yq で評価して右ペインに
// プレビューします。評価はキー入力ごとに UI スレッドをブロックしないよう、
// デバウンスしたうえで tea.Cmd として非同期に実行します（#30）。補完は後続の
// Issue で実装します。
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

// previewMsg は非同期評価の結果です。gen はどの入力世代に対する結果かを表し、
// 最新世代でなければ（より新しい入力があれば）破棄されます。
type previewMsg struct {
	gen uint64
	out string
	err error
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

// evalCmd は yq 評価を UI スレッドの外（コマンドのゴルーチン）で実行し、結果を
// previewMsg として返します。
func evalCmd(gen uint64, query string, data []byte) tea.Cmd {
	return func() tea.Msg {
		out, err := yq.Evaluate(exprFor(query), data)
		return previewMsg{gen: gen, out: out, err: err}
	}
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

	case previewMsg:
		// 古い世代の結果は破棄し、最新クエリの結果だけを採用する。失敗時は
		// 直前の有効なプレビューを保持し、エラー状態だけを記録する。
		if msg.gen == m.gen {
			if msg.err != nil {
				m.evalErr = msg.err
			} else {
				m.evalErr = nil
				m.preview = msg.out
			}
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		// 中断
		case "ctrl+c", "esc":
			return m, tea.Quit

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
	left := box(leftOuterW, bodyOuterH,
		titleStyle.Render("補完候補")+"\n\n"+
			hintStyle.Render("(#8 以降で実装)"),
	)
	rightHeader := titleStyle.Render("プレビュー") + " " +
		hintStyle.Render(fmt.Sprintf("%s (%d bytes)", m.source.Name, len(m.source.Data)))
	right := box(rightOuterW, bodyOuterH,
		rightHeader+"\n\n"+strings.TrimRight(m.preview, "\n"),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := footerStyle.Render("Ctrl+C / Esc: 終了")

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
