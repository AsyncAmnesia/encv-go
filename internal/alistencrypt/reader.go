package alistencrypt

import (
	"bytes"
	"fmt"
	"io"
)

type DecryptReader struct {
	reader    io.Reader
	cipher    Cipher
	pos       int64
	v2Header  *ContentHeader
	headerSkipped bool
}

func NewDecryptReader(r io.Reader, password string, fileSize int64) (*DecryptReader, error) {
	peekBuf := make([]byte, contentHeaderSize)
	n, err := io.ReadFull(r, peekBuf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	var header *ContentHeader
	var cipher Cipher
	var source io.Reader

	if n >= contentHeaderSize && IsV2Format(peekBuf) {
		header, err = DetectContentHeader(peekBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to parse V2 header: %w", err)
		}
		cipher, err = Create(password, "aesctr", header.PlainSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create cipher: %w", err)
		}
		source = r
	} else {
		cipher, err = Create(password, "aesctr", fileSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create cipher: %w", err)
		}
		source = io.MultiReader(bytes.NewReader(peekBuf[:n]), r)
	}

	return &DecryptReader{
		reader:    source,
		cipher:    cipher,
		pos:       0,
		v2Header:  header,
		headerSkipped: header == nil,
	}, nil
}

func (dr *DecryptReader) Read(p []byte) (int, error) {
	if dr.v2Header != nil && !dr.headerSkipped {
		dr.headerSkipped = true
	}

	n, err := dr.reader.Read(p)
	if n > 0 {
		dr.cipher.Decrypt(p[:n])
		dr.pos += int64(n)
	}
	return n, err
}

func (dr *DecryptReader) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = dr.pos + offset
	case io.SeekEnd:
		if dr.v2Header != nil {
			newPos = dr.v2Header.PlainSize + offset
		} else {
			return 0, fmt.Errorf("seek from end not supported for non-V2 format")
		}
	default:
		return 0, fmt.Errorf("invalid whence value")
	}

	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}

	if err := dr.cipher.SetPosition(newPos); err != nil {
		return 0, err
	}
	dr.pos = newPos
	return newPos, nil
}

func (dr *DecryptReader) Position() int64 {
	return dr.pos
}
