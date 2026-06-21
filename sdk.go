package vaultblob

import (
	"errors"
	"fmt"
	"io"
)

type Options struct {
	MaxChunkSize uint64
	MaxBlobSize  uint64
	SplitFiles   bool
	StripeChunks bool
	Verbose      bool // kept as no-op for backward compatibility
}

var errorMessages = map[int32]string{
	0:   "ok",
	-1:  "I/O error",
	-2:  "AEAD decrypt failed",
	-3:  "AEAD encrypt failed",
	-4:  "index chain broken",
	-5:  "tail hash mismatch",
	-6:  "index MAC mismatch",
	-7:  "no valid locator",
	-8:  "invalid master key",
	-9:  "invalid config",
	-10: "invalid chunk size",
	-11: "file not found",
	-12: "incomplete file",
	-13: "file already exists",
	-14: "file hash mismatch",
	-15: "blob not found",
	-16: "no writable blob",
	-17: "file exceeds blob capacity",
	-18: "integer overflow",
	-19: "unsupported platform",
	-20: "retry budget exceeded",
	-21: "invalid session",
	-23: "buffer too small",
	-24: "invalid argument",
}

func vaultError(code int32) error {
	if msg, ok := errorMessages[code]; ok {
		return errors.New("vaultblob: " + msg)
	}
	return fmt.Errorf("vaultblob: unknown error code %d", code)
}

type Session struct {
	id uint64
}

func OpenVault(dir string, masterKey [32]byte, opts *Options) (*Session, error) {
	if err := ensureLib(); err != nil {
		return nil, err
	}

	maxChunk := uint64(4 * 1024 * 1024)
	maxBlob := uint64(1024 * 1024 * 1024)
	split := int32(0)
	stripe := int32(0)

	if opts != nil {
		if opts.MaxChunkSize != 0 {
			maxChunk = opts.MaxChunkSize
		}
		if opts.MaxBlobSize != 0 {
			maxBlob = opts.MaxBlobSize
		}
		if opts.SplitFiles {
			split = 1
		}
		if opts.StripeChunks {
			stripe = 1
		}
	}

	var out u64Result
	vaultOpen(cstr(dir), &masterKey[0], maxChunk, maxBlob, split, stripe, &out)
	if out.Code != 0 {
		return nil, vaultError(out.Code)
	}

	return &Session{id: out.Value}, nil
}

func (s *Session) Close() {
	if s.id == 0 {
		return
	}
	var out result
	vaultClose(s.id, &out)
	s.id = 0
}

func (s *Session) PutFile(data []byte) (string, error) {
	return s.PutFileWithID(data, "")
}

func (s *Session) PutFileWithID(data []byte, fileID string) (string, error) {
	if s.id == 0 {
		return "", errors.New("vaultblob: session closed")
	}

	var fileIDPtr *byte
	if fileID != "" {
		fileIDPtr = cstr(fileID)
	}

	var dataPtr *byte
	if len(data) > 0 {
		dataPtr = &data[0]
	}

	var out hexResult
	vaultPutFile(s.id, dataPtr, len(data), fileIDPtr, &out)
	if out.Code != 0 {
		return "", vaultError(out.Code)
	}

	return cstrFromPtr(&out.Value[0]), nil
}

func (s *Session) ReadFile(fileID string) ([]byte, error) {
	if s.id == 0 {
		return nil, errors.New("vaultblob: session closed")
	}

	var sizeOut u64Result
	vaultFileSize(s.id, cstr(fileID), &sizeOut)
	if sizeOut.Code != 0 {
		return nil, vaultError(sizeOut.Code)
	}

	buf := make([]byte, sizeOut.Value)
	if sizeOut.Value == 0 {
		return buf, nil
	}

	var readOut u64Result
	vaultReadFile(s.id, cstr(fileID), &buf[0], uint64(len(buf)), &readOut)
	if readOut.Code != 0 {
		return nil, vaultError(readOut.Code)
	}

	return buf[:readOut.Value], nil
}

func (s *Session) ReadFileRange(fileID string, offset, length uint64) ([]byte, error) {
	if s.id == 0 {
		return nil, errors.New("vaultblob: session closed")
	}

	// Allocate the full requested length — the C side copies what's available.
	if length == 0 {
		return nil, nil
	}

	buf := make([]byte, length)

	var out u64Result
	vaultReadFileRange(s.id, cstr(fileID), offset, length, &buf[0], uint64(len(buf)), &out)
	if out.Code != 0 {
		return nil, vaultError(out.Code)
	}

	return buf[:out.Value], nil
}

func (s *Session) NewFileReader(fileID string) *FileReader {
	return &FileReader{s: s, fileID: fileID}
}

