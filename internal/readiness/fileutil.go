package readiness

import (
	"os"
)

func writeToFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
