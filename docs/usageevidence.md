# Usage evidence

`depsusage.UsageEvidence` represents the evidence of usage of a module in a file. It is generated in [depsusage](depsusage.md) plugin. Fields -

- PackageHint - string
  
  Imported modules aren't exactly same as packages they refer to. It can be a submodule with separators or the top-level module may not match exact package name. As a usage evidence, PackageHint reports the hint of actual dependency being used.
  
  For example, the `PyYAML` package is imported as `yaml` or `yaml.composer` where the imported top level module `yaml` isn't equal to the package name.

  PackageHint is resolved from the `ModuleName` provided by [ImportNode](/core/ast/import.go) by resolving the base module and searching it in the top-level module to dependency mapping [Read more](https://github.com/safedep/code/issues/6).
  

  Moreover, this may not be the final truth, since different languages & package managers may have some package aliasing eg. Shadow JAR in java. Hence, it is just a "hint".
  This can be verified or enriched accurately by the consumer of this API using the required package manifest information which isn't in scope of code analysis framework.

- Identifier - string

  The identifier that was mentioned in the code leading to this Usage evidence. It can be an imported function, class or variable.
  
  eg.
  ```python
  import ujson
  ujson.decode('{"a": 1, "b": 2}')
  ```
  
  Here, the identifier `ujson` was used, leading to this UsageEvidence

- FilePath - string

  File path where the dependency was used

- Line - uint
	
  Line number where the usage was found
  
  Note - Line number of usage is reported, not the import

Fields taken directly from ImportNode. [Read more](imports.md)
- ModuleName - string
- ModuleItem - string
- ModuleAlias - string
- IsWildCardUsage - bool

