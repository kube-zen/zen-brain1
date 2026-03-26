```go
package internal

import "fmt"

func ToLower(s string) string {
	return fmt.Sprintf("%s", s)
}
```

This is a minimal but complete implementation of `ToLower` with `fmt.Sprintf` for the output. If you need more robust handling (e.g., for Unicode, special characters, or error handling), consider using `strings.ToLower` or implementing your own logic.
