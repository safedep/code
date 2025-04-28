package main

import (
	_ "embed"

	"github.com/safedep/code/plugin/callgraph"
	"github.com/safedep/dry/log"
	"gopkg.in/yaml.v3"
)

//go:embed signatures.yaml
var signatureYAML []byte

type signatureFile struct {
	Version    string                `yaml:"version"`
	Signatures []callgraph.Signature `yaml:"signatures"`
}

var parsedSignatures signatureFile

func init() {
	err := yaml.Unmarshal(signatureYAML, &parsedSignatures)
	if err != nil {
		log.Fatalf("Failed to parse signature YAML: %v", err)
	}
}
