// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux && amd64

package ptracer

import (
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	OpenNr           = unix.SYS_OPEN              // OpenNr defines the syscall ID for amd64
	OpenatNr         = unix.SYS_OPENAT            // OpenatNr defines the syscall ID for amd64
	Openat2Nr        = unix.SYS_OPENAT2           // Openat2Nr defines the syscall ID for amd64
	CreatNr          = unix.SYS_CREAT             // CreatNr defines the syscall ID for amd64
	NameToHandleAtNr = unix.SYS_NAME_TO_HANDLE_AT // NameToHandleAtNr defines the syscall ID for amd64
	OpenByHandleAtNr = unix.SYS_OPEN_BY_HANDLE_AT // OpenByHandleAtNr defines the syscall ID for amd64
	ExecveNr         = unix.SYS_EXECVE            // ExecveNr defines the syscall ID for amd64
	ExecveatNr       = unix.SYS_EXECVEAT          // ExecveatNr defines the syscall ID for amd64
	CloneNr          = unix.SYS_CLONE             // CloneNr defines the syscall ID for amd64
	ForkNr           = unix.SYS_FORK              // ForkNr defines the syscall ID for amd64
	VforkNr          = unix.SYS_VFORK             // VforkNr defines the syscall ID for amd64
	ExitNr           = unix.SYS_EXIT              // ExitNr defines the syscall ID for amd64
	FcntlNr          = unix.SYS_FCNTL             // FcntlNr defines the syscall ID for amd64
	DupNr            = unix.SYS_DUP               // DupNr defines the syscall ID for amd64
	Dup2Nr           = unix.SYS_DUP2              // Dup2Nr defines the syscall ID for amd64
	Dup3Nr           = unix.SYS_DUP3              // Dup3Nr defines the syscall ID for amd64
	ChdirNr          = unix.SYS_CHDIR             // ChdirNr defines the syscall ID for amd64
	FchdirNr         = unix.SYS_FCHDIR            // FchdirNr defines the syscall ID for amd64
	SetuidNr         = unix.SYS_SETUID            // SetuidNr defines the syscall ID for amd64
	SetgidNr         = unix.SYS_SETGID            // SetgidNr defines the syscall ID for amd64
	SetreuidNr       = unix.SYS_SETREUID          // SetreuidNr defines the syscall ID for amd64
	SetregidNr       = unix.SYS_SETREGID          // SetregidNr defines the syscall ID for amd64
	SetresuidNr      = unix.SYS_SETRESUID         // SetresuidNr defines the syscall ID for amd64
	SetresgidNr      = unix.SYS_SETRESGID         // SetresgidNr defines the syscall ID for amd64
	SetfsuidNr       = unix.SYS_SETFSUID          // SetfsuidNr defines the syscall ID for amd64
	SetfsgidNr       = unix.SYS_SETFSGID          // SetfsgidNr defines the syscall ID for amd64
	CloseNr          = unix.SYS_CLOSE             // CloseNr defines the syscall ID for amd64
	MemfdCreateNr    = unix.SYS_MEMFD_CREATE      // MemfdCreateNr defines the syscall ID for amd64
	CapsetNr         = unix.SYS_CAPSET            // CapsetNr defines the syscall ID for amd64
	UnlinkNr         = unix.SYS_UNLINK            // UnlinkNr defines the syscall ID for amd64
	UnlinkatNr       = unix.SYS_UNLINKAT          // UnlinkatNr defines the syscall ID for amd64
	RmdirNr          = unix.SYS_RMDIR             // RmdirNr defines the syscall ID for amd64
	RenameNr         = unix.SYS_RENAME            // RenameNr defines the syscall ID for amd64
	RenameAtNr       = unix.SYS_RENAMEAT          // RenameAtNr defines the syscall ID for amd64
	RenameAt2Nr      = unix.SYS_RENAMEAT2         // RenameAt2Nr defines the syscall ID for amd64

	ptraceFlags = 0 |
		syscall.PTRACE_O_TRACEVFORK |
		syscall.PTRACE_O_TRACEFORK |
		syscall.PTRACE_O_TRACECLONE |
		syscall.PTRACE_O_TRACEEXEC |
		syscall.PTRACE_O_TRACESYSGOOD |
		unix.PTRACE_O_TRACESECCOMP
)

// https://github.com/torvalds/linux/blob/v5.0/arch/x86/entry/entry_64.S#L126
func (t *Tracer) argToRegValue(regs syscall.PtraceRegs, arg int) uint64 {
	switch arg {
	case 0:
		return regs.Rdi
	case 1:
		return regs.Rsi
	case 2:
		return regs.Rdx
	case 3:
		return regs.R10
	case 4:
		return regs.R8
	case 5:
		return regs.R9
	}

	return 0
}

// ReadRet reads and returns the return value
func (t *Tracer) ReadRet(regs syscall.PtraceRegs) int64 {
	return int64(regs.Rax)
}

// GetSyscallNr returns the given syscall number
func GetSyscallNr(regs syscall.PtraceRegs) int {
	return int(regs.Orig_rax)
}
