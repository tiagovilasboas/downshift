// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// maxBinaryBytes caps the extracted binary so a hostile archive cannot fill
// the disk. Release binaries are around 20 MB.
const maxBinaryBytes = 256 << 20

// ExtractBinary returns the named executable from a .tar.gz or .zip archive.
// Only an entry whose base name equals name counts; directories and links are
// ignored.
func ExtractBinary(archive []byte, archiveName, name string) ([]byte, error) {
	if strings.HasSuffix(archiveName, ".zip") {
		return fromZip(archive, name)
	}
	return fromTarGz(archive, name)
}

func fromTarGz(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("archive is not gzip: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("archive has no %s", name)
		}
		if err != nil {
			return nil, fmt.Errorf("reading archive: %w", err)
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == name {
			return readCapped(tr)
		}
	}
}

func fromZip(archive []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("archive is not zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Mode().IsRegular() && path.Base(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return readCapped(rc)
		}
	}
	return nil, fmt.Errorf("archive has no %s", name)
}

func readCapped(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBinaryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBinaryBytes {
		return nil, fmt.Errorf("binary in archive is larger than %d MB", maxBinaryBytes>>20)
	}
	return data, nil
}
