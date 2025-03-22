/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var propertiesMap = map[string]interface{}{
	"example.str":      "string\n",
	"example.int":      123,
	"example.bool":     true,
	"example.float":    1.23,
	"example.double":   1.23,
	"example.duration": "300ms",
	"example.time":     "2022-10-22",
	"example.filemode": "-rwxrwxr-x",
}

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
example.filemode = -rwxrwxr-x
`

var propertiesFileYAML = `
example:
  str: "string\n"
  int: 123
  bool: true
  float: 1.23
  double: 1.23
  duration: 300ms
  time: "2022-10-22"
  filemode: -rwxrwxr-x
`

const expectedPropertiesNum = 8

type beanWithProperties struct {
	Str    string   `value:"example.str"`
	DefStr string   `value:"example.str.def,default=def"`
	ArrStr []string `value:"example.str.arr,default=a;b;c"`
	//StrFn func() (string, error) `value:"example.str"`

	Int    int   `value:"example.int"`
	DefInt int   `value:"example.int.def,default=555"`
	ArrInt []int `value:"example.int.arr,default=1;2;3"`
	//IntFn   func() (int, error) `value:"example.int"`

	Bool    bool   `value:"example.bool"`
	DefBool bool   `value:"example.bool.def,default=true"`
	ArrBool []bool `value:"example.bool.arr,default=true;false;true"`
	//BoolFn   func() (bool, error) `value:"example.int"`

	Float32    float32   `value:"example.float"`
	DefFloat32 float32   `value:"example.float.def,default=5.55"`
	ArrFloat32 []float32 `value:"example.float.arr,default=1.2;1.3"`
	//Float32Fn   func() (float32, error) `value:"example.float"`

	Float64    float64   `value:"example.double"`
	DefFloat64 float64   `value:"example.double.def,default=5.55"`
	ArrFloat64 []float64 `value:"example.double.arr,default=1.2;1.3"`
	//Float64Fn func() (float64, error) `value:"example.double"`

	Duration    time.Duration   `value:"example.duration"`
	DefDuration time.Duration   `value:"example.duration.def,default=500ms"`
	ArrDuration []time.Duration `value:"example.duration.arr,default=100ms;200ms"`
	//DurationFn func() (time.Duration, error) `value:"example.duration"`

	Time    time.Time   `value:"example.time,layout=2006-01-02"`
	DefTime time.Time   `value:"example.time.def,layout=2006-01-02,default=2022-10-21"`
	ArrTime []time.Time `value:"example.time.arr,layout=2006-01-02,default=2022-10-21;2022-10-22"`
	//TimeFn func() (time.Time, error) `value:"example.time,layout=2006-01-02"`s

	FileMode    os.FileMode   `value:"example.filemode"`
	DefFileMode os.FileMode   `value:"example.filemode.def,default=-rw-rw-r--"`
	ArrFileMode []os.FileMode `value:"example.filemode.arr,default=-rw-rw-r--;-rw-rw-rw-"`
	//FileModeFn func() (time.Time, error) `value:"example.filemode"`

	Properties glue.Properties `inject`
}

type oneFile struct {
	name    string
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
	name string
	size int
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

type onePropertyResolver struct {
	key   string
	value string
}

func (t onePropertyResolver) Priority() int {
	// very low priority
	return 0
}

func (t onePropertyResolver) GetProperty(key string) (value string, ok bool) {
	if t.key == key {
		return t.value, true
	}
	return "", false
}

func TestPropertyResolver(t *testing.T) {

	p := glue.NewProperties()
	err := p.Parse(propertiesFile)
	require.NoError(t, err)

	p.Register(&onePropertyResolver{key: "new.property", value: "new.value"})
	require.Equal(t, "new.value", p.GetString("new.property", ""))
}

func TestProperties(t *testing.T) {

	p := glue.NewProperties()
	err := p.Parse(propertiesFile)
	require.NoError(t, err)

	require.Equal(t, expectedPropertiesNum, p.Len())

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

	require.Equal(t, os.FileMode(0775), p.GetFileMode("example.filemode", os.FileMode(0775)))
	require.Equal(t, 0, len(p.GetComments("example.filemode")))

	/**
	Test defaults
	*/

	require.Equal(t, "def", p.GetString("example.str.def", "def"))
	require.Equal(t, 555, p.GetInt("example.int.def", 555))
	require.Equal(t, true, p.GetBool("example.bool.def", true))
	require.Equal(t, float32(1.23), p.GetFloat("example.float.def", 1.23))
	require.Equal(t, 1.23, p.GetDouble("example.double.def", 1.23))
	require.Equal(t, time.Duration(300000000), p.GetDuration("example.duration.def", time.Duration(300000000)))
	require.Equal(t, os.FileMode(0775), p.GetFileMode("example.filemode.def", os.FileMode(0775)))

	//println(p.Dump())

}

func TestPlaceholderMapProperties(t *testing.T) {

	b := new(beanWithProperties)

	ctx, err := glue.New(
		glue.PropertyMap(propertiesMap),
		b,
	)

	require.NoError(t, err)
	defer ctx.Close()

	verifyPropertyBean(t, b)
}

func TestPlaceholderProperties(t *testing.T) {

	validatePropertiesFile(t, "application.properties", propertiesFile)
	validatePropertiesFile(t, "application.yaml", propertiesFileYAML)

}

func validatePropertiesFile(t *testing.T, fileName string, fileContent string) {

	b := new(beanWithProperties)

	ctx, err := glue.New(
		glue.ResourceSource{
			Name:       "resources",
			AssetNames: []string{fileName},
			AssetFiles: oneFile{name: fileName, content: fileContent},
		},
		glue.PropertySource{Path: "resources:" + fileName},
		b,
	)

	require.NoError(t, err)
	defer ctx.Close()

	res, ok := ctx.Resource("resources:" + fileName)
	require.True(t, ok)

	file, err := res.Open()
	require.NoError(t, err)
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, fileContent, string(content))

	verifyPropertyBean(t, b)

	/**
	Should be the same object
	*/
	require.Equal(t, ctx.Properties(), b.Properties)

	/**
	Runtime injection test
	*/
	b2 := new(beanWithProperties)
	err = ctx.Inject(b2)
	require.NoError(t, err)

	verifyPropertyBean(t, b2)

}

func verifyPropertyBean(t *testing.T, b *beanWithProperties) {

	require.NotNil(t, b.Properties)
	require.Equal(t, expectedPropertiesNum, b.Properties.Len())

	/**
	Test injected properties
	*/

	require.Equal(t, "string\n", b.Str)
	require.Equal(t, 123, b.Int)
	require.Equal(t, true, b.Bool)
	require.Equal(t, float32(1.23), b.Float32)
	require.Equal(t, 1.23, b.Float64)
	require.Equal(t, time.Duration(300000000), b.Duration)

	tm22, err := time.Parse("2006-01-02", "2022-10-22")
	require.NoError(t, err)
	require.Equal(t, tm22, b.Time)

	require.Equal(t, os.FileMode(0775), b.FileMode)

	/**
	Test default properties
	*/
	require.Equal(t, "def", b.DefStr)
	require.Equal(t, 555, b.DefInt)
	require.Equal(t, true, b.DefBool)
	require.Equal(t, float32(5.55), b.DefFloat32)
	require.Equal(t, 5.55, b.DefFloat64)
	require.Equal(t, time.Duration(500000000), b.DefDuration)

	tm21, err := time.Parse("2006-01-02", "2022-10-21")
	require.NoError(t, err)
	require.Equal(t, tm21, b.DefTime)

	require.Equal(t, os.FileMode(0664), b.DefFileMode)

	/**
	Test array properties
	*/
	require.Equal(t, []string{"a", "b", "c"}, b.ArrStr)
	require.Equal(t, []int{1, 2, 3}, b.ArrInt)
	require.Equal(t, []bool{true, false, true}, b.ArrBool)
	require.Equal(t, []float32{1.2, 1.3}, b.ArrFloat32)
	require.Equal(t, []float64{1.2, 1.3}, b.ArrFloat64)
	require.Equal(t, []time.Duration{100000000, 200000000}, b.ArrDuration)
	require.Equal(t, []time.Time{tm21, tm22}, b.ArrTime)

	require.Equal(t, []os.FileMode{os.FileMode(0664), os.FileMode(0666)}, b.ArrFileMode)

}

func TestMergeProperties(t *testing.T) {
	parent := glue.NewProperties()
	parent.Set("parent", "parent")
	parent.Set("same", "parent")

	child := glue.NewProperties()
	child.Set("ctx", "ctx")
	child.Set("same", "ctx")
	child.Extend(parent)

	require.Equal(t, "parent", parent.GetString("parent", ""))
	require.Equal(t, "parent", parent.GetString("same", ""))
	require.Equal(t, 1, len(parent.PropertyResolvers()))

	require.Equal(t, "parent", child.GetString("parent", ""))
	require.Equal(t, "ctx", child.GetString("ctx", ""))
	require.Equal(t, "ctx", child.GetString("same", ""))
	require.Equal(t, 2, len(child.PropertyResolvers()))

	for _, r := range parent.PropertyResolvers() {
		require.Equal(t, parent, r)
	}

	for _, r := range child.PropertyResolvers() {
		if r == parent {
			require.Equal(t, 100, r.Priority())
		}
		if r == child {
			require.Equal(t, 101, r.Priority())
		}
	}

}

func TestParseFileMode(t *testing.T) {

	knownModes := map[string]os.FileMode{
		"-rwxrwxr-x": os.FileMode(0775),
		"-rw-rw-r--": os.FileMode(0664),
		"-rw-rw-rw-": os.FileMode(0666),
		"-rwxrwx---": os.FileMode(0770),
	}

	for expected, mode := range knownModes {

		str := mode.String()
		require.Equal(t, expected, str)

		//actual := glue.parseFileMode(str)
		//require.Equal(t, mode, actual, mode.String())

	}

}
