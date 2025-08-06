package scan

type Config struct {
	InputDirectory     string
	OutputDatabasePath string
	SkipInitSchema     bool
}

type scanner struct {
	config Config
}

func New(config Config) (*scanner, error) {
	return &scanner{
		config: config,
	}, nil
}

func (s *scanner) Run() error {
	return nil
}
