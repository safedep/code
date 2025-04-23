package callgraph

import (
	_ "embed"
	"log"
	"strings"

	"github.com/safedep/code/core"
	"gopkg.in/yaml.v3"
)

//go:embed signatures.yaml
var signatureYAML []byte

type SignatureFile struct {
	Version    string      `yaml:"version"`
	Signatures []Signature `yaml:"signatures"`
}

type Signature struct {
	ID          string                                 `yaml:"id"`
	Description string                                 `yaml:"description"`
	Tags        []string                               `yaml:"tags"`
	Languages   map[core.LanguageCode]LanguageMatchers `yaml:"languages"`
}

const (
	MatchAny = "any"
	MatchAll = "all"
)

type LanguageMatchers struct {
	Match      string      `yaml:"match"`
	Conditions []Condition `yaml:"conditions"`
}

type Condition struct {
	Type  string `yaml:"type"`  // "call" or "import_module"
	Value string `yaml:"value"` // function or module name
}

var (
	ParsedSignatures   SignatureFile
	signatureByID      map[string]*Signature
	signaturesByPrefix map[string][]*Signature
)

func init() {
	err := yaml.Unmarshal(signatureYAML, &ParsedSignatures)
	if err != nil {
		log.Fatalf("Failed to parse signature YAML: %v", err)
	}

	// Initialize lookup maps
	signatureByID = make(map[string]*Signature)
	signaturesByPrefix = make(map[string][]*Signature)

	for i := range ParsedSignatures.Signatures {
		sig := &ParsedSignatures.Signatures[i]
		signatureByID[sig.ID] = sig

		// build hierarchical prefix map (e.g., "gcp", "gcp.storage", etc.)
		parts := strings.Split(sig.ID, ".")
		for i := 1; i <= len(parts); i++ {
			prefix := strings.Join(parts[:i], ".")
			signaturesByPrefix[prefix] = append(signaturesByPrefix[prefix], sig)
		}
	}

}

func GetSignatureByID(id string) (*Signature, bool) {
	sig, ok := signatureByID[id]
	return sig, ok
}

func GetSignaturesByPrefix(prefix string) []*Signature {
	return signaturesByPrefix[prefix]
}
