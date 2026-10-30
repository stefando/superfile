//go:build !windows

package gitinfo

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchRejectsNamedPipes(t *testing.T) {
	for _, metadata := range []string{".git", filepath.Join(".git", "HEAD")} {
		t.Run(metadata, func(t *testing.T) {
			root := t.TempDir()
			pipe := filepath.Join(root, metadata)
			require.NoError(t, os.MkdirAll(filepath.Dir(pipe), 0700))
			require.NoError(t, syscall.Mkfifo(pipe, 0600))

			// Keep a writer open so an incorrect read blocks until we release it.
			// This lets the test detect the hang without leaving a blocked goroutine.
			writer, err := os.OpenFile(pipe, os.O_RDWR|syscall.O_NONBLOCK, 0)
			require.NoError(t, err)
			t.Cleanup(func() { _ = writer.Close() })
			done := make(chan bool, 1)
			go func() {
				_, _, ok := Branch(root)
				done <- ok
			}()

			select {
			case ok := <-done:
				assert.False(t, ok)
			case <-time.After(time.Second):
				require.NoError(t, writer.Close())
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("metadata reader did not stop after releasing the pipe")
				}
				t.Fatal("branch detection blocked on a named pipe")
			}
		})
	}
}
