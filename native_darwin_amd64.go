//go:build darwin && amd64

package vaultblob

import _ "embed"

//go:embed libs/libvaultblob_darwin_amd64.dylib
var darwinAmd64Lib []byte

func init() { libBinary = darwinAmd64Lib }
