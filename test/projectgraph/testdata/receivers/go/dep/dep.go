package dep

// Thing is a type of the dependency with methods.
type Thing struct {
	Child *Thing
}

// NewThing returns a Thing.
func NewThing() *Thing { return &Thing{} }

// NewPair returns a Thing and an error.
func NewPair() (*Thing, error) { return &Thing{}, nil }

// Do is the method that the receiver forms call.
func (t *Thing) Do() {}

// Next returns the child of the Thing.
func (t *Thing) Next() *Thing { return t.Child }

// Doer is an interface that Thing implements.
type Doer interface {
	Do()
}

// NewDoer returns a Doer that holds a Thing.
func NewDoer() Doer { return &Thing{} }

// Runner is an interface that no type of the graph implements.
type Runner interface {
	Run()
}

// NewRunner returns a Runner of another module.
func NewRunner() Runner { return nil }
