package vaultblob

import (
	"errors"
	"io"
	"unsafe"
)

type Options struct {
	MaxChunkSize uint64
	MaxBlobSize  uint64
	SplitFiles   bool
	StripeChunks bool
	Verbose      bool
}

type Session struct {
	handle unsafe.Pointer
}

func OpenVault(dir string, vaultID [16]byte, masterKey [32]byte, opts *Options) (*Session, error) {
	if err := ensureLib(); err != nil {
		return nil, err
	}

	maxChunk := uint64(4 * 1024 * 1024)
	maxBlob := uint64(1024 * 1024 * 1024)
	split := int32(0)
	stripe := int32(0)
	verbose := int32(0)

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
		if opts.Verbose {
			verbose = 1
		}
	}

	var errPtr *byte
	handle := vaultOpenVault(cstr(dir), &vaultID[0], &masterKey[0], maxChunk, maxBlob, split, stripe, verbose, &errPtr)
	if handle == 0 {
		msg := cstrFromPtr(errPtr)
		vaultFreeString(errPtr)
		return nil, errors.New(msg)
	}

	return &Session{handle: unsafe.Pointer(handle)}, nil
}

func (s *Session) Close() {
	if s.handle != nil {
		vaultClose(uintptr(s.handle))
		s.handle = nil
	}
}

func (s *Session) PutFile(data []byte) (string, error) {
	return s.PutFileWithID(data, "")
}

func (s *Session) PutFileWithID(data []byte, fileID string) (string, error) {
	if s.handle == nil {
		return "", errors.New("vaultblob: session closed")
	}

	var fileIDPtr *byte
	if fileID != "" {
		fileIDPtr = cstr(fileID)
	}

	var outPtr *byte
	var errPtr *byte

	result := vaultPutFile(
		uintptr(s.handle),
		&data[0],
		len(data),
		fileIDPtr,
		&outPtr,
		&errPtr,
	)

	if result != 0 {
		msg := cstrFromPtr(errPtr)
		vaultFreeString(errPtr)
		return "", errors.New(msg)
	}

	id := cstrFromPtr(outPtr)
	vaultFreeString(outPtr)
	return id, nil
}

func (s *Session) ReadFile(fileID string) ([]byte, error) {
	if s.handle == nil {
		return nil, errors.New("vaultblob: session closed")
	}

	var outLen int
	var errPtr *byte

	p := vaultReadFile(uintptr(s.handle), cstr(fileID), &outLen, &errPtr)
	if p == 0 {
		msg := cstrFromPtr(errPtr)
		vaultFreeString(errPtr)
		return nil, errors.New(msg)
	}

	data := make([]byte, outLen)
	slice := unsafe.Slice((*byte)(unsafe.Pointer(p)), outLen)
	copy(data, slice)
	vaultFreeBytes((*byte)(unsafe.Pointer(p)), outLen)
	return data, nil
}

func (s *Session) ReadFileRange(fileID string, offset, length uint64) ([]byte, error) {
	if s.handle == nil {
		return nil, errors.New("vaultblob: session closed")
	}

	var outData *byte
	var outLen int
	var errPtr *byte

	result := vaultReadFileRange(uintptr(s.handle), cstr(fileID), offset, length, &outData, &outLen, &errPtr)
	if result != 0 {
		msg := cstrFromPtr(errPtr)
		vaultFreeString(errPtr)
		return nil, errors.New(msg)
	}

	if outLen == 0 {
		return nil, nil
	}

	data := make([]byte, outLen)
	slice := unsafe.Slice(outData, outLen)
	copy(data, slice)
	vaultFreeBytes(outData, outLen)
	return data, nil
}

type FileReader struct {
	s        *Session
	fileID   string
	offset   uint64
	fileSize uint64
}

func (r *FileReader) Read(p []byte) (int, error) {
	if r.s == nil || r.s.handle == nil {
		return 0, errors.New("vaultblob: session closed")
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

	data, err := r.s.ReadFileRange(r.fileID, r.offset, length)
	if err != nil {
		return 0, err
	}

	n := copy(p, data)
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
	if s.handle == nil {
		return 0, errors.New("vaultblob: session closed")
	}

	var size uint64
	var errPtr *byte

	result := vaultFileSize(uintptr(s.handle), cstr(fileID), &size, &errPtr)
	if result != 0 {
		msg := cstrFromPtr(errPtr)
		vaultFreeString(errPtr)
		return 0, errors.New(msg)
	}

	return size, nil
}

func (s *Session) BlobIDs() ([]string, error) {
	if s.handle == nil {
		return nil, errors.New("vaultblob: session closed")
	}

	var outPtr **byte
	var outCount int
	var errPtr *byte

	result := vaultBlobIDs(uintptr(s.handle), &outPtr, &outCount, &errPtr)
	if result != 0 {
		msg := cstrFromPtr(errPtr)
		vaultFreeString(errPtr)
		return nil, errors.New(msg)
	}

	ids := make([]string, outCount)
	for i := 0; i < outCount; i++ {
		p := *(**byte)(unsafe.Add(unsafe.Pointer(outPtr), uintptr(i)*unsafe.Sizeof(outPtr)))
		ids[i] = cstrFromPtr(p)
	}

	vaultFreeStringArray(outPtr, outCount)
	return ids, nil
}
