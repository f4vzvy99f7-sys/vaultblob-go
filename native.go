package vaultblob

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	libLoaded bool
	libOnce   sync.Once
	libErr    error
)

func ensureLib() error {
	libOnce.Do(func() {
		if libBinary == nil {
			libErr = errors.New("vaultblob: native library not available for " + runtime.GOOS + "/" + runtime.GOARCH)
			return
		}

		cacheDir, err := os.UserCacheDir()
		if err != nil {
			cacheDir = os.TempDir()
		}
		libDir := filepath.Join(cacheDir, "vaultblob-go")
		os.MkdirAll(libDir, 0755)
		libPath := filepath.Join(libDir, libFilename())

		os.WriteFile(libPath, libBinary, 0755)

		handle, err := purego.Dlopen(libPath, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			libErr = err
			return
		}

		purego.RegisterLibFunc(&vaultOpen, handle, "vaultblob_open")
		purego.RegisterLibFunc(&vaultClose, handle, "vaultblob_close")
		purego.RegisterLibFunc(&vaultPutFile, handle, "vaultblob_put_file")
		purego.RegisterLibFunc(&vaultReadFile, handle, "vaultblob_read_file")
		purego.RegisterLibFunc(&vaultReadFileRange, handle, "vaultblob_read_file_range")
		purego.RegisterLibFunc(&vaultFileSize, handle, "vaultblob_file_size")
		purego.RegisterLibFunc(&vaultVaultID, handle, "vaultblob_vault_id")
		purego.RegisterLibFunc(&vaultBlobCount, handle, "vaultblob_blob_count")
		purego.RegisterLibFunc(&vaultBlobID, handle, "vaultblob_blob_id")
		purego.RegisterLibFunc(&vaultFileChunkCount, handle, "vaultblob_file_chunk_count")
		purego.RegisterLibFunc(&vaultFileChunk, handle, "vaultblob_file_chunk")

		libLoaded = true
	})
	return libErr
}

var libBinary []byte

func libFilename() string {
	switch runtime.GOOS {
	case "darwin":
		return "libvaultblob.dylib"
	case "linux":
		return "libvaultblob.so"
	case "windows":
		return "vaultblob.dll"
	default:
		return "libvaultblob.so"
	}
}

// ---------------------------------------------------------------------------
// C result struct types — layouts match #[repr(C)] in lib.rs
// ---------------------------------------------------------------------------

type result struct {
	Code int32
}

type u64Result struct {
	Code  int32
	Value uint64
}

type sizeResult struct {
	Code  int32
	Value uint64
}

type hexResult struct {
	Code  int32
	Value [37]byte
}

type chunkInfo struct {
	BlobIDHex       [37]byte
	SequenceNumber  uint64
	OffsetInBlob    uint64
	PlaintextLength uint64
	ContentHash     [32]byte
}

type chunkInfoResult struct {
	Code  int32
	Value chunkInfo
}

// ---------------------------------------------------------------------------
// C function pointer variables
// ---------------------------------------------------------------------------

var (
	vaultOpen func(path, masterKey *byte, maxChunkSize, maxBlobSize uint64,
		split, stripe int32, out *u64Result)

	vaultClose func(session uint64, out *result)

	vaultPutFile func(session uint64, data *byte, dataLen int, fileID *byte,
		out *hexResult)

	vaultReadFile func(session uint64, fileID *byte, buffer *byte, bufferLen uint64,
		out *u64Result)

	vaultReadFileRange func(session uint64, fileID *byte, offset, length uint64,
		buffer *byte, bufferLen uint64, out *u64Result)

	vaultFileSize func(session uint64, fileID *byte, out *u64Result)

	vaultVaultID func(session uint64, out *hexResult)

	vaultBlobCount func(session uint64, out *sizeResult)

	vaultBlobID func(session uint64, index uint64, out *hexResult)

	vaultFileChunkCount func(session uint64, fileID *byte, out *sizeResult)

	vaultFileChunk func(session uint64, fileID *byte, index uint64,
		out *chunkInfoResult)
)

// ---------------------------------------------------------------------------
// C string helpers
// ---------------------------------------------------------------------------

func cstr(s string) *byte {
	if s == "" {
		return nil
	}
	b := append([]byte(s), 0)
	return &b[0]
}

func cstrFromPtr(p *byte) string {
	if p == nil {
		return ""
	}
	end := unsafe.Pointer(p)
	for *(*byte)(end) != 0 {
		end = unsafe.Add(end, 1)
	}
	n := uintptr(end) - uintptr(unsafe.Pointer(p))
	return string(unsafe.Slice(p, n))
}
