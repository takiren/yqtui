package yq

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
)

// ChildKeys は expression が指すノードの直下にある補完候補を、ドキュメント上の
// 出現順で列挙します。補完候補（直下キーの提示）の生成に使います。
//
// 解決先がマップの場合は直下のキー名（例: "name"、"spec"）を返します。配列の
// 場合はインデックス候補 "[0]"、"[1]"… と、全要素を辿る "[]" ワイルドカードを
// 返します。返り値のうち "[" で始まるものは配列用の断片で、そのままクエリへ
// 連結できます。それ以外はマップのキー名なので、先頭に "." を付けて連結します
// （連結は補完UI側の責務）。
//
// 空の expression は恒等式 "." とみなし、ドキュメント直下のキーを返します。
// 解決先がマップでも配列でもない（スカラーなど）場合は、候補無しとして空スライス
// を返します（エラーにはしません）。式のパース／評価エラーは err として返ります。
//
// 同じ候補が複数ノードから得られた場合は最初の1つだけを残して重複を除きます。
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
	add := func(candidate string) {
		if _, dup := seen[candidate]; dup {
			return
		}
		seen[candidate] = struct{}{}
		keys = append(keys, candidate)
	}
	for el := results.Front(); el != nil; el = el.Next() {
		node, ok := el.Value.(*yqlib.CandidateNode)
		if !ok {
			continue
		}
		switch node.Kind {
		case yqlib.MappingNode:
			// MappingNode の Content は key,value,key,value... の交互配置。
			// 偶数番目がキーノードなので、その Value を取り出す。
			for i := 0; i+1 < len(node.Content); i += 2 {
				add(node.Content[i].Value)
			}
		case yqlib.SequenceNode:
			// 配列はインデックス候補 "[0]"… と、全要素を辿る "[]" を出す。
			for i := range node.Content {
				add("[" + strconv.Itoa(i) + "]")
			}
			add("[]")
		}
	}
	return keys, nil
}
