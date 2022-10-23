/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Properties contains the key/value pairs from the properties input.
type properties struct {

	sync.RWMutex

	store map[string]string
	comments map[string][]string

	// property conversion error handler
	errorHandler func(string, error)

}

func NewProperties() Properties {
	return &properties {
		store: make(map[string]string),
		comments: make(map[string][]string),
	}
}

func (t *properties) Load(reader io.Reader) error {
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	return t.Parse(string(content))
}

func (t *properties) Save(writer io.Writer) (n int, err error) {
	return writer.Write([]byte(t.Dump()))
}

func (t *properties) Parse(content string) error {
	var key string
	comments := make([]string, 0, 5)
	var inside bool

	t.Lock()
	defer t.Unlock()

	for _, item := range lex(content) {
		switch item.typ {
		case itemEOF:
			if inside {
				t.comments[key] = comments
				t.store[key] = ""
			}
			break
		case itemComment:
			if inside {
				return errors.Errorf("comment is not expected inside the property on key '%s'", key)
			}
			comments = append(comments, item.val)
		case itemKey:
			if inside {
				return errors.Errorf("key is not expected inside the property on key '%s'", key)
			}
			key = item.val
			inside = true
		case itemValue:
			if !inside {
				return errors.Errorf("value is not expected outside of the property after key '%s'", key)
			}
			t.store[key] = item.val
			if len(comments) > 0 {
				t.comments[key] = comments
				comments = make([]string, 0, 5)
			}
			inside = false
		case itemError:
			if inside {
				return errors.Errorf("property parsing error on key '%s', %s", key, item.val)
			} else {
				return errors.Errorf("property parsing error after key '%s', %s", key, item.val)
			}
		}
	}
	return nil
}

func (t *properties) Dump() string {
	var output strings.Builder

	keys := t.Keys()
	sort.Strings(keys)

	t.RLock()
	defer t.RUnlock()

	for _, key := range keys {

		if value, ok := t.store[key]; ok {

			for _, comment := range t.comments[key] {
				if len(comment) > 0 {
					output.WriteString("# ")
					output.WriteString(comment)
					output.WriteByte('\n')
				}
			}

			output.WriteString(fmt.Sprintf("%s = %s\n", encodeUtf8(key, " :"), encodeUtf8(value, "")))

		}

	}

	return output.String()
}

func (t *properties) Merge(other Properties) {
	m := other.Map()
	t.Lock()
	defer t.Unlock()
	for k, v := range m {
		t.store[k] = v
		comments := other.GetComments(k)
		if len(comments) > 0 {
			t.comments[k] = comments
		}
	}
}

func (t *properties) Len() int {
	t.RLock()
	defer t.RUnlock()
	return len(t.store)
}

func (t *properties) Keys() []string {
	t.RLock()
	defer t.RUnlock()
	keys := make([]string, 0, len(t.store))
	for k, _ := range t.store {
		keys = append(keys, k)
	}
	return keys
}

func (t *properties) Map() map[string]string {
	t.RLock()
	defer t.RUnlock()
	m := make(map[string]string)
	for k, v := range t.store {
		m[k] = v
	}
	return m
}

func (t *properties) Contains(key string) bool {
	t.RLock()
	defer t.RUnlock()
	_, ok := t.store[key]
	return ok
}

func (t *properties) Get(key string) (value string, ok bool) {
	t.RLock()
	defer t.RUnlock()
	value, ok = t.store[key]
	return
}

func (t *properties) GetString(key, def string) string {
	if value, ok := t.Get(key); ok {
		return value
	} else {
		return def
	}
}

func (t *properties) GetErrorHandler() func(string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.errorHandler
}

func (t *properties) SetErrorHandler(onError func(string, error)) {
	t.Lock()
	defer t.Unlock()
	t.errorHandler = onError
}

func (t *properties) GetBool(key string, def bool) bool {
	if value, ok := t.Get(key); ok {
		if v, err := parseBool(value); err != nil {
			cb := t.GetErrorHandler()
			if cb != nil {
				cb(key, err)
			}
			return def
		} else {
			return v
		}
	} else {
		return def
	}
}

func (t *properties) GetInt(key string, def int) int {
	if value, ok := t.Get(key); ok {
		if v, err := strconv.Atoi(value); err != nil {
			cb := t.GetErrorHandler()
			if cb != nil {
				cb(key, err)
			}
			return def
		} else {
			return v
		}
	} else {
		return def
	}
}

func (t *properties) GetFloat(key string, def float32) float32 {
	if value, ok := t.Get(key); ok {
		if f, err := strconv.ParseFloat(value, 32); err != nil {
			cb := t.GetErrorHandler()
			if cb != nil {
				cb(key, err)
			}
			return def
		} else {
			return float32(f)
		}
	} else {
		return def
	}
}

func (t *properties) GetDouble(key string, def float64) float64 {
	if value, ok := t.Get(key); ok {
		if f, err := strconv.ParseFloat(value, 64); err != nil {
			cb := t.GetErrorHandler()
			if cb != nil {
				cb(key, err)
			}
			return def
		} else {
			return f
		}
	} else {
		return def
	}
}

func (t *properties) GetDuration(key string, def time.Duration) time.Duration {
	if str, ok := t.Get(key); ok {
		if value, err := time.ParseDuration(str); err != nil {
			cb := t.GetErrorHandler()
			if cb != nil {
				cb(key, err)
			}
			return def
		} else {
			return value
		}
	} else {
		return def
	}
}

func (t *properties) Set(key string, value string) {
	t.Lock()
	defer t.Unlock()
	t.store[key] = value
}

func (t *properties) Remove(key string) bool {
	t.Lock()
	defer t.Unlock()
	_, ok := t.store[key]
	if !ok {
		return false
	}
	delete(t.store, key)
	delete(t.comments, key)
	return true
}

func (t *properties) Clear() {
	t.Lock()
	defer t.Unlock()
	t.store = make(map[string]string)
	t.comments = make(map[string][]string)
}

func (t *properties) GetComments(key string) []string {
	t.RLock()
	defer t.RUnlock()
	return t.comments[key]
}

func (t *properties) SetComments(key string, comments []string) {
	t.Lock()
	defer t.Unlock()
	t.comments[key] = comments
}

func (t *properties) ClearComments() {
	t.Lock()
	defer t.Unlock()
	t.comments = make(map[string][]string)
}

func encodeUtf8(s string, special string) string {
	v := ""
	for pos := 0; pos < len(s); {
		r, w := utf8.DecodeRuneInString(s[pos:])
		pos += w
		v += escape(r, special)
	}
	return v
}

func escape(r rune, special string) string {
	switch r {
	case '\f':
		return "\\f"
	case '\n':
		return "\\n"
	case '\r':
		return "\\r"
	case '\t':
		return "\\t"
	case '\\':
		return "\\\\"
	default:
		if strings.ContainsRune(special, r) {
			return "\\" + string(r)
		}
		return string(r)
	}
}

func parseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "on", "ON", "On":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False", "off", "OFF", "Off":
		return false, nil
	}
	return false, errors.Errorf("invalid syntax '%s'", str)
}