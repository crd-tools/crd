package crd

import (
	"testing"
	"testing/fstest"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd/registry"
)

// clientID описывает свою схему через Ruler
type clientID string

type testRule struct{ max int64 }

func (r testRule) Schema() (apiextv1.JSONSchemaProps, error) {
	return apiextv1.JSONSchemaProps{Type: "string", MaxLength: &r.max}, nil
}

func (clientID) Rule() (Rule, error) {
	return testRule{max: 64}, nil
}

type plain struct{}

const clientIDKey = "crd.tools/crd.clientID"

// snapshot строит каталог снимков: Parent ссылается на clientID, у которого в снимке maxLength 10
func snapshot(t *testing.T) fstest.MapFS {
	t.Helper()
	ten := int64(10)
	schema := func(s apiextv1.JSONSchemaProps) []byte {
		data, err := s.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	ref := clientIDKey
	parent := schema(apiextv1.JSONSchemaProps{
		Type:       "object",
		Properties: map[string]apiextv1.JSONSchemaProps{"id": {Ref: &ref}},
	})
	child := schema(apiextv1.JSONSchemaProps{Type: "string", MaxLength: &ten})

	files := map[string]registry.File{
		registry.FileName("example.com/snap"): {Package: "example.com/snap", Types: []registry.Type{{Name: "Parent", Refs: []string{ref}, Schema: parent}}},
		registry.FileName("crd.tools/crd"):    {Package: "crd.tools/crd", Types: []registry.Type{{Name: "clientID", Schema: child}}},
	}
	fsys := fstest.MapFS{}
	var idx registry.Index
	for name, f := range files {
		data, err := registry.MarshalFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
		idx.Files = append(idx.Files, name)
		for _, tp := range f.Types {
			idx.Types = append(idx.Types, registry.Entry{
				Type: registry.Key(f.Package, tp.Name),
				File: name,
				Hash: registry.Hash(tp.Schema),
			})
		}
	}
	data, err := registry.MarshalIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	fsys[registry.IndexFile] = &fstest.MapFile{Data: data}
	return fsys
}

func TestState(t *testing.T) {
	Convey("DER source and runtime rules", t, func() {
		s := newState()
		s.setDERSource(snapshot(t))

		Convey("snapshot is used until a rule is registered", func() {
			got, err := s.schema("example.com/snap.Parent")
			So(err, ShouldBeNil)
			So(*got.Properties["id"].MaxLength, ShouldEqual, 10)
		})

		Convey("runtime rule wins, also through $ref", func() {
			So(s.register(clientID("")), ShouldBeNil)

			got, err := s.schema("example.com/snap.Parent")
			So(err, ShouldBeNil)
			So(*got.Properties["id"].MaxLength, ShouldEqual, 64)

			direct, err := s.schema(clientIDKey)
			So(err, ShouldBeNil)
			So(*direct.MaxLength, ShouldEqual, 64)
		})

		Convey("register rejects types without Describer or Ruler", func() {
			So(Code(s.register(plain{})), ShouldEqual, NotDescribed)
			So(Code(s.register(nil)), ShouldEqual, InvalidArgument)
		})

		Convey("unknown type returns ErrNotFound", func() {
			_, err := s.schema("example.com/snap.Nope")
			So(Code(err), ShouldEqual, NotFound)
		})
	})

	Convey("DER source is read lazily", t, func() {
		s := newState()
		s.setDERSource(fstest.MapFS{})
		So(s.registerSchema(clientID(""), apiextv1.JSONSchemaProps{Type: "string"}), ShouldBeNil)
		_, err := s.schema(clientIDKey)
		So(Code(err), ShouldEqual, SourceLoad)
	})

	Convey("package functions", t, func() {
		So(Register(clientID("")), ShouldBeNil)
		got, err := SchemaFor[clientID]()
		So(err, ShouldBeNil)
		So(*got.MaxLength, ShouldEqual, 64)
	})
}
