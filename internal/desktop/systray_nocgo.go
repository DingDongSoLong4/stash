//go:build !cgo

package desktop

// The systray package requires cgo, so we cannot use it when building with cgo disabled.

func startSystray(exit chan<- int, favicon FaviconProvider) {

}
