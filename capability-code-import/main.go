package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/safedep/code/plugin/depsusage"
	"github.com/safedep/dry/log"
)

// Capability represents each capability entry in the JSON
type Capability struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Type               string   `json:"type"`
	PythonDependencies []string `json:"pythonDependencies"`
}

// Capabilities represents the structure of the JSON
type Capabilities struct {
	Capabilities []Capability `json:"capabilities"`
}

var capabilityMap map[string][]Capability

const capabilitiesJsonFile = "capabilities.json"

var (
	dirToWalk string
	languages arrayFlags
)

type arrayFlags []string

func (a *arrayFlags) String() string {
	return strings.Join(*a, ", ")
}
func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func init() {
	log.InitZapLogger("walker", "dev")

	flag.StringVar(&dirToWalk, "dir", "", "Directory to walk")
	flag.Var(&languages, "lang", "Languages to use for parsing files")

	flag.Parse()

	// Read the JSON file
	data, err := os.ReadFile(capabilitiesJsonFile)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Unmarshal JSON into Capabilities struct
	var caps Capabilities
	err = json.Unmarshal(data, &caps)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	// Initialize and build map[string]Capability where key is dependency name
	capabilityMap = make(map[string][]Capability)
	for _, cap := range caps.Capabilities {
		for _, dep := range cap.PythonDependencies {
			capabilityMap[dep] = append(capabilityMap[dep], cap)
		}
	}
}

func main() {
	if dirToWalk == "" {
		flag.Usage()
		return
	}

	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{dirToWalk},
	})

	if err != nil {
		return fmt.Errorf("failed to create local filesystem: %w", err)
	}

	var filteredLanguages []core.Language
	if len(languages) == 0 {
		filteredLanguages, err = lang.AllLanguages()
		if err != nil {
			return fmt.Errorf("failed to get all languages: %w", err)
		}
	} else {
		for _, language := range languages {
			lang, err := lang.GetLanguage(language)
			if err != nil {
				return fmt.Errorf("failed to get language: %w", err)
			}
			filteredLanguages = append(filteredLanguages, lang)
		}
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, filteredLanguages)
	if err != nil {
		return fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, filteredLanguages)
	if err != nil {
		return fmt.Errorf("failed to create tree walker: %w", err)
	}

	taken := make(map[string]bool)

	// consume usage evidences
	var usageCallback depsusage.DependencyUsageCallback = func(ctx context.Context, evidence *depsusage.UsageEvidence) error {
		capability, ok := capabilityMap[evidence.PackageHint]
		if ok {
			if !taken[evidence.PackageHint] {
				capabilityIds := make([]string, 0, len(capability))
				for _, cap := range capability {
					capabilityIds = append(capabilityIds, cap.ID)
				}

				fmt.Println(evidence.PackageHint, "->", capabilityIds)
				taken[evidence.PackageHint] = true
			}
		} else {
			if len(evidence.PackageHint) > 0 {
				fmt.Println(evidence.PackageHint, "-> Unknown")
			}
		}
		return nil
	}

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		depsusage.NewDependencyUsagePlugin(usageCallback),
	})

	if err != nil {
		return fmt.Errorf("failed to create plugin executor: %w", err)
	}

	err = pluginExecutor.Execute(context.Background(), fileSystem)
	if err != nil {
		return fmt.Errorf("failed to execute plugin: %w", err)
	}

	return nil
}

func infer(summaryFile, findingsFile string) {
	b, err := os.ReadFile(summaryFile) // just pass the file name
	if err != nil {
		fmt.Print(err)
	}
	str := string(b)
	prompt := generatePrompt(&str)
	// fmt.Println("Prompt:", prompt)

	azureOpenAIKey := os.Getenv("OPENAI_API_KEY")
	azureOpenaiBaseUrl := os.Getenv("OPENAI_BASE_URL")
	modelDeploymentID := "gpt-4o"

	if azureOpenAIKey == "" || azureOpenaiBaseUrl == "" {
		fmt.Println("Error: OPENAI_API_KEY and OPENAI_BASE_URL environment variables must be set.")
		return
	}

	// use github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai package to interact with OpenAI API

	// create a new client
	keyCredential := azcore.NewKeyCredential(azureOpenAIKey)
	client, err := azopenai.NewClientWithKeyCredential(azureOpenaiBaseUrl, keyCredential, nil)
	if err != nil {
		fmt.Println("Error creating client:", err)
		return
	}

	resp, err := client.GetChatCompletions(context.Background(), azopenai.ChatCompletionsOptions{
		Messages: []azopenai.ChatRequestMessageClassification{
			&azopenai.ChatRequestSystemMessage{Content: azopenai.NewChatRequestSystemMessageContent("You are a code analysis expert assistant")},

			&azopenai.ChatRequestUserMessage{Content: azopenai.NewChatRequestUserMessageContent(prompt)},
		},
		DeploymentName: &modelDeploymentID,
	}, nil)

	if err != nil {
		// TODO: Update the following line with your application specific error handling logic
		fmt.Printf("ERROR: %s", err)
		return
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil || resp.Choices[0].Message.Content == nil {
		fmt.Println("No response, blank results.")
		return
	}

	response := *resp.Choices[0].Message.Content
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimSuffix(response, "```")

	err = os.WriteFile(findingsFile, []byte(response), 0644)
	if err != nil {
		fmt.Println("Error writing to", findingsFile, err)
		return
	}
}

var knownCapabilities = ""

func generatePrompt(code_content *string) string {

	promptFormatStr := `Analyze the following code usage and identify program dependencies and capabilities. Provide the results in JSON format, including capability IDs and corresponding code snippets) For every capability, limit number of code snippets to maximum 5. Identify only capabilities that strongly match the code usage snippets


Some key usage snippets to consider
{%s}

Some example capabilities:
{%s}

you may also add more capabilities if strongly required which follow the same format.

Provide the results **strictly in JSON format**, without any additional explanation or text and only with capabilities that are strongly identified in the code. Ensure that the JSON format is valid and characters like "" are properly escaped.

Return the output **only** in the following JSON format:
{{
  'capabilities': [
    {{
      'capability_id': "network:http",
      'evidence': [
        {{
          "snippet": "response = requests.get(\"https://api.example.com/data\")"
        }}
      ]
    }},
    ...
  ]
}}`
	return fmt.Sprintf(promptFormatStr, *code_content, knownCapabilities)
}

func init() {
	// read capabilities.txt into knownCapabilities
	b, err := os.ReadFile("capabilities.txt")
	if err != nil {
		panic(err)
	}
	knownCapabilities = string(b)
}

func ensureDirExists(dirPath string) error {
	_, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dirPath, err)
		}
	}
	return nil
}
