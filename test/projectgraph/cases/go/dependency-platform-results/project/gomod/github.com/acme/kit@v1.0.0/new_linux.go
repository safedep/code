package kit

type Linux struct{}

func (*Linux) Common() {}

func New() *Linux { return &Linux{} }
