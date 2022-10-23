/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue_test

import (
	"bytes"
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
example.time = 2022-10-22
`

var propertiesFileYAML = `
example:
	str: "string\n"
    int: 123
    bool: true
    float: 1.23
    double: 1.23
    duration: 300ms
    time: 2022-10-22
`

type beanWithProperties struct {

	Str  string `value:"example.str"`
	DefStr  string `value:"example.str.def,default=def"`

	Int  int `value:"example.int"`
	DefInt  int `value:"example.int.def,default=555"`

	Bool bool `value:"example.bool"`
	DefBool  bool `value:"example.bool.def,default=true"`

	Float32 float32 `value:"example.float"`
	DefFloat32 float32 `value:"example.float.def,default=5.55"`

	Float64 float64 `value:"example.double"`
	DefFloat64 float64 `value:"example.double.def,default=5.55"`

	Duration time.Duration `value:"example.duration"`
	DefDuration time.Duration `value:"example.duration.def,default=500ms"`

	Time time.Time  `value:"example.time,layout=2006-01-02"`
	DefTime time.Time  `value:"example.time.def,layout=2006-01-02,default=2022-10-21"`

	Properties  glue.Properties `inject`

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

	require.Equal(t, 7, p.Len())

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

	b := new(beanWithProperties)

	ctx, err := glue.New(
		glue.Verbose{ Log: log.Default() },
		glue.ResourceSource{
			Name: "resources",
			AssetNames: []string{ "application.properties" },
			AssetFiles: oneFile{ name: "application.properties", content: propertiesFile },
		},
		glue.PropertySource{Path: "resources:application.properties"},
		b,
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

	require.NotNil(t, b.Properties)
	require.Equal(t, 7, b.Properties.Len())

	/**
	Test injected properties
	 */

	require.Equal(t, "string\n", b.Str)
	require.Equal(t, 123, b.Int)
	require.Equal(t, true, b.Bool)
	require.Equal(t, float32(1.23), b.Float32)
	require.Equal(t, 1.23, b.Float64)
	require.Equal(t, time.Duration(300000000), b.Duration)

	tm, err := time.Parse( "2006-01-02", "2022-10-22")
	require.NoError(t, err)
	require.Equal(t, tm, b.Time)

	/**
	Test default properties
	 */
	require.Equal(t, "def", b.DefStr)
	require.Equal(t, 555, b.DefInt)
	require.Equal(t, true, b.DefBool)
	require.Equal(t, float32(5.55), b.DefFloat32)
	require.Equal(t, 5.55, b.DefFloat64)
	require.Equal(t, time.Duration(500000000), b.DefDuration)

	tm, err = time.Parse( "2006-01-02", "2022-10-21")
	require.NoError(t, err)
	require.Equal(t, tm, b.DefTime)

	/**
	Should be the same object
	 */
	require.Equal(t, ctx.Properties(), b.Properties)

}