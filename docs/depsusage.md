## Dependency Usage Plugin
The depsusage plugin helps identify and analyze if and where dependencies are used within your codebase. The plugin scans codebase and identifies the imports and corresponding identifiers. It uses this identifier -> import association to identify actual usage of dependencies in rest of the code.

The depsusage plugin accepts a `depsusage.DependencyUsageCallback` which is invoked on capturing any evidence in the codebase. It has two parameters, `context.Context` and [depsusage.UsageEvidence](usageevidence.md)


### Usage example with plugin executor
```go
// callback to consume usage evidences
var usageCallback depsusage.DependencyUsageCallback = func(ctx context.Context, evidence *depsusage.UsageEvidence) error {
  fmt.Println(evidence)
  return nil
}

// Plugin instance
pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
	depsusage.NewDependencyUsagePlugin(usageCallback),
})

// Execute plugin
pluginExecutor.Execute(context.Background(), fileSystem)

```

