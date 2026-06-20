//go:build windows && amd64

package vaultblob

import _ "embed"

//go:embed libs/libvaultblob_windows_amd64.dll
var windowsAmd64Lib []byte

func init() { libBinary = windowsAmd64Lib }
