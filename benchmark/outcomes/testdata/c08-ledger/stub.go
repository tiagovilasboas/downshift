package task

type Ledger struct{}

func NewLedger(balances map[string]int64) *Ledger              { panic("not implemented") }
func (l *Ledger) Balance(account string) int64                 { panic("not implemented") }
func (l *Ledger) Transfer(from, to string, amount int64) error { panic("not implemented") }
