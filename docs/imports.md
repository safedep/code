# Imports

Imports returned by `language.Resolvers().ResolveImports()` are represented as [ImportNode](/core/ast/import.go) in a language agnostic manner. It has following fields -

- `moduleNameNode` exposed by `ModuleName` 

  The sitter node referring to the imported package or module. It contains entire module name, a non-empty string which can be resolved to the target package or source file on the disk.

  eg. In python, ModuleName `x.y.z` is resolved for imports - `from x.y.z import p` is `x.y.z`, `import x.y.z` or `import x.y.z as xz`

  eg. In javascript, ModuleName can be `../relative/import`, `@gilbarbara/eslint-config`, `express`
  
- `moduleItemNode` exposed by `ModuleItem`

	The sitter node referring to the specific item (function, class, variable, etc) imported from the `ModuleName` mentioned above. It is an empty string if the entire module is imported.

  eg. For python import `from sklearn import dastasets as ds` is resolved to ModuleItem - `datasets`

  eg. For javascript import `import { hex } from 'chalk/ansi-styles'`, ModuleItem is `hex`

- `moduleAliasNode` exposed by `ModuleAlias`

	The sitter node referring to alias of the import in the current scope. It is mapped as equivalent to the `ModuleItem` (if it is empty, then `ModuleName`).  If no alias is specified in code, then it contains the node for actual Moduleltem or ModuleName.

  eg. For python import `from sklearn import datasets as ds`, alias is `ds` referring to ModuleItem - `datasets`
  However, for `import pandas as pd`, alias `pd` refers to ModuleName - `pandas` since ModuleItem is empty.

- `isWildcardImport` exposed by `IsWildcardImport`

	Boolean flag Indicating whether the import is a wildcard import

  eg. In python - `from seaborn import *`

  eg. In java - `import java.util.*`


## Note
For composite imports, multiple `ImportNode`s are generated.
For example, `import ReactDOM, { render, flushSync as flushIt } from 'react-dom'` is resolved to three import nodes -
```
ImportNode{ModuleName: react-dom, ModuleItem: , ModuleAlias: ReactDOM, WildcardImport: false}
ImportNode{ModuleName: react-dom, ModuleItem: render, ModuleAlias: render, WildcardImport: false}
ImportNode{ModuleName: react-dom, ModuleItem: flushSync, ModuleAlias: flushIt, WildcardImport: false}
```

For different edge cases refer to `ImportExpectations` testcases in `_test` files in [lang/](/lang) directory
