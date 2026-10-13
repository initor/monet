# Effective Go Convention Checklist (22 categories)

Walk through each category against the sampled code. For each category, check whether the code follows the convention. Note violations and good patterns.

1. **Formatting** -- Code uses gofmt; opening braces on same line
2. **Names: Packages** -- Lowercase, single-word, no underscores or mixedCaps
3. **Names: Getters** -- No `Get` prefix on getters; `Set` prefix for setters
4. **Names: Interfaces** -- One-method interfaces use `-er` suffix (Reader, Writer, Stringer)
5. **Names: MixedCaps** -- MixedCaps for exported, mixedCaps for unexported; never underscores in names
6. **Control: If** -- Supports init statement; omit unnecessary else after early return
7. **Control: For** -- Three forms (C-style, while, range); use `range` for collections
8. **Control: Switch** -- No automatic fall-through; use expression switches for type dispatch
9. **Functions: Multiple returns** -- Use the `(result, error)` return pattern
10. **Functions: Named results** -- Use for documentation; avoid naked returns in long functions
11. **Functions: Defer** -- Use defer for resource cleanup (close files, unlock mutexes); LIFO execution order
12. **Data: new vs make** -- `new` for zero-value pointers; `make` for slices, maps, and channels
13. **Data: Slices** -- Use `append` to grow; nil slices are valid (no need to `make` an empty slice)
14. **Data: Maps** -- Use comma-ok idiom for existence checks; nil maps panic on write
15. **Methods: Pointer vs value receivers** -- Pointer receivers when mutating state; value receivers for read-only
16. **Interfaces: Design** -- Small interfaces (1-3 methods); interfaces are satisfied implicitly
17. **Interfaces: Embedding** -- Promotes methods from embedded types; avoids forwarding boilerplate
18. **Concurrency: Goroutines** -- Share memory by communicating, not communicate by sharing memory
19. **Concurrency: Channels** -- Unbuffered channels synchronize; buffered channels decouple
20. **Errors: Interface** -- Return `error` as the final return value; nil means success
21. **Errors: Custom types** -- Include package and operation prefix in error messages
22. **Errors: Panic/Recover** -- Panic only for truly exceptional situations; recover only in deferred functions

For each category:
- If a violation is found: note the file, approximate location, what the convention expects, and a concrete suggestion.
- If the code follows the convention well: note it as a good pattern (collect these for the "Good Patterns" section).
- If the convention is not applicable to the sampled code: skip it silently.
