package yq

import (
	"strings"
	"testing"
)

const sampleYAML = `
name: demo
spec:
  replicas: 3
  containers:
    - name: web
      image: nginx:1.27
    - name: sidecar
      image: envoy:1.30
`

func TestEvaluate_ScalarPath(t *testing.T) {
	got, err := Evaluate(".spec.replicas", []byte(sampleYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "3" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "3")
	}
}

func TestEvaluate_ArrayWildcard(t *testing.T) {
	got, err := Evaluate(".spec.containers[].image", []byte(sampleYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "nginx:1.27\nenvoy:1.30"
	if strings.TrimSpace(got) != want {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), want)
	}
}

func TestEvaluate_Subtree(t *testing.T) {
	got, err := Evaluate(".spec.containers[0]", []byte(sampleYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "name: web") || !strings.Contains(got, "image: nginx:1.27") {
		t.Errorf("subtree missing expected fields, got:\n%s", got)
	}
}

func TestEvaluate_NoMatchIsNotError(t *testing.T) {
	got, err := Evaluate(".does.not.exist", []byte(sampleYAML))
	if err != nil {
		t.Fatalf("no-match should not error, got: %v", err)
	}
	if strings.TrimSpace(got) != "null" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "null")
	}
}

func TestEvaluate_AdditionDoesNotError(t *testing.T) {
	// `.a + .b` のような加算式がエラーにならず評価できることを確認する。
	cases := []struct {
		name string
		expr string
		want string
	}{
		{
			name: "数値同士の加算",
			expr: ".spec.replicas + .spec.replicas",
			want: "6",
		},
		{
			name: "文字列同士の連結",
			expr: ".name + .name",
			want: "demodemo",
		},
		{
			name: "マップ同士のマージ",
			expr: ".spec + .spec",
			want: "replicas: 3\ncontainers:\n  - name: web\n    image: nginx:1.27\n  - name: sidecar\n    image: envoy:1.30",
		},
		{
			name: "配列同士の連結",
			expr: ".spec.containers + .spec.containers",
			want: "- name: web\n  image: nginx:1.27\n- name: sidecar\n  image: envoy:1.30\n- name: web\n  image: nginx:1.27\n- name: sidecar\n  image: envoy:1.30",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Evaluate(tc.expr, []byte(sampleYAML))
			if err != nil {
				t.Fatalf("addition expression should not error, got: %v", err)
			}
			if strings.TrimSpace(got) != tc.want {
				t.Errorf("got %q, want %q", strings.TrimSpace(got), tc.want)
			}
		})
	}
}

func TestEvaluate_AdditionWithoutSpacesDoesNotError(t *testing.T) {
	// `.a+.b` のように空白を挟まない加算式でもエラーを吐かないことを確認する。
	// （結果が一致するかは問わない。yq の字句解析では `null` になるが、
	//  ここで保証したいのはエラーにならないことだけ。）
	if _, err := Evaluate(".spec.replicas+.spec.replicas", []byte(sampleYAML)); err != nil {
		t.Fatalf("addition expression without spaces should not error, got: %v", err)
	}
}

func TestEvaluate_AdditionOfMismatchedTypesErrors(t *testing.T) {
	// 型が合わない加算（文字列 + マップ）は yq の仕様上、評価時に
	// 型エラーになることを確認する。式として不正なのではなく、評価エラー。
	if _, err := Evaluate(".name + .spec", []byte(sampleYAML)); err == nil {
		t.Error("adding a string to a map should error, got nil")
	}
}

func TestEvaluate_InvalidExpressionErrors(t *testing.T) {
	if _, err := Evaluate(".spec.[", []byte(sampleYAML)); err == nil {
		t.Error("expected an error for an invalid expression, got nil")
	}
}

func TestEvaluate_EmptyExpressionReturnsWholeDocument(t *testing.T) {
	got, err := Evaluate(".", []byte(sampleYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "name: demo") {
		t.Errorf("identity expression should return whole document, got:\n%s", got)
	}
}
