// Package input は yqtui が操作対象とする YAML を解決し、TUI 用のキーボード
// 入力源を用意するパッケージです。
//
// データはファイル引数（`yqtui file.yaml`）またはパイプ（`cat file.yaml | yqtui`）
// から取得します。パイプ経由の場合、os.Stdin は読み切られて端末でもなくなるため、
// 対話 UI を動かすために /dev/tty を開きます。
package input

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

// ErrNoInput は、ファイル引数もパイプされた stdin も無い場合、つまり
// yqtui を表示するデータなしで対話起動したときに返されます。
var ErrNoInput = errors.New("no input provided")

// Source は解決済みの、操作対象となる YAML 入力です。
type Source struct {
	Data []byte // 生の YAML バイト列
	Name string // 由来を表す表示名。ファイルパスまたは "<stdin>"
}

// Load は YAML 入力を解決します。args にパスが含まれていればそのファイルを読み、
// 無ければ stdin がパイプ・リダイレクト（端末でない）の場合に stdin を読みます。
// どちらも利用できない場合は ErrNoInput を返します。
func Load(args []string, stdin *os.File) (Source, error) {
	if len(args) > 0 {
		path := args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return Source{}, fmt.Errorf("reading %s: %w", path, err)
		}
		return Source{Data: data, Name: path}, nil
	}

	if !isTerminal(stdin) {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return Source{}, fmt.Errorf("reading stdin: %w", err)
		}
		return Source{Data: data, Name: "<stdin>"}, nil
	}

	return Source{}, ErrNoInput
}

// TUIInput は端末 UI を駆動するための reader を返します。stdin が端末なら
// それをそのまま使い、stdin がパイプ（端末でない）でキー入力を運べない場合は
// 代わりに /dev/tty を開きます。
//
// 返り値の io.Closer は新しくファイルを開いたときだけ非 nil になります。
// その場合は呼び出し側で閉じる責任があります。
func TUIInput(stdin *os.File) (io.Reader, io.Closer, error) {
	if isTerminal(stdin) {
		return stdin, nil, nil
	}

	tty, err := os.Open("/dev/tty")
	if err != nil {
		return nil, nil, fmt.Errorf("opening /dev/tty for interactive input: %w", err)
	}
	return tty, tty, nil
}

// isTerminal は f が対話的な端末（tty）かどうかを返します。パイプ・通常ファイル・
// /dev/null のような非 tty のキャラクタデバイスは false になります。
func isTerminal(f *os.File) bool {
	return f != nil && term.IsTerminal(f.Fd())
}
