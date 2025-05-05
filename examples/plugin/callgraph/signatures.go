package main

import (
	_ "embed"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"github.com/safedep/dry/log"
	"gopkg.in/yaml.v3"
)

//go:embed signatures.yaml
var signatureYAML []byte

type signatureFile struct {
	Version    string                  `yaml:"version"`
	Signatures []callgraphv1.Signature `yaml:"signatures"`
}

var parsedSignatures signatureFile

func init() {
	err := yaml.Unmarshal(signatureYAML, &parsedSignatures)
	if err != nil {
		log.Fatalf("Failed to parse signature YAML: %v", err)
	}
}
