package store

type Store struct {
	name string
	enc  *Encoder
}

func New(name string) *Store {
	return &Store{name: name, enc: &Encoder{}}
}

func (s *Store) Put(key string) {
	s.enc.Encode(key)
}

func (s *Store) Drop(key string) {
	s.enc.Remove(key)
}
