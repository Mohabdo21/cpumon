package main

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	Row uint16
	Col uint16
	_   uint16
	_   uint16
}

func termWidth() int {
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(os.Stdout.Fd()),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 || ws.Col == 0 {
		return 80
	}
	return int(ws.Col)
}

func enableRawMode() (orig syscall.Termios, err error) {
	fd := uintptr(os.Stdin.Fd())
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&orig)),
	)
	if errno != 0 {
		return orig, errno
	}

	raw := orig
	raw.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&raw)),
	)
	if errno != 0 {
		return orig, errno
	}
	return orig, nil
}

func restoreTermMode(t syscall.Termios) {
	fd := uintptr(os.Stdin.Fd())
	_, _, _ = syscall.Syscall(
		syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&t)),
	)
}
