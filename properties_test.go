/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue_test

import (
	"bytes"
	"github.com/schwid/glue"
	"github.com/stretchr/testify/require"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var propertiesFile = `
# comment
! also comment
example.str = str\
ing\n
example.int = 123
example.bool = true
example.float = 1.23
example.double = 1.23
`

type beanWithProperties struct {

	Str  string `value:"example.str"`
	Int  int `value:"example.int"`
	Bool bool `value:"example.bool"`
	Float32 float32 `value:"example.float"`
	Float64 float64 `value:"example.double"`

}

type oneFile struct {
	name string
	content string
}

func (t oneFile) Open(name string) (http.File, error) {
	if t.name != name {
		return nil, os.ErrNotExist
	}
	return assetFile{name: name, Reader: bytes.NewReader([]byte(t.content)), size: len(t.content)}, nil
}

type assetFile struct {
	*bytes.Reader
	name            string
	size            int
}

func (t assetFile) Close() error {
	return nil
}

func (t assetFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (t assetFile) Stat() (fs.FileInfo, error) {
	return t, nil
}

func (t assetFile) Name() string {
	return filepath.Base(t.name)
}

func (t assetFile) Size() int64 {
	return int64(t.size)
}

func (t assetFile) Mode() os.FileMode {
	return os.FileMode(0664)
}

func (t assetFile) ModTime() time.Time {
	return time.Now()
}

func (t assetFile) IsDir() bool {
	return false
}

func (t assetFile) Sys() interface{} {
	return t
}


func TestPlaceholderProperties(t *testing.T) {

	ctx, err := glue.New(
		glue.Verbose{ Log: log.Default() },
		glue.ResourceSource{
			Name: "resources",
			AssetNames: []string{ "application.properties" },
			AssetFiles: oneFile{ name: "application.properties", content: propertiesFile },
		},
	)

	require.NoError(t, err)
	defer ctx.Close()

}