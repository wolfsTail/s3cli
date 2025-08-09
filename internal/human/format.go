package human

import (
	"fmt"
	"time"
)

func Bytes(n int64) string {
	const (
		_         = 1 << (10 * iota)
		KiB int64 = 1 << (10 * iota)
		MiB
		GiB
		TiB
	)
	switch {
	case n >= TiB:
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	case n >= GiB:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func Time(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2025-01-02 15:22")
}
