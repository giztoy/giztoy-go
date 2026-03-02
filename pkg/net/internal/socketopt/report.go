package socketopt

import (
	"fmt"
	"strings"
)

// OptimizationEntry records the result of a single socket optimization attempt.
type OptimizationEntry struct {
	Name    string
	Applied bool
	Detail  string
	Err     error
}

// OptimizationReport collects the results of all optimization attempts.
type OptimizationReport struct {
	Entries []OptimizationEntry
}

func (r *OptimizationReport) String() string {
	var b strings.Builder
	b.WriteString("[udp] socket optimizations:")
	for _, e := range r.Entries {
		if e.Applied {
			fmt.Fprintf(&b, "\n  %-40s [ok]", e.Detail)
		} else if e.Err != nil {
			fmt.Fprintf(&b, "\n  %-40s [not available: %v]", e.Name, e.Err)
		} else if e.Detail != "" {
			fmt.Fprintf(&b, "\n  %-40s [%s]", e.Name, e.Detail)
		} else {
			fmt.Fprintf(&b, "\n  %-40s [skipped]", e.Name)
		}
	}
	return b.String()
}

// firstError returns the first non-nil error from the arguments.
func firstError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
