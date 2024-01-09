//go:build brotli
// +build brotli

package ui

import (
	"github.com/vearutop/statigz/brotli"
)

func init() {
	brotli.AddEncoding(UIServer)
}
