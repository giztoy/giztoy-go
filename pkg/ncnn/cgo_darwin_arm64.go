//go:build darwin && arm64 && cgo

package ncnn

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/ncnn/prebuilt/darwin-arm64/include
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/ncnn/prebuilt/darwin-arm64/lib -lncnn -lc++
#include <ncnn/c_api.h>
*/
import "C"

var _ = C.int(0)
