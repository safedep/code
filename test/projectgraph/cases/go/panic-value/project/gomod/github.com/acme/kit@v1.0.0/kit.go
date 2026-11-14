package kit

type failure struct{}

func Fail() error { return &failure{} }

func (f *failure) Error() string { return report() }

func report() string { return "" }

func (f *failure) Unwrap() error { return unwrap() }

func unwrap() error { return nil }
