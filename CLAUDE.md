# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

yqtui は `mikefarah/yq` をライブラリとして組み込んだ、YAML を閲覧するための TUI（bubbletea v2 製）です。yq のクエリ構文を覚えていなくても、補完とインクリメンタル検索でパス式を組み立てながら結果をライブプレビューできることを目指しています。MVP の設計判断はメモリ（`yqtui-design`）に記録されています。

## コマンド

```sh
go build ./...                                     # ビルド
go run ./cmd <file.yaml>                            # 実行（ファイル指定）
cat file.yaml | go run ./cmd                        # 実行（パイプ）
go test ./...                                       # 全テスト
go test ./internal/yq/                              # 単一パッケージのテスト
go test -run TestChildKeys_NestedPath ./internal/yq/   # 単一テスト
go vet ./...                                        # 静的解析
gofmt -l internal/ cmd/                             # 整形漏れの検出（出力が空なら OK）
```

## アーキテクチャ

データの流れは `input → root（bubbletea Model）→ yq` です。

- **`cmd/main.go`**: エントリポイント。`input.Load` で対象 YAML を解決し、`input.TUIInput` で UI 用のキー入力源を用意してから bubbletea を起動する。
- **`internal/input`**: 操作対象 YAML の解決と、TUI 用キー入力源の用意。パイプ経由で起動すると `os.Stdin` は読み切られて端末でなくなるため、対話 UI を駆動するために `/dev/tty` を開き直す（`TUIInput`）。この二重性がこのパッケージの存在理由。
- **`internal/root`**: アプリ全体の bubbletea `Model`。上部に入力バー、下部を左右分割（左＝補完候補、右＝プレビュー）した3領域レイアウトで、端末サイズ変化に追従する。入力バーに組み立てた式を yq で評価し、右ペインにプレビューする。
- **`internal/yq`**: `yqlib` の薄いラッパー。外部の `yq` バイナリは呼ばず、メモリ上の YAML バイト列に対して動く。`yqlib` への依存はこのパッケージに閉じ込め、他パッケージからは直接 import しない方針。
  - `Evaluate(expression, input)`: 式を評価し、結果を YAML 文字列で返す（プレビュー用）。
  - `ChildKeys(expression, input)`: 式が指すマップノードの直下キーを出現順で列挙する（補完候補の生成用）。

### 設計上の要点

- **空クエリは恒等式 `.`**: 入力が空のときはドキュメント全体を表示する（`exprFor`）。`ChildKeys` の空式も同様にルート直下キーを返す。
- **評価はUIをブロックしない**: yq 評価はキー入力ごとに同期実行せず、`tea.Tick` でデバウンスしてから `tea.Cmd` として非同期に走らせる。入力世代カウンタ（`gen`）で、古い評価結果が新しいプレビューを上書きしないようにする。
- **エラー時はプレビューを保持する**: yq 評価が失敗しても直前の有効なプレビューを残し、入力バーのプロンプト記号（`>` → `✗`）でエラー状態を控えめに示すだけにする。
- **枠あふれを起こさない**: レイアウトの各ボックスは `box()` で `Width/Height` に加え `MaxWidth/MaxHeight` も固定し、内容が長くても枠からあふれてレイアウトが崩れないようにする。入力バーは `fitInputBar()` で末尾（入力中の箇所）が見えるよう先頭を切り詰める。
- **キー入力は `msg.Text` で取る**: `KeyPressMsg.String()` のルーン数判定だとスペース（`"space"` を返す）等を取りこぼし yq 式を入力できないため、実際の入力文字が入る `msg.Text` を使う。

## コーディング規約

- **コメント・ドキュメントは日本語で書く**（パッケージ doc、関数 doc、CLI の usage/ヘルプを含む）。英語で書かない。サードパーティの skill ファイルは対象外。
- Go の error 文字列は慣例に従い英語のままにしている。判断に迷えば確認する。
- テストは標準 `testing` のみ。テスト名は `Test<対象>_<シナリオ>` 形式（例: `TestChildKeys_NestedPath`、`TestPreview_StaleResultIsDropped`）。
- bubbletea のテストは、`Update` が返す `tea.Cmd` を手で実行してメッセージを `Update` に戻す形で非同期パイプラインを駆動する（`internal/root` の `pump` ヘルパ参照）。

## ワークフロー

- Issue ごとに作業ブランチを切り、Issue 単位で個別の PR を出す（`feat/<番号>-<要約>`）。
- コミットメッセージ・PR は日本語。完了条件のチェックリストを PR 本文に含める。
