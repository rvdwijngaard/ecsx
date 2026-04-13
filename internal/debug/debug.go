package debug

import (
	"fmt"
	"log"
	"os"
	"time"
)

var logger *log.Logger

func Enable() {
	f, err := os.CreateTemp("", "ecsx-debug-*.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "debug: failed to create log file: %v\n", err)
		return
	}
	logger = log.New(f, "", 0)
	logger.Printf("ecsx debug log started at %s", time.Now().Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "debug: logging to %s\n", f.Name())
}

func Log(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Printf("[%s] %s", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
