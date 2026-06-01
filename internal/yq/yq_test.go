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
