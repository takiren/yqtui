package yq

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
)

// ChildKeys は expression が指すノードの直下にあるマップのキー名を、ドキュメント
// 上の出現順で列挙します。補完候補（直下キーの提示）の生成に使います。
//
// 空の expression は恒等式 "." とみなし、ドキュメント直下のキーを返します。
// 解決先がマップでない（スカラーや配列など）場合は、キー無しとして空スライスを
// 返します（エラーにはしません）。式のパース／評価エラーは err として返ります。
//
// 同名キーが複数ノードから得られた場合は最初の1つだけを残して重複を除きます。
func ChildKeys(expression string, input []byte) ([]string, error) {
	if strings.TrimSpace(expression) == "" {
		expression = "."
	}

	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)
	documents, err := yqlib.ReadDocuments(bufio.NewReader(bytes.NewReader(input)), decoder)
	if err != nil {
		return nil, err
	}

	results, err := yqlib.NewAllAtOnceEvaluator().EvaluateCandidateNodes(expression, documents)
	if err != nil {
		return nil, err
	}

	var keys []string
	seen := make(map[string]struct{})
	for el := results.Front(); el != nil; el = el.Next() {
		node, ok := el.Value.(*yqlib.CandidateNode)
		if !ok || node.Kind != yqlib.MappingNode {
			continue
		}
		// MappingNode の Content は key,value,key,value... の交互配置。
		// 偶数番目がキーノードなので、その Value を取り出す。
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys, nil
}
