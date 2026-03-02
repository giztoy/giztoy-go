//go:build cgo && darwin && arm64

package portaudio

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/audio/prebuilt/portaudio/darwin-arm64/include
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/audio/prebuilt/portaudio/darwin-arm64/lib -lportaudio -framework AudioToolbox -framework AudioUnit -framework CoreAudio -framework CoreFoundation
#include <portaudio.h>
*/
import "C"

var _ = C.int(0)
