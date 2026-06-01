// Package yq wraps mikefarah/yq's library so the rest of the application can
// evaluate yq expressions against in-memory YAML without shelling out to the
// external `yq` binary.
package yq

import (
	"github.com/mikefarah/yq/v4/pkg/yqlib"
)

// Evaluate runs the yq expression against the given YAML input and returns the
// result encoded as YAML. It evaluates across every document in the input,
// mirroring the default behaviour of the yq CLI.
//
// A parse error in the expression or an evaluation error is returned as err;
// an expression that simply matches nothing yields a non-error result (e.g.
// "null\n").
func Evaluate(expression string, input []byte) (string, error) {
	encoder := yqlib.NewYamlEncoder(yqlib.ConfiguredYamlPreferences)
	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)
	return yqlib.NewStringEvaluator().EvaluateAll(expression, string(input), encoder, decoder)
}
