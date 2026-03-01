package genx

import (
	"errors"
	"io"
)

func isDoneErr(err error) bool {
	return errors.Is(err, ErrDone) || errors.Is(err, io.EOF)
}
