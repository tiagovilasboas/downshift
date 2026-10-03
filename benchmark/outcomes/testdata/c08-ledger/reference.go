package task

import (
	"errors"
	"sync"
)

type account struct {
	mu  sync.Mutex
	bal int64
}

type Ledger struct{ accts map[string]*account }

func NewLedger(balances map[string]int64) *Ledger {
	l := &Ledger{accts: map[string]*account{}}
	for k, v := range balances {
		l.accts[k] = &account{bal: v}
	}
	return l
}

func (l *Ledger) Balance(name string) int64 {
	a, ok := l.accts[name]
	if !ok {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.bal
}

func (l *Ledger) Transfer(from, to string, amount int64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	if from == to {
		return errors.New("self transfer")
	}
	a, okA := l.accts[from]
	b, okB := l.accts[to]
	if !okA || !okB {
		return errors.New("unknown account")
	}
	first, second := a, b
	if to < from {
		first, second = b, a
	}
	first.mu.Lock()
	defer first.mu.Unlock()
	second.mu.Lock()
	defer second.mu.Unlock()
	if a.bal < amount {
		return errors.New("insufficient funds")
	}
	a.bal -= amount
	b.bal += amount
	return nil
}
