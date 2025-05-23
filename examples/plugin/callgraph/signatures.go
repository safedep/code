package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"github.com/safedep/code/plugin/callgraph"
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
	}

	ParsedSignatures = make([]*callgraphv1.Signature, len(parsedSignatureFile.Signatures))
	for i := range parsedSignatureFile.Signatures {
		ParsedSignatures[i] = &parsedSignatureFile.Signatures[i]
	}

	err = callgraph.ValidateSignatures(ParsedSignatures)
	if err != nil {
		fmt.Printf("Signature validation failed: %v\n", err)
		os.Exit(1)
	}
}
