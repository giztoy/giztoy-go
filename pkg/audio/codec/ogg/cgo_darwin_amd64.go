//go:build darwin && amd64 && cgo

package ogg

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/libogg/darwin-amd64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/libogg/darwin-amd64/lib -logg
#include <ogg/ogg.h>
*/
import "C"

var _ = C.int(0)
