//go:build cgo && linux && arm64

package portaudio

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/audio/prebuilt/portaudio/linux-arm64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/audio/prebuilt/portaudio/linux-arm64/lib -lportaudio -lpthread -lm -ldl
#include <portaudio.h>
*/
import "C"

var _ = C.int(0)
