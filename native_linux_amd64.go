//go:build linux && amd64

package vaultblob

import _ "embed"

//go:embed libs/libvaultblob_linux_amd64.so
var linuxAmd64Lib []byte

func init() { libBinary = linuxAmd64Lib }
