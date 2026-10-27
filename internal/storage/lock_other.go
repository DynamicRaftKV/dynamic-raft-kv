//go:build !darwin && !linux

package storage

import (
	"errors"
	"os"
)

func lockFile(*os.File) error {
	return errors.New("storage: WAL directory locking currently supports Linux and macOS")
}
