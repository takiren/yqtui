package yq

import (
	"reflect"
	"testing"
)

const keysYAML = `
name: demo
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: web
        tier: frontend
  containers:
    - name: web
`

func TestChildKeys_RootKeys(t *testing.T) {
	got, err := ChildKeys(".", []byte(keysYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"name", "spec"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 空の式は恒等式 "." と同じく直下キーを返すこと。
func TestChildKeys_EmptyExpressionIsRoot(t *testing.T) {
	got, err := ChildKeys("", []byte(keysYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"name", "spec"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ネストしたパスの直下キーを辿れること。
func TestChildKeys_NestedPath(t *testing.T) {
	got, err := ChildKeys(".spec", []byte(keysYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"replicas", "template", "containers"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 深いネストでも解決でき、キーの出現順が保たれること。
func TestChildKeys_DeepNestedPathPreservesOrder(t *testing.T) {
	got, err := ChildKeys(".spec.template.metadata.labels", []byte(keysYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"app", "tier"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// マップでないノード（スカラー）はキー無しとして空を返すこと。
func TestChildKeys_ScalarHasNoKeys(t *testing.T) {
	got, err := ChildKeys(".spec.replicas", []byte(keysYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("スカラーはキーを持たないはず, got %v", got)
	}
}

// 配列ノードも（インデックス対応は #9 のため）ここではキー無しとすること。
func TestChildKeys_SequenceHasNoKeys(t *testing.T) {
	got, err := ChildKeys(".spec.containers", []byte(keysYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("配列はここではキーを持たないはず, got %v", got)
	}
}

// 該当ノードが無い場合はエラーにせず空を返すこと。
func TestChildKeys_NoMatchReturnsEmpty(t *testing.T) {
	got, err := ChildKeys(".does.not.exist", []byte(keysYAML))
	if err != nil {
		t.Fatalf("no-match should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("該当なしは空のはず, got %v", got)
	}
}

// 無効な式はエラーになること。
func TestChildKeys_InvalidExpressionErrors(t *testing.T) {
	if _, err := ChildKeys(".spec.[", []byte(keysYAML)); err == nil {
		t.Error("無効な式はエラーを返すべき")
	}
}
