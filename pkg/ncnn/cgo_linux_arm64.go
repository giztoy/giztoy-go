//go:build linux && arm64 && cgo

package ncnn

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/ncnn/prebuilt/linux-arm64/include
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/ncnn/prebuilt/linux-arm64/lib -lncnn -lstdc++ -lpthread -lm
#include <ncnn/c_api.h>
*/
import "C"

var _ = C.int(0)
