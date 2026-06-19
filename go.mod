module github.com/f4vzvy99f7-sys/vaultblob-go

go 1.26

require (
	github.com/ebitengine/purego v0.10.1
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/darwin_amd64 v0.1.9
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/darwin_arm64 v0.1.9
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/linux_amd64 v0.1.9
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/linux_arm64 v0.1.9
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/windows_amd64 v0.1.9
)

replace (
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/darwin_amd64 => ./bin/darwin_amd64
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/darwin_arm64 => ./bin/darwin_arm64
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/linux_amd64 => ./bin/linux_amd64
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/linux_arm64 => ./bin/linux_arm64
	github.com/f4vzvy99f7-sys/vaultblob-go/bin/windows_amd64 => ./bin/windows_amd64
)
