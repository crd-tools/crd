package registry

import (
	"reflect"
	"testing"
	"testing/fstest"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func ptr[T any](v T) *T { return &v }

func mustSchema(t *testing.T, s apiextv1.JSONSchemaProps) []byte {
	t.Helper()
	data, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type sample struct{}

func TestFormat(t *testing.T) {
	Convey("DER round trip", t, func() {
		f := File{
			Package: "example.com/a/v1",
			Module:  "example.com/a",
			Version: "v1.2.3",
			Types:   []Type{{Name: "T", Refs: []string{"x.Y"}, Schema: []byte{1, 2, 3}}},
		}
		data, err := MarshalFile(f)
		So(err, ShouldBeNil)
		got, err := UnmarshalFile(data)
		So(err, ShouldBeNil)
		f.Format = FormatVersion
		So(got, ShouldResemble, f)

		_, err = UnmarshalFile(append(data, 0))
		So(err, ShouldNotBeNil)
	})

	Convey("File names and keys", t, func() {
		So(FileName("k8s.io/api/core/v1"), ShouldEqual, "k8s.io~api~core~v1.der")
		So(FileName("a/b~c"), ShouldNotEqual, FileName("a/b/c"))

		pkg, name, err := SplitKey("example.com/a.b/v1.Type")
		So(err, ShouldBeNil)
		So(pkg, ShouldEqual, "example.com/a.b/v1")
		So(name, ShouldEqual, "Type")
		_, _, err = SplitKey("example.com/a.b/v1")
		So(err, ShouldNotBeNil)

		key, ok := KeyOf(reflect.TypeFor[*sample]())
		So(ok, ShouldBeTrue)
		So(key, ShouldEqual, "crd.tools/crd/registry.sample")
		_, ok = KeyOf(reflect.TypeFor[int]())
		So(ok, ShouldBeFalse)
	})
}

func TestResolve(t *testing.T) {
	Convey("Resolve inlines references with field overlays", t, func() {
		list := apiextv1.JSONSchemaProps{
			Type:        "object",
			Description: "type doc",
			Properties: map[string]apiextv1.JSONSchemaProps{
				"cpu": {Type: "integer", Maximum: ptr(10.0)},
			},
		}
		root := apiextv1.JSONSchemaProps{
			Type: "object",
			AllOf: []apiextv1.JSONSchemaProps{
				{Ref: ptr("p.Base")},
			},
			Properties: map[string]apiextv1.JSONSchemaProps{
				"limits": {Ref: ptr("p.List"), Description: "field doc", MaxProperties: ptr(int64(3))},
			},
		}
		base := apiextv1.JSONSchemaProps{
			Type:       "object",
			Required:   []string{"owner"},
			Properties: map[string]apiextv1.JSONSchemaProps{"owner": {Type: "string"}},
		}

		r := New()
		err := r.Add(Index{}, File{Package: "p", Types: []Type{
			{Name: "Root", Schema: mustSchema(t, root)},
			{Name: "List", Schema: mustSchema(t, list)},
			{Name: "Base", Schema: mustSchema(t, base)},
		}})
		So(err, ShouldBeNil)

		s, err := r.Resolve("p.Root")
		So(err, ShouldBeNil)
		So(s.AllOf, ShouldBeEmpty)
		So(s.Required, ShouldResemble, []string{"owner"})
		So(s.Properties, ShouldContainKey, "owner")
		limits := s.Properties["limits"]
		So(limits.Ref, ShouldBeNil)
		So(limits.Description, ShouldEqual, "field doc")
		So(*limits.MaxProperties, ShouldEqual, 3)
		So(*limits.Properties["cpu"].Maximum, ShouldEqual, 10)

		Convey("conflicting constraints are kept in allOf", func() {
			dst := apiextv1.JSONSchemaProps{Maximum: ptr(10.0)}
			So(mergeInto(&dst, apiextv1.JSONSchemaProps{Maximum: ptr(5.0)}), ShouldBeNil)
			So(dst.Maximum, ShouldBeNil)
			So(dst.AllOf, ShouldHaveLength, 2)
		})

		Convey("conflicting types are an error", func() {
			dst := apiextv1.JSONSchemaProps{Type: "string"}
			So(mergeInto(&dst, apiextv1.JSONSchemaProps{Type: "integer"}), ShouldNotBeNil)
		})

		Convey("recursive references are an error", func() {
			loop := New()
			So(loop.Add(Index{}, File{Package: "p", Types: []Type{
				{Name: "A", Schema: mustSchema(t, apiextv1.JSONSchemaProps{Ref: ptr("p.A")})},
			}}), ShouldBeNil)
			_, err := loop.Resolve("p.A")
			So(err, ShouldNotBeNil)
		})

		Convey("same key with a different schema conflicts", func() {
			err := r.Add(Index{}, File{Package: "p", Types: []Type{
				{Name: "Base", Schema: mustSchema(t, list)},
			}})
			So(err, ShouldNotBeNil)
		})

		Convey("missing type returns ErrNotFound", func() {
			_, err := r.Resolve("p.Nope")
			So(err, ShouldWrap, ErrNotFound)
		})
	})
}

func TestLazyFS(t *testing.T) {
	Convey("AddFS reads only the index", t, func() {
		good := mustSchema(t, apiextv1.JSONSchemaProps{Type: "string"})
		goodFile, err := MarshalFile(File{Package: "p", Types: []Type{{Name: "Good", Schema: good}}})
		So(err, ShouldBeNil)
		idx, err := MarshalIndex(Index{
			Files: []string{"p.der", "q.der"},
			Types: []Entry{
				{Type: "p.Good", File: "p.der", Hash: Hash(good)},
				{Type: "q.Bad", File: "q.der", Hash: Hash([]byte{1})},
			},
		})
		So(err, ShouldBeNil)
		fsys := fstest.MapFS{
			IndexFile: {Data: idx},
			"p.der":   {Data: goodFile},
			"q.der":   {Data: []byte("broken")},
		}

		r := New()
		So(r.AddFS(fsys), ShouldBeNil)
		So(r.Keys(), ShouldResemble, []string{"p.Good", "q.Bad"})

		s, err := r.Get("p.Good")
		So(err, ShouldBeNil)
		So(s.Type, ShouldEqual, "string")
		So(r.entries["p.Good"].raw, ShouldBeNil)

		_, err = r.Get("q.Bad")
		So(err, ShouldNotBeNil)

		info, err := r.Info("p.Good")
		So(err, ShouldBeNil)
		So(info.Package, ShouldEqual, "p")
	})
}

func TestResolveInlineDescription(t *testing.T) {
	Convey("an object without its own description takes it from the inline type", t, func() {
		inner := mustSchema(t, apiextv1.JSONSchemaProps{
			Type:        "object",
			Description: "описание встроенного типа",
			Properties:  map[string]apiextv1.JSONSchemaProps{"v": {Type: "string"}},
		})
		wrapper := mustSchema(t, apiextv1.JSONSchemaProps{
			Type:  "object",
			AllOf: []apiextv1.JSONSchemaProps{{Ref: ptr("p.Inner")}},
		})
		described := mustSchema(t, apiextv1.JSONSchemaProps{
			Type:        "object",
			Description: "своё описание",
			AllOf:       []apiextv1.JSONSchemaProps{{Ref: ptr("p.Inner")}},
		})
		r := New()
		So(r.Add(Index{}, File{Package: "p", Types: []Type{
			{Name: "Inner", Schema: inner},
			{Name: "Wrapper", Schema: wrapper},
			{Name: "Described", Schema: described},
		}}), ShouldBeNil)

		s, err := r.Resolve("p.Wrapper")
		So(err, ShouldBeNil)
		So(s.Description, ShouldEqual, "описание встроенного типа")
		So(s.Properties, ShouldContainKey, "v")

		s, err = r.Resolve("p.Described")
		So(err, ShouldBeNil)
		So(s.Description, ShouldEqual, "своё описание")
	})
}
