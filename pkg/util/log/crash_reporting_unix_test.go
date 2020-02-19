// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

// +build !windows

package log

import (
	"os"

	"golang.org/x/sys/unix"
)

func init() {
	safeErrorTestCases = append(safeErrorTestCases, safeErrorTestCase{
		format: "", rs: []interface{}{os.NewSyscallError("write", unix.ENOSPC)},
		expErr: "?:0: *os.SyscallError: write: syscall.Errno: no space left on device",
	})
}