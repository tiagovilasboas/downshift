// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// ttyColumns is the column count of the terminal behind f, or 0 when f is not
// a terminal.
func ttyColumns(f *os.File) int {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0
	}
	return int(ws.Col)
}
