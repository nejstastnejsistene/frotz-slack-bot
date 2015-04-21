package main

import (
	"errors"
	"io"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// How long of a timeout should indicate the end of input.
const timeout = 10 * time.Millisecond

var CleanExit = errors.New("clean exit")

type Zork struct {
	cmd    *exec.Cmd
	chErr  chan error
	stdin  io.Writer
	stdout chan string
	mutex  sync.Mutex
}

func StartZork(dfrotz, datFile string) (z *Zork, initialOutput string, err error) {
	z = new(Zork)
	z.chErr = make(chan error)
	z.cmd = exec.Command(dfrotz, datFile)

	z.stdin, err = z.cmd.StdinPipe()
	if err != nil {
		return
	}
	var stdout io.ReadCloser
	stdout, err = z.cmd.StdoutPipe()
	if err != nil {
		return
	}

	if err := z.cmd.Start(); err != nil {
		log.Fatal(err)
	}
	go func() {
		status := z.cmd.Wait()
		if status == nil {
			status = CleanExit
		}
		z.chErr <- status
	}()
	z.stdout = sepByTimeout(stdout, timeout)

	initialOutput = <-z.stdout
	return
}

func (z *Zork) ExecuteCommand(command string) (output string, err error) {
	// Write the input (plus a newline) to zork.
	if _, err = z.stdin.Write([]byte(command + "\n")); err != nil {
		return
	}
	// Read the output from zork, excepting the process dying.
	var ok bool
	select {
	case output, ok = <-z.stdout:
		// The read will fail if the process exitted
		if !ok {
			err = <-z.chErr
		}
		return
	case err = <-z.chErr:
		return
	}
}

func (z *Zork) Close() error {
	// Signal 0 checks if the process is alive.
	if z.cmd.Process.Signal(syscall.Signal(0)) == nil {
		return z.cmd.Process.Kill()
	}
	return nil
}

// Returns a channel that yields all data received by a reader only after
// a timeout is reached. Needed for zork because there is never an EOF, and you
// ordinarly only know the end of input by visually seeing that no more input
// is coming.
func sepByTimeout(r io.ReadCloser, timeout time.Duration) chan string {
	// Continuosly read chunks.
	chunks := make(chan []byte)
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := r.Read(chunk)
			// End input on error or if nothing is read.
			if n == 0 || err != nil {
				r.Close()
				close(chunks)
				return
			}
			chunks <- chunk[:n]
		}
	}()
	// Accumulate chunks until the timeout is reached.
	result := make(chan string)
	var buf []byte
	go func() {
		for {
			select {
			// Another chunk, timeout not reached yet.
			case chunk, ok := <-chunks:
				// End of input.
				if !ok {
					if len(buf) > 0 {
						result <- string(buf)
					}
					close(result)
					return
				}
				buf = append(buf, chunk...)
			// Timeout occurs before next data, yield accumulated buffer.
			case <-time.After(timeout):
				if len(buf) > 0 {
					result <- string(buf)
					buf = nil
				}
			}
		}
	}()
	return result
}
