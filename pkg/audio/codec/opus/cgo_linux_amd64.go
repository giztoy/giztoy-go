//go:build linux && amd64 && cgo

package opus

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/libopus/linux-amd64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/libopus/linux-amd64/lib -lopus -lm
#include <opus/opus.h>
*/
import "C"

var _ = C.int(0)
