// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux

// Package ptracer holds the start command of CWS injector
package ptracer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"syscall"

	"github.com/DataDog/datadog-agent/pkg/security/proto/ebpfless"
	"github.com/DataDog/datadog-agent/pkg/util/native"
)

func handleExecveAt(tracer *Tracer, process *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs, disableStats bool) error {
	fd := tracer.ReadArgInt32(regs, 0)

	filename, err := tracer.ReadArgString(process.Pid, regs, 1)
	if err != nil {
		return err
	}

	if filename == "" { // in this case, dirfd defines directly the file's FD
		var exists bool
		if filename, exists = process.Res.Fd[fd]; !exists || filename == "" {
			return errors.New("can't find related file path")
		}
	} else {
		filename, err = getFullPathFromFd(process, filename, fd)
		if err != nil {
			return err
		}
	}

	args, err := tracer.ReadArgStringArray(process.Pid, regs, 2)
	if err != nil {
		return err
	}
	args, argsTruncated := truncateArgs(args)

	envs, err := tracer.ReadArgStringArray(process.Pid, regs, 3)
	if err != nil {
		return err
	}
	envs, envsTruncated := truncateEnvs(envs)

	msg.Type = ebpfless.SyscallTypeExec
	msg.Exec = &ebpfless.ExecSyscallMsg{
		File: ebpfless.OpenSyscallMsg{
			Filename: filename,
		},
		Args:          args,
		ArgsTruncated: argsTruncated,
		Envs:          envs,
		EnvsTruncated: envsTruncated,
		TTY:           getPidTTY(process.Pid),
	}
	err = fillFileMetadata(filename, &msg.Exec.File, disableStats)
	if err != nil {
		return err
	}
	return nil
}

func handleExecve(tracer *Tracer, process *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs, disableStats bool) error {
	filename, err := tracer.ReadArgString(process.Pid, regs, 0)
	if err != nil {
		return err
	}

	filename, err = getFullPathFromFilename(process, filename)
	if err != nil {
		return err
	}

	args, err := tracer.ReadArgStringArray(process.Pid, regs, 1)
	if err != nil {
		return err
	}
	args, argsTruncated := truncateArgs(args)

	envs, err := tracer.ReadArgStringArray(process.Pid, regs, 2)
	if err != nil {
		return err
	}
	envs, envsTruncated := truncateEnvs(envs)

	msg.Type = ebpfless.SyscallTypeExec
	msg.Exec = &ebpfless.ExecSyscallMsg{
		File: ebpfless.OpenSyscallMsg{
			Filename: filename,
		},
		Args:          args,
		ArgsTruncated: argsTruncated,
		Envs:          envs,
		EnvsTruncated: envsTruncated,
		TTY:           getPidTTY(process.Pid),
	}
	err = fillFileMetadata(filename, &msg.Exec.File, disableStats)
	if err != nil {
		return err
	}
	return nil
}

func handleChdir(tracer *Tracer, process *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	// using msg to temporary store arg0, as it will be erased by the return value on ARM64
	dirname, err := tracer.ReadArgString(process.Pid, regs, 0)
	if err != nil {
		return err
	}

	dirname, err = getFullPathFromFilename(process, dirname)
	if err != nil {
		process.Res.Cwd = ""
		return err
	}

	msg.Chdir = &ebpfless.ChdirSyscallFakeMsg{
		Path: dirname,
	}
	return nil
}

func handleFchdir(tracer *Tracer, process *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	fd := tracer.ReadArgInt32(regs, 0)
	dirname, ok := process.Res.Fd[fd]
	if !ok {
		process.Res.Cwd = ""
		return nil
	}

	// using msg to temporary store arg0, as it will be erased by the return value on ARM64
	msg.Chdir = &ebpfless.ChdirSyscallFakeMsg{
		Path: dirname,
	}
	return nil
}

func handleSetuid(tracer *Tracer, _ *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	msg.Type = ebpfless.SyscallTypeSetUID
	msg.SetUID = &ebpfless.SetUIDSyscallMsg{
		UID:  tracer.ReadArgInt32(regs, 0),
		EUID: -1,
	}
	return nil
}

func handleSetgid(tracer *Tracer, _ *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	msg.Type = ebpfless.SyscallTypeSetGID
	msg.SetGID = &ebpfless.SetGIDSyscallMsg{
		GID:  tracer.ReadArgInt32(regs, 0),
		EGID: -1,
	}
	return nil
}

func handleSetreuid(tracer *Tracer, _ *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	msg.Type = ebpfless.SyscallTypeSetUID
	msg.SetUID = &ebpfless.SetUIDSyscallMsg{
		UID:  tracer.ReadArgInt32(regs, 0),
		EUID: tracer.ReadArgInt32(regs, 1),
	}
	return nil
}

func handleSetregid(tracer *Tracer, _ *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	msg.Type = ebpfless.SyscallTypeSetGID
	msg.SetGID = &ebpfless.SetGIDSyscallMsg{
		GID:  tracer.ReadArgInt32(regs, 0),
		EGID: tracer.ReadArgInt32(regs, 1),
	}
	return nil
}

func handleSetfsuid(tracer *Tracer, _ *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	msg.Type = ebpfless.SyscallTypeSetFSUID
	msg.SetFSUID = &ebpfless.SetFSUIDSyscallMsg{
		FSUID: tracer.ReadArgInt32(regs, 0),
	}
	return nil
}

func handleSetfsgid(tracer *Tracer, _ *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	msg.Type = ebpfless.SyscallTypeSetFSGID
	msg.SetFSGID = &ebpfless.SetFSGIDSyscallMsg{
		FSGID: tracer.ReadArgInt32(regs, 0),
	}
	return nil
}

func handleCapset(tracer *Tracer, process *Process, msg *ebpfless.SyscallMsg, regs syscall.PtraceRegs) error {
	pCaps, err := tracer.ReadArgData(process.Pid, regs, 1, 24 /*sizeof uint32 x3 x2*/)
	if err != nil {
		return err
	}
	var (
		tmp       uint32
		effective uint64
		permitted uint64
	)

	// extract low bytes of effective caps
	buf := bytes.NewReader(pCaps[:4])
	err = binary.Read(buf, native.Endian, &tmp)
	if err != nil {
		return err
	}
	effective = uint64(tmp)
	// extract high bytes of effective caps
	buf = bytes.NewReader(pCaps[12:16])
	err = binary.Read(buf, native.Endian, &tmp)
	if err != nil {
		return err
	}
	// merge them together
	effective |= uint64(tmp) << 32

	// extract low bytes of permitted caps
	buf = bytes.NewReader(pCaps[4:8])
	err = binary.Read(buf, native.Endian, &tmp)
	if err != nil {
		return err
	}
	permitted = uint64(tmp)
	// extract high bytes of permitted caps
	buf = bytes.NewReader(pCaps[16:20])
	err = binary.Read(buf, native.Endian, &tmp)
	if err != nil {
		return err
	}
	// merge them together
	permitted |= uint64(tmp) << 32

	msg.Type = ebpfless.SyscallTypeCapset
	msg.Capset = &ebpfless.CapsetSyscallMsg{
		Effective: uint64(effective),
		Permitted: uint64(permitted),
	}
	return nil
}
