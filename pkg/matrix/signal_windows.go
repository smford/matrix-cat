//go:build windows

package matrix

import "os"

func notifyResize(ch chan<- os.Signal) {
	// Windows terminal resizing is handled via console events; no SIGWINCH
}
