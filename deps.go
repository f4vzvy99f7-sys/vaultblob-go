//go:build tools

package vaultblob

import (
	_ "github.com/f4vzvy99f7-sys/vaultblob-go/bin/darwin_amd64"
	_ "github.com/f4vzvy99f7-sys/vaultblob-go/bin/darwin_arm64"
	_ "github.com/f4vzvy99f7-sys/vaultblob-go/bin/linux_amd64"
	_ "github.com/f4vzvy99f7-sys/vaultblob-go/bin/linux_arm64"
	_ "github.com/f4vzvy99f7-sys/vaultblob-go/bin/windows_amd64"
)