func (s *Session) NewFileReadSeeker(fileID string) *FileReadSeeker {
	return &FileReadSeeker{FileReader: FileReader{s: s, fileID: fileID}}
}

type FileReader struct {
	s        *Session
	fileID   string
	offset   uint64
	fileSize uint64
}

func (r *FileReader) Read(p []byte) (int, error) {
	if r.s == nil || r.s.id == 0 {
		return 0, errors.New("vaultblob: session closed")
	}

	if len(p) == 0 {
		return 0, nil
	}

	if r.fileSize == 0 {
		size, err := r.s.FileSize(r.fileID)
		if err != nil {
			return 0, err
		}
		r.fileSize = size
	}

	if r.offset >= r.fileSize {
		return 0, io.EOF
	}

	length := uint64(len(p))
	remaining := r.fileSize - r.offset
	if length > remaining {
		length = remaining
	}

	var out u64Result
	vaultReadFileRange(r.s.id, cstr(r.fileID), r.offset, length, &p[0], uint64(len(p)), &out)
	if out.Code != 0 {
		return 0, vaultError(out.Code)
	}

	n := int(out.Value)
	r.offset += uint64(n)
	return n, nil
}

type FileReadSeeker struct {
	FileReader
}

func (rs *FileReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = int64(rs.offset) + offset
	case io.SeekEnd:
		if rs.fileSize == 0 {
			size, err := rs.s.FileSize(rs.fileID)
			if err != nil {
				return 0, err
			}
			rs.fileSize = size
		}
		newOffset = int64(rs.fileSize) + offset
	default:
		return 0, errors.New("vaultblob: invalid whence")
	}
	if newOffset < 0 {
		return 0, errors.New("vaultblob: negative position")
	}
	rs.offset = uint64(newOffset)
	return newOffset, nil
}

func (s *Session) FileSize(fileID string) (uint64, error) {
	if s.id == 0 {
		return 0, errors.New("vaultblob: session closed")
	}

	var out u64Result
	vaultFileSize(s.id, cstr(fileID), &out)
	if out.Code != 0 {
		return 0, vaultError(out.Code)
	}

	return out.Value, nil
}

func (s *Session) BlobIDs() ([]string, error) {
	if s.id == 0 {
		return nil, errors.New("vaultblob: session closed")
	}

	var countOut sizeResult
	vaultBlobCount(s.id, &countOut)
	if countOut.Code != 0 {
		return nil, vaultError(countOut.Code)
	}

	ids := make([]string, countOut.Value)
	for i := uint64(0); i < countOut.Value; i++ {
		var idOut hexResult
		vaultBlobID(s.id, i, &idOut)
		if idOut.Code != 0 {
			return nil, vaultError(idOut.Code)
		}
		ids[i] = cstrFromPtr(&idOut.Value[0])
	}
	return ids, nil
}

// VaultID returns the vault's unique ID as a hex string.
func (s *Session) VaultID() (string, error) {
	if s.id == 0 {
		return "", errors.New("vaultblob: session closed")
	}

	var out hexResult
	vaultVaultID(s.id, &out)
	if out.Code != 0 {
		return "", vaultError(out.Code)
	}

	return cstrFromPtr(&out.Value[0]), nil
}

// FileChunkCount returns the number of chunks in a stored file.
func (s *Session) FileChunkCount(fileID string) (int, error) {
	if s.id == 0 {
		return 0, errors.New("vaultblob: session closed")
	}

	var out sizeResult
	vaultFileChunkCount(s.id, cstr(fileID), &out)
	if out.Code != 0 {
		return 0, vaultError(out.Code)
	}

	return int(out.Value), nil
}

// ChunkInfo describes a single chunk in a stored file.
type ChunkInfo struct {
	BlobIDHex       string
	SequenceNumber  uint64
	OffsetInBlob    uint64
	PlaintextLength uint64
	ContentHash     [32]byte
}

// FileChunk returns info about the i-th chunk of a file.
func (s *Session) FileChunk(fileID string, index int) (*ChunkInfo, error) {
	if s.id == 0 {
		return nil, errors.New("vaultblob: session closed")
	}

	var out chunkInfoResult
	vaultFileChunk(s.id, cstr(fileID), uint64(index), &out)
	if out.Code != 0 {
		return nil, vaultError(out.Code)
	}

	return &ChunkInfo{
		BlobIDHex:       cstrFromPtr(&out.Value.BlobIDHex[0]),
		SequenceNumber:  out.Value.SequenceNumber,
		OffsetInBlob:    out.Value.OffsetInBlob,
		PlaintextLength: out.Value.PlaintextLength,
		ContentHash:     out.Value.ContentHash,
	}, nil
}


