# Go Code Review Comments Checklist (31 rules)

Walk through each rule against the sampled code. Same approach as the Effective Go checklist: note violations and good patterns.

1. **Gofmt** -- All code must be formatted with gofmt
2. **Comment Sentences** -- Comments are full sentences, start with the name being described, end with a period
3. **Contexts** -- `context.Context` is the first parameter; do not store contexts in structs
4. **Copying** -- Do not copy values that have pointer-method receivers (e.g., `sync.Mutex`)
5. **Crypto Rand** -- Use `crypto/rand` for cryptographic keys, not `math/rand`
6. **Declaring Empty Slices** -- Prefer `var t []string` over `t := []string{}` for nil slices
7. **Doc Comments** -- All exported names (types, functions, constants, variables) need doc comments
8. **Don't Panic** -- Use error returns for normal error handling, not `panic`
9. **Error Strings** -- Error messages are lowercase, no trailing punctuation
10. **Examples** -- Include runnable examples in `_test.go` files where helpful
11. **Goroutine Lifetimes** -- Every goroutine must have a clear exit condition; no goroutine leaks
12. **Handle Errors** -- Never discard errors with `_`; handle or propagate every error
13. **Imports** -- Group imports: stdlib first, blank line, then third-party packages
14. **Import Blank** -- Side-effect imports (`_ "pkg"`) only in main packages or test files
15. **Import Dot** -- Dot imports (`. "pkg"`) only for circular dependency test workarounds
16. **In-Band Errors** -- Return `(value, error)` not sentinel values or in-band error indicators
17. **Indent Error Flow** -- Handle errors first, return early; keep the happy path at minimal indentation
18. **Initialisms** -- `URL` not `Url`, `ID` not `Id`, `HTTP` not `Http`
19. **Interfaces** -- Define interfaces where they are used, not where they are implemented; return concrete types
20. **Line Length** -- No rigid limit, but break on semantic boundaries for readability
21. **Mixed Caps** -- `mixedCaps` for unexported, `MixedCaps` for exported; no `ALL_CAPS` except for C interop
22. **Named Result Parameters** -- Use only for godoc clarity; avoid naked returns with named results
23. **Naked Returns** -- Only in short, simple functions (a few lines)
24. **Package Comments** -- Adjacent to the `package` clause, no blank line between comment and declaration
25. **Package Names** -- Short, no stuttering (use `http.Client` not `http.HTTPClient`)
26. **Pass Values** -- Pass by value unless pointer semantics are needed (mutation, large structs)
27. **Receiver Names** -- 1-2 letters, consistent across methods; never use `this`, `self`, or `me`
28. **Receiver Type** -- Pointer receiver for mutation or large structs; value receiver for small immutable types
29. **Synchronous Functions** -- Prefer synchronous APIs; let the caller add concurrency
30. **Useful Test Failures** -- Test failure messages should show: input, got, want
31. **Variable Names** -- Short names for small scopes (`i`, `r`); longer names for wider scopes

For each rule:
- If a violation is found: note the file, approximate location, and a concrete suggestion.
- If the code follows the rule well: note it as a good pattern.
- If the rule is not applicable: skip it silently.
