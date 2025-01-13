/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type itemType int

const (
	itemError itemType = iota
	itemEOF
	itemKey
	itemValue
	itemComment
)

const (
	eof = -1
	whitespace = " \f\t"
)

var unicodeLiteralMap = indexMap("0123456789abcdefABCDEF")

type item struct {
	typ itemType
	pos int
	val string
}

func (t item) String() string {
	switch {
	case t.typ == itemEOF:
		return "EOF"
	case t.typ == itemError:
		return t.val
	case len(t.val) > 10:
		return fmt.Sprintf("%.10q...", t.val)
	}
	return fmt.Sprintf("%q", t.val)
}

type stateFn func(*lexer) stateFn

type lexer struct {
	input   string
	state   stateFn
	pos     int
	start   int
	width   int
	runes   []rune
	items   []item
}

func (t *lexer) next() rune {
	if t.pos >= len(t.input) {
		t.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(t.input[t.pos:])
	t.width = w
	t.pos += t.width
	return r
}

func (t *lexer) peek() rune {
	r := t.next()
	t.backup()
	return r
}

func (t *lexer) backup() {
	t.pos -= t.width
}

func (t *lexer) emit(typ itemType) {
	i := item{typ, t.start, string(t.runes)}
	t.items = append(t.items, i)
	t.start = t.pos
	t.runes = t.runes[:0]
}

func (t *lexer) ignore() {
	t.start = t.pos
}

func (t *lexer) appendRune(r rune) {
	t.runes = append(t.runes, r)
}

func (t *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, t.next()) {
		return true
	}
	t.backup()
	return false
}

func (t *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, t.next()) {
	}
	t.backup()
}

func (t *lexer) errorf(format string, args ...interface{}) stateFn {
	i := item{itemError, t.start, fmt.Sprintf(format, args...)}
	t.items = append(t.items, i)
	return nil
}

func lex(input string) []item {
	l := &lexer{
		input: input,
		runes: make([]rune, 0, 32),
	}
	l.run()
	return l.items
}

func (t *lexer) run() {
	for t.state = lexBeforeKey(t); t.state != nil; {
		t.state = t.state(t)
	}
}

func lexBeforeKey(t *lexer) stateFn {
	switch r := t.next(); {
	case isEOF(r):
		t.emit(itemEOF)
		return nil

	case isEOL(r):
		t.ignore()
		return lexBeforeKey

	case isComment(r):
		return lexComment

	case isWhitespace(r):
		t.ignore()
		return lexBeforeKey

	default:
		t.backup()
		return lexKey
	}
}

func lexComment(t *lexer) stateFn {
	t.acceptRun(whitespace)
	t.ignore()
	for {
		switch r := t.next(); {
		case isEOF(r):
			t.ignore()
			t.emit(itemEOF)
			return nil
		case isEOL(r):
			t.emit(itemComment)
			return lexBeforeKey
		default:
			t.appendRune(r)
		}
	}
}

func lexKey(t *lexer) stateFn {
	var r rune

Loop:
	for {
		switch r = t.next(); {

		case isEscape(r):
			err := t.scanEscapeSequence()
			if err != nil {
				return t.errorf(err.Error())
			}

		case isEndOfKey(r):
			t.backup()
			break Loop

		case isEOF(r):
			break Loop

		default:
			t.appendRune(r)
		}
	}

	if len(t.runes) > 0 {
		t.emit(itemKey)
	}

	if isEOF(r) {
		t.emit(itemEOF)
		return nil
	}

	return lexBeforeValue
}

func lexBeforeValue(t *lexer) stateFn {
	t.acceptRun(whitespace)
	t.accept(":=")
	t.acceptRun(whitespace)
	t.ignore()
	return lexValue
}

func lexValue(t *lexer) stateFn {
	for {
		switch r := t.next(); {
		case isEscape(r):
			if isEOL(t.peek()) {
				t.next()
				t.acceptRun(whitespace)
			} else {
				err := t.scanEscapeSequence()
				if err != nil {
					return t.errorf(err.Error())
				}
			}

		case isEOL(r):
			t.emit(itemValue)
			t.ignore()
			return lexBeforeKey

		case isEOF(r):
			t.emit(itemValue)
			t.emit(itemEOF)
			return nil

		default:
			t.appendRune(r)
		}
	}
}

func (t *lexer) scanEscapeSequence() error {
	switch r := t.next(); {

	case isEscapedCharacter(r):
		t.appendRune(decodeEscapedCharacter(r))
		return nil

	case atUnicodeLiteral(r):
		return t.scanUnicodeLiteral()

	case isEOF(r):
		return fmt.Errorf("premature EOF")

	default:
		t.appendRune(r)
		return nil
	}
}

func (t *lexer) scanUnicodeLiteral() error {

	d := make([]rune, 4)
	for i := 0; i < 4; i++ {
		d[i] = t.next()
		if d[i] == eof || !unicodeLiteralMap[d[i]] {
			return fmt.Errorf("invalid unicode literal")
		}
	}

	r, err := strconv.ParseInt(string(d), 16, 0)
	if err != nil {
		return err
	}

	t.appendRune(rune(r))
	return nil
}

func decodeEscapedCharacter(r rune) rune {
	switch r {
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	default:
		return r
	}
}

func atUnicodeLiteral(r rune) bool {
	return r == 'u'
}

func isComment(r rune) bool {
	return r == '#' || r == '!'
}

func isEndOfKey(r rune) bool {
	return strings.ContainsRune(" \f\t\r\n:=", r)
}

func isEOF(r rune) bool {
	return r == eof
}

func isEOL(r rune) bool {
	return r == '\n' || r == '\r'
}

func isEscape(r rune) bool {
	return r == '\\'
}

func isEscapedCharacter(r rune) bool {
	return strings.ContainsRune(" :=fnrt", r)
}

func isWhitespace(r rune) bool {
	return strings.ContainsRune(whitespace, r)
}

func indexMap(s string) map[rune]bool {
	m := make(map[rune]bool)
	for _, r := range s {
		m[r] = true
	}
	return m
}
