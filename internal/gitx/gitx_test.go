package gitx_test

import (
	"context"
	"testing"

	"github.com/kyyril/perfguard/internal/gitx"
)

func TestResolveEmptyRef(t *testing.T) {
	client := gitx.New(".")
	_, err := client.Resolve(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref, got nil")
	}
}

func TestResolveWhitespaceRef(t *testing.T) {
	client := gitx.New(".")
	_, err := client.Resolve(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for whitespace ref, got nil")
	}
}
