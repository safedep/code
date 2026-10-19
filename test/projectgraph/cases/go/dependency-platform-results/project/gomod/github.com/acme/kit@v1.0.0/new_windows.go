package kit

type Windows struct{}

func (*Windows) Common() {}

func (*Windows) WindowsOnly() {}

func New() *Windows { return &Windows{} }
