# ADR

We are not building a static analysis tool that requires an intermediate
representation of code for generalised query and analysis. We are building
a system for lightweight discovery, parsing and analysis of code for software
supply chain security use-cases.

Supply chain use-cases include:

- Ability to build a call graph spanning across imported libraries
- Ability to track usage of imports, classes and functions
- Ability to track data flow within a source file, extending to multiple files
- Ability to build xBOM based on code and database of known signatures
- Ability to read snippets of code, uniquely identified by a snippet ID

A framework that provides the necessary building blocks will help development of
various use-cases faster and more efficiently. The key building blocks are:

- Discovery: Ability to discover and parse code from various sources
- Parsing: Ability to parse code into an AST and derive common primitives
- Data Model: A data model to represent common code primitives such as
    source file, imports, classes, functions, function calls etc.
- Plugin System: Ability to extend the system with custom plugins for various
    analysis and extensions

Common plugins include

- Import Resolver: Ability to resolve imports and load the file for analysis

Like most analysis systems, higher level of abstractions are built on top
of lower level abstractions. This means, the analysis plugins that produce
an entity, such as an `import` can be treated as a primitive for other analysis
plugins such as `import-usage`. The framework should allow cascading of analysis
plugins to build higher level abstractions.

## References

- https://ast-grep.github.io/advanced/core-concepts.html
- https://tree-sitter.github.io/tree-sitter/
- https://github.com/joernio/joern
- https://github.com/fasten-project/fasten
