package native

import "fmt"

func fmtSprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
