package ops

import (
	"fmt"
	"os"
	"time"
)

const mixLogFile = "bridge.log"

// MixLog appends a timestamped debug line to bridge.log.
func MixLog(format string, args ...interface{}) {
	f, err := os.OpenFile(mixLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "[%s] %s\n", ts, msg)
}
