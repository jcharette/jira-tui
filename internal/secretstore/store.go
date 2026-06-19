package secretstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/zalando/go-keyring"
)

const ServiceName = "jira-tui"

var ErrNotFound = errors.New("secret not found")

type Store interface {
	Get(ctx context.Context, account string) (string, error)
	Set(ctx context.Context, account string, secret string) error
	Delete(ctx context.Context, account string) error
}

type KeyringStore struct {
	service string
}

func NewKeyringStore() KeyringStore {
	return KeyringStore{service: ServiceName}
}

func (s KeyringStore) Get(ctx context.Context, account string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	account = strings.TrimSpace(account)
	if account == "" {
		return "", errors.New("secret account is required")
	}
	secret, err := keyring.Get(s.serviceName(), account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read OS keychain secret: %w", err)
	}
	return secret, nil
}

func (s KeyringStore) Set(ctx context.Context, account string, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	account = strings.TrimSpace(account)
	if account == "" {
		return errors.New("secret account is required")
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("secret value is required")
	}
	if err := keyring.Set(s.serviceName(), account, secret); err != nil {
		return fmt.Errorf("write OS keychain secret: %w", err)
	}
	return nil
}

func (s KeyringStore) Delete(ctx context.Context, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	account = strings.TrimSpace(account)
	if account == "" {
		return errors.New("secret account is required")
	}
	if err := keyring.Delete(s.serviceName(), account); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete OS keychain secret: %w", err)
	}
	return nil
}

func (s KeyringStore) serviceName() string {
	if strings.TrimSpace(s.service) == "" {
		return ServiceName
	}
	return strings.TrimSpace(s.service)
}

type MemoryStore struct {
	mu      sync.Mutex
	secrets map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{secrets: make(map[string]string)}
}

func (s *MemoryStore) Get(ctx context.Context, account string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	secret, ok := s.secrets[strings.TrimSpace(account)]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (s *MemoryStore) Set(ctx context.Context, account string, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	account = strings.TrimSpace(account)
	if account == "" {
		return errors.New("secret account is required")
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("secret value is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[account] = secret
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	account = strings.TrimSpace(account)
	if _, ok := s.secrets[account]; !ok {
		return ErrNotFound
	}
	delete(s.secrets, account)
	return nil
}

func AccountKey(profileName string, baseURL string, email string) string {
	return strings.Join([]string{
		normalizeSegment(profileName),
		normalizeSegment(baseURL),
		normalizeSegment(email),
	}, "|")
}

func normalizeSegment(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
