package vault

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrLocked is returned when another process holds the vault's write lock.
var ErrLocked = errors.New("vault is locked by another process")

// atomicWrite writes data to path so that a crash at any point leaves either
// the old file or the new one — never a partial write: temp file in the same
// directory + fsync + rename over the target.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// acquireLock takes the sidecar lock file (<vault>.lock) so GUI and CLI never
// write concurrently. O_EXCL makes creation atomic; a lock older than
// staleLockAge is treated as leftover from a crash and stolen.
const staleLockAge = 30 * time.Second

func acquireLock(path string) (func(), error) {
	lockPath := path + ".lock"
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			fmt.Fprintf(f, "pid=%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		info, statErr := os.Stat(lockPath)
		if statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			os.Remove(lockPath)
			continue
		}
		return nil, ErrLocked
	}
	return nil, ErrLocked
}
