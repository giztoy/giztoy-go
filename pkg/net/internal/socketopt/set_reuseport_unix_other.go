//go:build unix && !linux && !darwin && !freebsd && !netbsd && !openbsd

package socketopt

import "errors"

func SetReusePort(_ uintptr) error {
	return errors.New("SO_REUSEPORT not supported on this unix platform")
}
