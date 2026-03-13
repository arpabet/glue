/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import "log"

/*
*
Verbose logs if not nil
*/
var verbose *log.Logger

/**
Use this function operate verbose and logging level during container creation.
*/

func Verbose(log *log.Logger) (prev *log.Logger) {
	prev, verbose = verbose, log
	return
}

type nullLogger struct {
}

func (n nullLogger) Enabled() bool { return false }

func (n nullLogger) Printf(format string, v ...any) {}

func (n nullLogger) Println(v ...any) {}

type logAdapter struct {
	log *log.Logger
}

func (l logAdapter) Enabled() bool {
	return true
}

func (l logAdapter) Printf(format string, v ...any) {
	l.log.Printf(format, v...)
}

func (l logAdapter) Println(v ...any) {
	l.log.Println(v...)
}
