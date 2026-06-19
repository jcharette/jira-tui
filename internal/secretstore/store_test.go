package secretstore

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreSetGetDelete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	if err := store.Set(ctx, "default|https://example.atlassian.net|person@example.com", "secret"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := store.Get(ctx, "default|https://example.atlassian.net|person@example.com")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "secret" {
		t.Fatalf("secret = %q", got)
	}
	if err := store.Delete(ctx, "default|https://example.atlassian.net|person@example.com"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, "default|https://example.atlassian.net|person@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestAccountKeyNormalizesProfileURLAndEmail(t *testing.T) {
	got := AccountKey(" Work ", " HTTPS://Example.Atlassian.Net ", " Person@Example.Com ")

	if got != "work|https://example.atlassian.net|person@example.com" {
		t.Fatalf("AccountKey() = %q", got)
	}
}
