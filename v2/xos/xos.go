// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package xos

import (
	"fmt"
	"io"
	"os"

	"github.com/ironcore-dev/ironcore-image/v2/xio"
)

func WriteFileReader(name string, rd io.Reader, perm os.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, rd)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

func WriteFileOpener(name string, op xio.Source, perm os.FileMode) error {
	rd, err := op.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rd.Close() }()
	return WriteFileReader(name, rd, perm)
}

func CopyRegularFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("copy source file to destination file: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("close destination file: %w", err)
	}
	return nil
}
