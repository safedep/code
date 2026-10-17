# APIs for detections

A detection reads files from repositories that can hold malware. It asks other questions than a
resolver: "is this call eval?", "is this argument a plain string?", "does this file put require on the
global object?". These APIs answer such questions. Each one reads hostile input with a size limit, a
node limit and the context of the caller, and each walk is linear.

Pass a context with a deadline. Some inputs make the tree-sitter parse itself slow, as a long chain of
TypeScript assertions that the JavaScript grammar reads.

## Parse bytes as a named language

`parser.ParseBytes(ctx, language, data, opts...)` parses data as the given language. It does not look
at a file name. Node runs a required file of any name as JavaScript, so a detection parses a `.svg`
file as JavaScript. The tree keeps a copy of data.

- `parser.WithFileName(name)` sets the name that `tree.File().Name()` and `Path()` report. A
  resolver that keys its results by file name reads it.
- `parser.WithMaxBytes(n)` sets the size limit. Zero selects `parser.DefaultMaxBytes` (4 MiB). A
  negative value removes the limit. A larger input gives `parser.ErrInputTooLarge`.
- The parse stops when `ctx` is done and returns the error of `ctx`.

`lang.NewTypescriptLanguage` parses with the TSX grammar, which reads the type assertion `<T>x` as
JSX. An ERROR node then takes the rest of the file. Parse `.ts`, `.mts` and `.cts` files with
`lang.NewTypescriptWithoutJSXLanguage`.

## Walk a tree and read its parse problems

`ts.Walk(ctx, root, opts, visit)` visits each named node in source order with a tree cursor. A loop
over `NamedChild(i)` is quadratic in the number of children, and a recursive walk can exhaust the
stack on a deep tree. `Walk` does neither. It returns `ts.ErrNodeLimit` when it reaches
`opts.MaxNodes` nodes, named or anonymous. Zero selects `ts.DefaultMaxNodes` (one million), and a
negative value removes the limit. go-tree-sitter keeps a Go value for each node that a walk reaches,
so the limit also bounds the memory. `ts.NamedChildren(n)` returns the named children of a node in
linear time.

`ts.Diagnose(ctx, root, maxNodes)` returns:

- `HasError`: an ERROR or a MISSING node anywhere in the tree;
- the positions of the ERROR nodes and of the MISSING nodes, at most 100 of each, with a count of all;
- `Kinds`: the type of each named node in the tree, such as `call_expression` or `jsx_element`.

## What a name, a call and an assignment stand for in JavaScript and TypeScript

`ecmascript.Analyze(ctx, tree, opts)` reads one JavaScript or TypeScript tree and returns an
`Analysis`:

| Method | Answer |
| --- | --- |
| `Resolve(name)` | The values that a name stands for. |
| `Eval(node)` | The values that an expression stands for. |
| `Calls()` | Each call and new expression, with its callee, its callee name and its arguments. |
| `Assignments()` | Each assignment to a member of an object, with its key, a flag for a computed key and a flag for the global object. |
| `Bindings()` | Each name that the file binds, with the binding node, the right side and the member that the name takes. |
| `StringValue(node)` | The value of a plain string or of a sum of plain strings, with its escapes decoded. |
| `StringPrefix(node)` | The text of a string or a template up to its first substitution. |
| `ClassifyArg(node)` | The shape of an argument: a string, a template with a substitution, a name, a member, another literal or a computed value. |

`ecmascript.Args(call)` returns the arguments of any call node.

A `Value` is a global name (`require`, `eval`), a module (`child_process`), or a member of either
(`child_process` `exec`). The analysis follows:

- `const`, `let` and `var` declarations, assignments, assignments used as values, logical
  assignments, default parameters and chains of names;
- object destructuring, with renames and defaults;
- named, renamed, default and namespace imports, `await import('m')`, the `default` member of a
  module, and TypeScript `import x = require('m')`;
- `require('m')` through any name that stands for `require`, `createRequire`, `module.require`,
  `process.mainModule.require` and `require.main.require`;
- members and string subscripts, so `globalThis['eval']` is `eval`;
- a member of the global object as a name, so `global.r = require` binds `r`;
- `.call`, `.apply` and `.bind`, so `eval.call(null, x)` calls `eval`;
- both branches of `||`, `&&`, `??` and `?:`;
- parentheses, comma expressions such as `(0, eval)`, and TypeScript assertions such as
  `require as any`;
- escapes in strings and names: hex, unicode and legacy octal escapes, so `'child\x5fprocess'` is
  `child_process`;
- sums of plain strings, so `'ev' + 'al'` is `eval`.

The analysis does not track scopes. A name stands for each value that the file binds to it anywhere.
A name also stands for the global of that name, unless the file declares it at its top level. A
declaration in a function, or a `let` or `const` in a block, hides the global only there. A member of the global object always
stands for the global, so `globalThis.eval` is `eval` also in a file that declares a local `eval`.

After the walk, the analysis solves the names to a fixed point. A result does not depend on the order
of the calls of a detection. A set of values holds at most 16 values. A value that the limit drops marks the analysis as truncated. At the node limit, at the
work budget of the fixed point, or after four rounds of aliases of the global object, `Analyze`
returns what it has and `Truncated` reports it. A detection
that must not miss a call can also check the callee name in `Call.Name`.

See the example in `lang/ecmascript/example_test.go`.
