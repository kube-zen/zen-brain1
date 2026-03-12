# Factory Template Escape Sequence Issues

## Status

Three factory templates are **temporarily disabled** due to complex escape sequence issues in their Command string fields:
- `registerRepoAwareDocsTemplate()` (line 326)
- `registerRepoAwareCICDTemplate()` (line 484)
- `registerRepoAwareMigrationTemplate()` (line 644)

## Root Cause

The templates contain extremely long shell commands with complex escaping:
- Multi-line shell commands using heredocs
- Backticks, dollar signs, quotes requiring proper Go string escaping
- Sequences like `\`, `\$`, `\n` etc. that confuse Go's parser

The Go compiler reports:
```
line 358: unknown escape
line 700: invalid character U+0024 '$'
line 700: syntax error: unexpected name f in composite literal
```

## Fix Approach

To properly fix these templates:
1. Extract each Command string
2. Rewrite with proper Go escaping:
   - `\` for backticks (valid escape)
   - `\$` for dollar signs (not valid escape - use `$` in double-quoted strings)
   - `\\` for literal backslashes
   - `\"` for literal double quotes
   - `\n` for actual newlines
3. Test compilation
4. Re-enable the registration calls

## Temporary Workaround

Registration calls are commented out in `registerRepoAwareTemplates()`:
```go
// r.registerRepoAwareDocsTemplate() // DISABLED: Complex escape sequences in Command strings
// r.registerRepoAwareCICDTemplate() // DISABLED: Complex escape sequences in Command strings
// r.registerRepoAwareMigrationTemplate() // DISABLED: Complex escape sequences in Command strings
```

The function bodies remain in the file but are not registered, so they don't affect compilation.

## Priority

Block 4 (Factory) is **nice-to-have** for current ~95% completeness goal:
- Block 2 (Office): ✅ Fixed
- Block 3 (Nervous System): ✅ Working
- Block 5 (Intelligence): ✅ Working
- Block 6 (DevEx): ✅ Working

Factory templates can be revisited after core validation completes.

## Related

- Commit: `a8d085a` (initial workaround)
- Commit: (this commit) (documentation)
- Patch pack: zen-brain_patch_pack_followup_2026-03-12.zip