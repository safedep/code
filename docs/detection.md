# APIs for detections

A detection reads files from repositories that can hold malware. It asks other questions than a
resolver: "is this call eval?", "is this argument a plain string?", "does this file put require on the
global object?". These APIs answer such questions. Each one reads hostile input with a size limit, a
node limit and the context deadline, and each walk is linear.

## Parse bytes as a named language

`parser.ParseBytes(ctx, language, data, opts...)` parses data as the given language. It does not look
at a file name. Node runs a required file of any name as JavaScript, so a detection parses a `.svg`
file as JavaScript.

- `parser.WithName(name)` sets the name that `tree.File().Name()` reports. A resolver that keys its
  results by file name reads it.
- `parser.WithMaxBytes(n)` sets the size limit. The default is `parser.DefaultMaxBytes` (32 MiB). A
  larger input gives `parser.ErrInputTooLarge`. Zero or less removes the limit.
- The parse stops when `ctx` is done and returns the error of `ctx`.

## Walk a tree and read its parse problems

`ts.Walk(ctx, root, opts, visit)` visits each named node in source order with a tree cursor. A loop
over `NamedChild(i)` is quadratic in the number of children, and a recursive walk can exhaust the
stack on a deep tree. `Walk` does neither. It returns `ts.ErrNodeLimit` at `opts.MaxNodes` (default
`ts.DefaultMaxNodes`). go-tree-sitter keeps a Go value for each node that a walk visits, so the limit
also bounds the memory. `ts.NamedChildren(n)` returns the named children of a node in linear time.

`ts.Diagnose(ctx, root, opts)` returns:

- `HasError`: an ERROR or a MISSING node anywhere in the tree;
- the positions of the ERROR nodes and of the MISSING nodes, at most 100 of each, with a count of all;
- `Kinds`: the type of each named node in the tree, such as `call_expression` or `jsx_element`.

## What a name, a call and an assignment stand for in JavaScript and TypeScript

`ecmascript.Analyze(ctx, tree, opts)` reads one JavaScript or TypeScript tree. It gives:

| Method | Answer |
| --- | --- |
| `Resolve(name)` | The values that a name stands for. |
| `Eval(node)` | The values that an expression stands for. |
| `Calls()` | Each call and new expression, with its callee, its callee name and its arguments. |
| `Assignments()` | Each assignment to a member of an object, with its key and a flag for a computed key. |
| `Bindings()` | Each name that the file binds, with the declaration and the value. |
| `StringValue(node)` | The value of a plain string, with its escapes decoded. |
| `ClassifyArg(node)` | The shape of an argument: a string, a template with a substitution, a name, a member, another literal or a computed value. |

A `Value` is a global name (`require`, `eval`), a module (`child_process`), or a member of either
(`child_process` `exec`). The analysis follows:

- `const`, `let` and `var` declarations, assignments, and chains of names;
- object destructuring, with renames and defaults;
- named, renamed, default and namespace imports, `await import('m')` and TypeScript
  `import x = require('m')`;
- `require('m')` through any name that stands for `require`, also from `createRequire`;
- members and string subscripts, so `globalThis['eval']` is `eval`;
- a member of the global object as a name, so `global.r = require` binds `r`;
- parentheses, comma expressions such as `(0, eval)`, and TypeScript assertions such as
  `require as any`;
- escapes in strings and names, so `'child\x5fprocess'` is `child_process` and `eval` is `eval`.

The analysis does not track scopes. A name stands for each value that the file binds to it anywhere,
and a name with no binding stands for the global of that name. A cycle of names, or a chain that is
longer than 32 names, gives a part of the values. A detection that must not miss a call can also check
the callee name in `Call.Name`.

At the node limit, `Analyze` returns the analysis of the nodes before the limit, and `Truncated`
reports it. See the example in `lang/ecmascript/example_test.go`.
