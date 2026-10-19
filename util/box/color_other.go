//go:build !windows

package box

import "os"

// enableColor has nothing to do outside Windows: terminals there draw colour
// sequences as they are
func enableColor(f *os.File) bool {
	return true
}
