package signatures

import (
	_ "embed"
	"fmt"
	"os"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"buf.build/go/protovalidate"
	"github.com/safedep/dry/log"
	"gopkg.in/yaml.v3"
)

//go:embed signatures.yaml
var signatureYAML []byte

type signatureFile struct {
	Version    string                  `yaml:"version"`
	Signatures []callgraphv1.Signature `yaml:"signatures"`
}

var ParsedSignatures []*callgraphv1.Signature

func init() {
	var parsedSignatureFile signatureFile
	err := yaml.Unmarshal(signatureYAML, &parsedSignatureFile)
	if err != nil {
		log.Fatalf("Failed to parse signature YAML: %v", err)
		os.Exit(-1)
	}

	ParsedSignatures = make([]*callgraphv1.Signature, len(parsedSignatureFile.Signatures))
	for i := range parsedSignatureFile.Signatures {
		ParsedSignatures[i] = &parsedSignatureFile.Signatures[i]
	}

	err = validateSignatures(ParsedSignatures)
	if err != nil {
		fmt.Printf("Signature validation failed: %v\n", err)
		os.Exit(-1)
	}
}

// post-init validaton as per protbuf spec
func validateSignatures(signatures []*callgraphv1.Signature) error {
	// Validate the unmarshalled protobuf messages
	v, err := protovalidate.New()
	if err != nil {
		return err
	}

	// Validate each signature in the file
	for i := range signatures {
		signature := &signatures[i]
		if err := v.Validate(*signature); err != nil {
			return err
		}
	}

	log.Infof("Successfully validated %d signatures", len(signatures))

	return nil
}
