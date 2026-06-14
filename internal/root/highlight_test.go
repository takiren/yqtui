package root

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// hasANSI は文字列に ANSI エスケープ（色付け）が含まれるかを判定する。
func hasANSI(s string) bool {
	return strings.Contains(s, "\x1b[")
}

// 色無効（NO_COLOR / 色非対応端末）では入力をそのまま返し、ANSI を一切含まない
// こと。これは Issue #12 の完了条件（フォールバック）。
func TestHighlightYAML_DisabledHasNoANSI(t *testing.T) {
	in := "name: demo\nspec:\n  replicas: 3\n  enabled: true\n  note: null\n"
	out := highlightYAML(in, false)
	if out != in {
		t.Errorf("色無効では入力をそのまま返すべき:\n got=%q\nwant=%q", out, in)
	}
	if hasANSI(out) {
		t.Errorf("色無効の出力に ANSI が含まれている: %q", out)
	}
}

// 空文字列は色の有効・無効によらずそのまま返ること。
func TestHighlightYAML_EmptyInput(t *testing.T) {
	if got := highlightYAML("", true); got != "" {
		t.Errorf("空入力 = %q, want \"\"", got)
	}
}

// 色有効では装飾（ANSI）が付与され、かつ見た目幅（lipgloss.Width）は
// 元のプレーン文字列と一致すること（枠あふれ防止）。
func TestHighlightYAML_EnabledKeepsVisibleWidth(t *testing.T) {
	// テスト環境では端末非接続でカラープロファイルが Ascii になりがちなので、
	// enabled を明示 true にして色付け経路を必ず通す。
	lines := []string{
		"name: demo",
		"  replicas: 3",
		"  enabled: true",
		"  note: null",
		"  - nginx",
		"  - name: web",
		"# コメント",
	}
	for _, line := range lines {
		got := highlightYAML(line, true)
		if w, want := lipgloss.Width(got), lipgloss.Width(line); w != want {
			t.Errorf("行 %q の見た目幅 = %d, want %d (out=%q)", line, w, want, got)
		}
	}
}

// 色有効では実際に色（ANSI）が付くこと。少なくともキー・値を含む行で装飾が
// 入る。Ascii プロファイルでも、明示 enabled=true なら highlightValue/keyStyle
// の Render を通すため、スタイルが no-op でなければ ANSI が出る。
//
// 注: スタイルの no-op 化はレンダラのプロファイル依存なので、ここでは「装飾が
// 付くこと」を強制せず、構造が壊れないこと（キー名・値が残ること）を検証する。
func TestHighlightYAML_PreservesContent(t *testing.T) {
	in := "name: demo\nspec:\n  replicas: 3\n  enabled: true\n  note: null\n  - nginx\n"
	out := highlightYAML(in, true)
	for _, want := range []string{"name", "demo", "replicas", "3", "enabled", "true", "null", "nginx"} {
		if !strings.Contains(out, want) {
			t.Errorf("色付け後に %q が失われた:\n%s", want, out)
		}
	}
	// 行数（改行）は保たれること。
	if strings.Count(out, "\n") != strings.Count(in, "\n") {
		t.Errorf("改行数が変化した: got=%d want=%d", strings.Count(out, "\n"), strings.Count(in, "\n"))
	}
}

// splitKeyValue が "key: value" を正しく分割すること（値内の ": " 等の境界条件）。
func TestSplitKeyValue_Cases(t *testing.T) {
	cases := []struct {
		in       string
		key, val string
		ok       bool
	}{
		{"name: demo", "name", "demo", true},
		{"url: http://x", "url", "http://x", true},
		{"msg: a: b", "msg", "a: b", true}, // 最初の ": " で分割
		{"justvalue", "", "", false},
		{"'quoted: key': v", "'quoted: key'", "v", true},
		{`"q: k": v`, `"q: k"`, "v", true},
	}
	for _, c := range cases {
		k, v, ok := splitKeyValue(c.in)
		if ok != c.ok || k != c.key || v != c.val {
			t.Errorf("splitKeyValue(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, k, v, ok, c.key, c.val, c.ok)
		}
	}
}

// 型推定（数値・真偽・null・文字列）が見た目幅を変えずに値を保つこと。
func TestHighlightValue_PreservesText(t *testing.T) {
	for _, v := range []string{"3", "-1.5", "0xff", "true", "False", "null", "~", "hello", `"x"`, "[1, 2]"} {
		got := highlightValue(v)
		if lipgloss.Width(got) != lipgloss.Width(v) {
			t.Errorf("highlightValue(%q) の見た目幅が変化: %q", v, got)
		}
		if !strings.Contains(got, v) {
			t.Errorf("highlightValue(%q) が値を失った: %q", v, got)
		}
	}
}

// colorEnabled は NO_COLOR がセットされていれば false を返すこと（フォールバック）。
func TestColorEnabled_NoColorDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorEnabled() {
		t.Error("NO_COLOR がセットされていれば色付けは無効であるべき")
	}
	// 空文字でも「存在する」だけで無効（no-color.org の慣習）。
	t.Setenv("NO_COLOR", "")
	if colorEnabled() {
		t.Error("NO_COLOR が空文字でも存在すれば無効であるべき")
	}
}
