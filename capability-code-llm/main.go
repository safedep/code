package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
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

	packageWiseUniqueIdentifierWiseUsage := make(map[string]map[string][]depsusage.UsageEvidence)

	// consume usage evidences
	var usageCallback depsusage.DependencyUsageCallback = func(ctx context.Context, evidence *depsusage.UsageEvidence) error {
		// fmt.Println(evidence)
		if packageWiseUniqueIdentifierWiseUsage[evidence.PackageHint] == nil {
			packageWiseUniqueIdentifierWiseUsage[evidence.PackageHint] = make(map[string][]depsusage.UsageEvidence)
		}
		packageWiseUniqueIdentifierWiseUsage[evidence.PackageHint][evidence.Identifier] = append(packageWiseUniqueIdentifierWiseUsage[evidence.PackageHint][evidence.Identifier], *evidence)
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

	outputDir := "outputs/" + strings.TrimSuffix(dirToWalk, "/") + "-results"
	summaryDir := path.Join(outputDir, "summary")
	findingsDir := path.Join(outputDir, "findings")
	err = ensureDirExists(outputDir)
	if err != nil {
		return err
	}
	err = ensureDirExists(summaryDir)
	if err != nil {
		return err
	}
	err = ensureDirExists(findingsDir)
	if err != nil {
		return err
	}

	collector, err := NewOutputCollector(summaryDir, "summary", "yaml", 20000)
	if err != nil {
		fmt.Println("Error creating OutputCollector:", err)
		return err
	}

	for packageHint, uniqueIdentifierWiseUsage := range packageWiseUniqueIdentifierWiseUsage {
		// append package name
		packageHeader := "\n" + packageHint + ":\n"
		err = collector.WriteString("", packageHeader)
		if err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}

		for usedIdentifier, usages := range uniqueIdentifierWiseUsage {
			importNamespace := usages[0].ModuleName
			originalIdentifier := usages[0].ModuleName
			if usages[0].ModuleItem != "" {
				originalIdentifier = usages[0].ModuleItem
				importNamespace += "." + usages[0].ModuleItem
			}
			if usedIdentifier != originalIdentifier {
				importNamespace += " aliased as " + usages[0].ModuleAlias
			}

			// append unique identifier
			identifierHeader := "  " + importNamespace + ":\n"
			err = collector.WriteString(packageHeader, identifierHeader)
			if err != nil {
				return fmt.Errorf("failed to write newline: %w", err)
			}

			for _, evidence := range usages {
				snippet := strings.ReplaceAll(evidence.EvidenceSnippet, "\n", "\\n")
				if len(snippet) > 300 {
					snippet = snippet[:200] + "..."
				}
				err = collector.WriteString(packageHeader+identifierHeader, "    - "+snippet+"\n")
				if err != nil {
					return fmt.Errorf("failed to write newline: %w", err)
				}
			}
		}
	}

	summaryFiles := collector.GetFiles()
	// findingFIles := []string{}
	for idx, summaryFile := range summaryFiles[:min(20, len(summaryFiles))] {
		findingsFile := path.Join(findingsDir, "findings-"+strconv.Itoa(idx)+".json")
		fmt.Println("Analysing Summary file:", summaryFile, "->", findingsFile, "....")
		infer(summaryFile, findingsFile)
		fmt.Println("Analysis completed. Findings saved to", findingsFile)
		// findingFIles = append(findingFIles, findingsFile)
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
