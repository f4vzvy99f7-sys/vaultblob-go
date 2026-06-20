//go:build linux && arm64

package vaultblob

import _ "embed"

//go:embed libs/libvaultblob_linux_arm64.so
var linuxArm64Lib []byte

func init() { libBinary = linuxArm64Lib }
