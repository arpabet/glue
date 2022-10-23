/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue_test

import (
	"bytes"
	"fmt"
	"github.com/schwid/glue"
	"github.com/stretchr/testify/require"
	"io/fs"
	"io/ioutil"
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
example.duration = 300ms
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

func TestProperties(t *testing.T) {

	p := glue.NewProperties()
	err := p.Parse(propertiesFile)
	require.NoError(t, err)

	require.Equal(t, 6, p.Len())

	require.Equal(t, "string\n", p.GetString("example.str", ""))
	require.Equal(t, 2, len(p.GetComments("example.str")))

	require.Equal(t, 123, p.GetInt("example.int", 0))
	require.Equal(t, 0, len(p.GetComments("example.int")))

	require.Equal(t, true, p.GetBool("example.bool", false))
	require.Equal(t, 0, len(p.GetComments("example.bool")))

	require.Equal(t, float32(1.23), p.GetFloat("example.float", 0.0))
	require.Equal(t, 0, len(p.GetComments("example.float")))

	require.Equal(t, 1.23, p.GetDouble("example.double", 0.0))
	require.Equal(t, 0, len(p.GetComments("example.double")))

	require.Equal(t, time.Duration(300000000), p.GetDuration("example.duration", 0.0))
	require.Equal(t, 0, len(p.GetComments("example.double")))

	//println(p.Dump())

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

	res, ok := ctx.Resource("resources:application.properties")
	require.True(t, ok)

	file, err := res.Open()
	require.NoError(t, err)
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, propertiesFile, string(content))

	fmt.Printf("content = %s\n", string(content))

}