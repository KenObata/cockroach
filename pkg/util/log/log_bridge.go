// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package log

import (
	"bytes"
	"context"
	"fmt"
	stdLog "log"
	"regexp"
	"strconv"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/util/log/logpb"
)

// NewStdLogger creates a *stdLog.Logger that forwards messages to the
// CockroachDB logs with the specified severity.
//
// The prefix should be the path of the package for which this logger
// is used. The prefix will be concatenated directly with the name
// of the file that triggered the logging.
func NewStdLogger(severity Severity, prefix string) *stdLog.Logger {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return stdLog.New(logBridge(severity), prefix, stdLog.Lshortfile)
}

// logBridge provides the Write method that enables copyStandardLogTo to connect
// Go's standard logs to the logs provided by this package.
type logBridge Severity

// copyStandardLogTo arranges for messages written to the Go "log"
// package's default logs to also appear in the CockroachDB logs with
// the specified severity.  Subsequent changes to the standard log's
// default output location or format may break this behavior.
//
// Valid names are "INFO", "WARNING", "ERROR", and "FATAL".  If the name is not
// recognized, copyStandardLogTo panics.
func copyStandardLogTo(severityName string) {
	sev, ok := logpb.SeverityByName(severityName)
	if !ok {
		panic(fmt.Sprintf("copyStandardLogTo(%q): unrecognized Severity name", severityName))
	}
	// Set a log format that captures the user's file and line:
	//   d.go:23: message
	stdLog.SetFlags(stdLog.Lshortfile)
	stdLog.SetOutput(logBridge(sev))
}

func init() {
	copyStandardLogTo("INFO")
}

var ignoredLogMessagesRe = regexp.MustCompile(
	// The HTTP package complains when a client opens a TCP connection
	// and immediately closes it. We don't care.
	`^net/http.*:\d+\: http: TLS handshake error from .*: EOF\s*$`,
)

// Write parses the standard logging line and passes its components to the
// logger for Severity(lb).
func (lb logBridge) Write(b []byte) (n int, err error) {
	if ignoredLogMessagesRe.Match(b) {
		return len(b), nil
	}

	entry := MakeEntry(context.Background(),
		Severity(lb), 0, /* depth */
		// Note: because the caller is using the stdLog interface, they are
		// bypassing all the log marker logic. This means that the entire
		// log message should be assumed to contain confidential
		// information—it is thus not redactable.
		false /* redactable */, "")

	// Split "d.go:23: message" into "d.go", "23", and "message".
	if parts := bytes.SplitN(b, []byte{':'}, 3); len(parts) != 3 || len(parts[0]) < 1 || len(parts[2]) < 1 {
		entry.Message = fmt.Sprintf("bad log format: %s", b)
	} else {
		// We use a "(gostd)" prefix so that these log lines correctly point
		// to the go standard library instead of our own source directory.
		entry.File = "(gostd) " + string(parts[0])
		entry.Message = string(parts[2][1 : len(parts[2])-1]) // skip leading space and trailing newline
		entry.Line, err = strconv.ParseInt(string(parts[1]), 10, 64)
		if err != nil {
			entry.Message = fmt.Sprintf("bad line number: %s", b)
			entry.Line = 1
		}
	}
	debugLog.outputLogEntry(entry)
	return len(b), nil
}
