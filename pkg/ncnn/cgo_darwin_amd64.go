//go:build darwin && amd64 && cgo

package ncnn

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/ncnn/prebuilt/darwin-amd64/include
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/ncnn/prebuilt/darwin-amd64/lib -lncnn -lc++
#include <ncnn/c_api.h>
*/
import "C"

var _ = C.int(0)
