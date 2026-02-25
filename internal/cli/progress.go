package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

type spinnerStop func(err error)

func startSpinner(out io.Writer, label string) spinnerStop {
	if !isTTYWriter(out) {
		return func(error) {}
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	frames := []string{"|", "/", "-", `\`}

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(out, "\r%s %s", frames[i%len(frames)], label)
				i++
			}
		}
	}()

	return func(err error) {
		close(done)
		<-stopped
		if err != nil {
			fmt.Fprintf(out, "\r! %s failed\n", label)
			return
		}
		fmt.Fprintf(out, "\r+ %s done\n", label)
	}
}

func isTTYWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
