//go:build darwin && amd64 && cgo

package opus

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/libopus/darwin-amd64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/libopus/darwin-amd64/lib -lopus
#include <opus/opus.h>
*/
import "C"

var _ = C.int(0)
