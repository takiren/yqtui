// Package yq は mikefarah/yq のライブラリをラップし、外部の `yq` バイナリを
// 呼び出すことなく、アプリの他の部分がメモリ上の YAML に対して yq 式を
// 評価できるようにするパッケージです。
package yq

import (
	"github.com/mikefarah/yq/v4/pkg/yqlib"
)

// Evaluate は与えられた YAML 入力に対して yq 式を実行し、結果を YAML として
// エンコードして返します。入力中のすべてのドキュメントを横断して評価し、
// yq CLI のデフォルト挙動に揃えています。
//
// 式のパースエラーや評価エラーは err として返ります。式が単に何にも一致しない
// 場合はエラーにならず、結果（例: "null\n"）が返ります。
func Evaluate(expression string, input []byte) (string, error) {
	encoder := yqlib.NewYamlEncoder(yqlib.ConfiguredYamlPreferences)
	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)
	return yqlib.NewStringEvaluator().EvaluateAll(expression, string(input), encoder, decoder)
}
