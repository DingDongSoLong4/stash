package ui

import (
	"embed"
	"io/fs"
	"runtime"

	"github.com/vearutop/statigz"
)

//go:embed v2.5/build
var uiBox embed.FS
var UIBox fs.FS
var UIServer *statigz.Server

//go:embed login
var loginUIBox embed.FS
var LoginUIBox fs.FS
var LoginUIServer *statigz.Server

func init() {
	var err error
	UIBox, err = fs.Sub(uiBox, "v2.5/build")
	if err != nil {
		panic(err)
	}
	UIServer = statigz.FileServer(UIBox.(fs.ReadDirFS))

	LoginUIBox, err = fs.Sub(loginUIBox, "login")
	if err != nil {
		panic(err)
	}
	LoginUIServer = statigz.FileServer(LoginUIBox.(fs.ReadDirFS))
}

type faviconProvider struct{}

var FaviconProvider = faviconProvider{}

func (p *faviconProvider) GetFavicon() []byte {
	if runtime.GOOS == "windows" {
		ret, _ := fs.ReadFile(UIBox, "favicon.ico")
		return ret
	}

	return p.GetFaviconPng()
}

func (p *faviconProvider) GetFaviconPng() []byte {
	ret, _ := fs.ReadFile(UIBox, "favicon.png")
	return ret
}
