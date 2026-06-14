package root

import (
	"os"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

// このファイルはプレビューの YAML 文字列に対する軽量なシンタックスハイライトを
// 提供します（#12）。外部の重いハイライトライブラリは追加せず、行ベースの
// 簡易トークナイザで yq の出力（標準的な2スペースインデントのブロックスタイル
// YAML）をキー・値・型ごとに色分けします。
//
// 方針:
//   - View 描画段階で m.preview に適用する純粋関数 highlightYAML として実装し、
//     テストしやすくする。色の有効/無効は引数で明示的に渡す。
//   - NO_COLOR 環境変数がセットされている場合や、色非対応端末では色付けを無効化
//     してプレーンテキストにフォールバックする（colorEnabled で判定）。
//   - 行頭インデント・"- " リスト記号・コメントなどの構造はそのまま残し、
//     キー（": " の左側）と値（右側）を別スタイルで描画する。値は型を推定して
//     数値・真偽・null・文字列で色を変える。

var (
	// keyStyle はマップのキー名（"key:" の左側）。
	keyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // 青
	// numberStyle は数値の値。
	numberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // マゼンタ
	// boolStyle は真偽値（true/false）。
	boolStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // 黄
	// nullStyle は null（~ を含む）。
	nullStyle = lipgloss.NewStyle().Faint(true)
	// stringStyle はそれ以外の文字列値。
	stringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // 緑
	// commentStyle は "#" から始まるコメント。
	commentStyle = lipgloss.NewStyle().Faint(true)
)

// 数値・真偽・null の判定用パターン。YAML スカラの一般的な表記に合わせている。
var (
	numberRe = regexp.MustCompile(`^[-+]?(?:0x[0-9a-fA-F]+|0o[0-7]+|\d+(?:\.\d+)?(?:[eE][-+]?\d+)?|\.\d+(?:[eE][-+]?\d+)?)$`)
	boolRe   = regexp.MustCompile(`^(?:true|false|True|False|TRUE|FALSE)$`)
	nullRe   = regexp.MustCompile(`^(?:null|Null|NULL|~)$`)
)

// colorEnabled は現在の環境で色付けを行うべきかを判定します。外部依存を増やさず
// stdlib だけで判定するため、次の順で確認します。
//
//   - NO_COLOR がセットされていれば（値に関わらず）無効。no-color.org の慣習に
//     従い、変数が「存在する」だけで無効化する（完了条件のフォールバック）。
//   - 出力（os.Stdout）が端末でなければ（パイプ・リダイレクト・テスト実行など）
//     無効。文字デバイス（ModeCharDevice）かどうかで素朴に判定する。
//
// 注: bubbletea/lipgloss は実描画時に端末のカラープロファイルへ合わせて ANSI を
// 出し分けるため、ここでの判定は「そもそも色を付けるか」の安全側ゲートとして働く。
func colorEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return isTerminal(os.Stdout)
}

// isTerminal は f が端末（文字デバイス）かどうかを判定します。x/term などの
// 追加依存を避けるため、FileMode の ModeCharDevice を見る素朴な実装です。
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// highlightYAML は YAML 文字列をキー・値・型で色分けして返します。enabled が
// false のときは入力をそのまま返し（ANSI エスケープを一切含めない）、色非対応
// 端末・NO_COLOR でのフォールバックとします。
//
// レイアウトの幅計算が ANSI でズレないよう、装飾は表示文字にのみ付与し、
// インデントや記号といった構造文字列は変更しません（見た目幅は不変）。
func highlightYAML(s string, enabled bool) string {
	if !enabled || s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = highlightLine(line)
	}
	return strings.Join(lines, "\n")
}

// highlightLine は1行を色分けします。行を「先頭の空白＋任意のリスト記号 "- "」と
// 「残り」に分け、残りを "key: value" として解釈します。コメント行・空行・
// document marker（--- / ...）はそのまま、もしくは控えめに色付けします。
func highlightLine(line string) string {
	// 先頭インデントを温存する。
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	rest := line[len(indent):]
	if rest == "" {
		return line
	}

	// コメント行。
	if strings.HasPrefix(rest, "#") {
		return indent + commentStyle.Render(rest)
	}
	// ドキュメント区切り（--- / ...）はそのまま。
	if rest == "---" || rest == "..." {
		return line
	}

	// リスト要素の "- " を温存しつつ、その後ろを再帰的に解釈する。
	// 値だけの要素（"- nginx"）やネストしたマップ（"- name: web"）に対応する。
	if rest == "-" {
		return line
	}
	if strings.HasPrefix(rest, "- ") {
		return indent + "- " + highlightInline(rest[len("- "):])
	}
	return indent + highlightInline(rest)
}

// highlightInline は "key: value"、もしくは値のみのスカラを色分けします。
// クォートやインデントを含まない、行の本体部分を受け取ります。
func highlightInline(s string) string {
	if k, v, ok := splitKeyValue(s); ok {
		if v == "" {
			// 値が空（ネストの親など）。キーのみ色付け。
			return keyStyle.Render(k) + ":"
		}
		return keyStyle.Render(k) + ": " + highlightValue(v)
	}
	// "key:" だけで値が続かない行。
	if strings.HasSuffix(s, ":") {
		return keyStyle.Render(strings.TrimSuffix(s, ":")) + ":"
	}
	// キーを持たない値（リストのスカラ要素など）。
	return highlightValue(s)
}

// splitKeyValue は "key: value" を key と value に分けます。値側に出てくる
// ": " と区別するため、最初の ": "（または末尾の ":"）で1回だけ分割します。
// キーがクォートされている場合も素朴に対応します。
func splitKeyValue(s string) (key, value string, ok bool) {
	// クォート済みキー（'a: b': v / "a: b": v）は閉じクォートの後の ":" を探す。
	if s != "" && (s[0] == '\'' || s[0] == '"') {
		q := s[0]
		if end := strings.IndexByte(s[1:], q); end >= 0 {
			rest := s[1+end+1:]
			if strings.HasPrefix(rest, ": ") {
				return s[:1+end+1], rest[len(": "):], true
			}
			if rest == ":" {
				return s[:1+end+1], "", true
			}
		}
		return "", "", false
	}
	if idx := strings.Index(s, ": "); idx >= 0 {
		return s[:idx], s[idx+len(": "):], true
	}
	return "", "", false
}

// highlightValue は値スカラを型推定して色分けします。クォート文字列・数値・
// 真偽・null・その他文字列で色を変えます。フロー（[...] / {...}）は素朴に
// 文字列スタイルで色付けします。
func highlightValue(v string) string {
	switch {
	case v == "":
		return v
	case numberRe.MatchString(v):
		return numberStyle.Render(v)
	case boolRe.MatchString(v):
		return boolStyle.Render(v)
	case nullRe.MatchString(v):
		return nullStyle.Render(v)
	default:
		// クォート文字列・素の文字列・フローコレクションなどはまとめて文字列扱い。
		return stringStyle.Render(v)
	}
}
