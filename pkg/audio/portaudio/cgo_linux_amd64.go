//go:build cgo && linux && amd64

package portaudio

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/audio/prebuilt/portaudio/linux-amd64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/audio/prebuilt/portaudio/linux-amd64/lib -lportaudio -lpthread -lm -ldl
#include <portaudio.h>
*/
import "C"

var _ = C.int(0)
