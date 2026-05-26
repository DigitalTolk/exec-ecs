//go:build !windows

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// runPTYCommand runs cmd on a PTY, wires stdin/stdout/SIGWINCH like a real
// terminal would, and returns the child's exit code once it terminates.
func runPTYCommand(cmd *exec.Cmd) (int, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("start pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)
	go func() {
		for range resizeCh {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	resizeCh <- syscall.SIGWINCH

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		_ = cmd.Wait()
		return 1, fmt.Errorf("raw mode: %w", err)
	}
	restore := func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }
	defer restore()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		s, ok := <-sigCh
		if !ok {
			return
		}
		restore()
		if cmd.Process != nil {
			_ = cmd.Process.Signal(s)
		}
	}()
	defer signal.Stop(sigCh)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, ptmx)
	}()

	stopStdin := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		for {
			select {
			case <-stopStdin:
				return
			default:
			}
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if _, werr := ptmx.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
	close(stopStdin)

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
