package transactions

import (
	"errors"
	"sync"
	"time"
)

type Type string

const (
	TypeReceive Type = "receive"
	TypePay     Type = "pay"
)

var (
	ErrNotFound = errors.New("transação não encontrada")
	ErrExists   = errors.New("reference já existe")
)

// ValidationError vira HTTP 400.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

type Transaction struct {
	Reference    string     `json:"reference"`
	Type         Type       `json:"type"`
	Status       string     `json:"status"`
	StatusDetail string     `json:"status_detail,omitempty"`
	AmountCents  int64      `json:"amount_cents"`
	Description  string     `json:"description,omitempty"`
	ProviderID   string     `json:"provider_id,omitempty"`
	PixCode      string     `json:"pix_copia_cola,omitempty"`
	QRCodeBase64 string     `json:"qr_code_base64,omitempty"`
	TicketURL    string     `json:"ticket_url,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	KeyType      string     `json:"key_type,omitempty"`
	PixKey       string     `json:"-"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Store interface {
	Create(tx *Transaction) error
	Get(reference string) (*Transaction, error)
	Update(reference string, fn func(*Transaction)) (*Transaction, error)
}

// MemoryStore: provisório. Reiniciou o servidor, perdeu tudo. Troque por MySQL antes de produção.
type MemoryStore struct {
	mu sync.Mutex
	m  map[string]*Transaction
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]*Transaction{}} }

func (s *MemoryStore) Create(tx *Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[tx.Reference]; ok {
		return ErrExists
	}
	now := time.Now()
	tx.CreatedAt, tx.UpdatedAt = now, now
	cp := *tx
	s.m[tx.Reference] = &cp
	return nil
}

func (s *MemoryStore) Get(reference string) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[reference]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (s *MemoryStore) Update(reference string, fn func(*Transaction)) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[reference]
	if !ok {
		return nil, ErrNotFound
	}
	fn(t)
	t.UpdatedAt = time.Now()
	cp := *t
	return &cp, nil
}