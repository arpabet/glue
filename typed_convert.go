//go:build go1.18

/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func convertPropertyValue[T any](s string) (T, error) {
	var zero T
	typ := beanType[T]()
	value, err := convertTypedString(strings.TrimSpace(s), typ)
	if err != nil {
		return zero, err
	}
	return value.Interface().(T), nil
}

func convertTypedString(s string, typ reflect.Type) (reflect.Value, error) {
	switch {
	case typ.Kind() == reflect.Slice:
		parts := typedTrimSplit(s, ";")
		slice := reflect.MakeSlice(typ, 0, len(parts))
		for _, part := range parts {
			val, err := convertTypedString(part, typ.Elem())
			if err != nil {
				return reflect.Zero(typ), err
			}
			slice = reflect.Append(slice, val)
		}
		return slice, nil
	case isTypedDuration(typ):
		dur, err := time.ParseDuration(s)
		if err != nil {
			return reflect.Zero(typ), err
		}
		return reflect.ValueOf(dur).Convert(typ), nil
	case isTypedTime(typ):
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return reflect.Zero(typ), err
		}
		return reflect.ValueOf(t).Convert(typ), nil
	case isTypedFileMode(typ):
		return reflect.ValueOf(typedParseFileMode(s)), nil
	case isTypedBool(typ):
		v, err := typedParseBool(s)
		if err != nil {
			return reflect.Zero(typ), err
		}
		return reflect.ValueOf(v).Convert(typ), nil
	case isTypedString(typ):
		return reflect.ValueOf(s).Convert(typ), nil
	case isTypedFloat(typ):
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return reflect.Zero(typ), err
		}
		return reflect.ValueOf(f).Convert(typ), nil
	case isTypedInt(typ):
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return reflect.Zero(typ), err
		}
		return reflect.ValueOf(i).Convert(typ), nil
	case isTypedUint(typ):
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return reflect.Zero(typ), err
		}
		return reflect.ValueOf(u).Convert(typ), nil
	default:
		return reflect.Zero(typ), errors.Errorf("unsupported property type %s", typ)
	}
}

func typedTrimSplit(s, sep string) []string {
	parts := strings.Split(s, sep)
	var out []string
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func typedParseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "on", "ON", "On":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False", "off", "OFF", "Off":
		return false, nil
	}
	return false, errors.Errorf("invalid syntax '%s'", str)
}

func typedParseFileMode(s string) os.FileMode {
	var m uint32
	const rwx = "rwxrwxrwx"
	off := len(s) - len(rwx)
	if off < 0 {
		buf := []byte("---------")
		copy(buf[-off:], s)
		s = string(buf)
	} else {
		s = s[off:]
	}
	for i, c := range rwx {
		if byte(c) == s[i] {
			m |= 1 << uint(9-1-i)
		}
	}
	return os.FileMode(m)
}

func isTypedDuration(t reflect.Type) bool {
	return t == durationClass
}

func isTypedTime(t reflect.Type) bool {
	return t == timeClass
}

func isTypedFileMode(t reflect.Type) bool {
	return t == osFileModeClass
}

func isTypedBool(t reflect.Type) bool {
	return t.Kind() == reflect.Bool
}

func isTypedString(t reflect.Type) bool {
	return t.Kind() == reflect.String
}

func isTypedFloat(t reflect.Type) bool {
	k := t.Kind()
	return k == reflect.Float32 || k == reflect.Float64
}

func isTypedInt(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

func isTypedUint(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}
