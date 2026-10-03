package task

type Ring struct{}

func NewRing(replicas int) *Ring      { panic("not implemented") }
func (r *Ring) Add(node string)       { panic("not implemented") }
func (r *Ring) Remove(node string)    { panic("not implemented") }
func (r *Ring) Get(key string) string { panic("not implemented") }
