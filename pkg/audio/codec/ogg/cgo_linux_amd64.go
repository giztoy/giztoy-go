//go:build linux && amd64 && cgo

package ogg

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/libogg/linux-amd64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/libogg/linux-amd64/lib -logg -lm
#include <ogg/ogg.h>
*/
import "C"

var _ = C.int(0)
