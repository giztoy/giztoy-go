//go:build darwin && arm64 && cgo

package opus

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/libopus/darwin-arm64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/libopus/darwin-arm64/lib -lopus
#include <opus/opus.h>
*/
import "C"

var _ = C.int(0)
