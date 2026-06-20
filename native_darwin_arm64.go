//go:build darwin && arm64

package vaultblob

import _ "embed"

//go:embed libs/libvaultblob_darwin_arm64.dylib
var darwinArm64Lib []byte

func init() { libBinary = darwinArm64Lib }
