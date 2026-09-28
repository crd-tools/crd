package crd

import (
	"reflect"
	"testing"
	"testing/fstest"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd/registry"
)

// prepSpec попадает в снимки, его поле имеет тип с Rule
type prepSpec struct {
	Owner clientID `json:"owner"`
}

// snapshotOf строит каталог снимков из схем по ключам
func snapshotOf(t *testing.T, schemas map[string]apiextv1.JSONSchemaProps) fstest.MapFS {
	t.Helper()
	files := map[string]*registry.File{}
	for key, s := range schemas {
		pkg, name, err := registry.SplitKey(key)
		if err != nil {
			t.Fatal(err)
		}
		data, err := s.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		f, ok := files[pkg]
		if !ok {
			f = &registry.File{Package: pkg}
			files[pkg] = f
		}
		f.Types = append(f.Types, registry.Type{Name: name, Refs: registry.Refs(&s), Schema: data})
	}
	fsys := fstest.MapFS{}
	var idx registry.Index
	for pkg, f := range files {
		data, err := registry.MarshalFile(*f)
		if err != nil {
			t.Fatal(err)
		}
		name := registry.FileName(pkg)
		fsys[name] = &fstest.MapFile{Data: data}
		idx.Files = append(idx.Files, name)
		for _, tp := range f.Types {
			idx.Types = append(idx.Types, registry.Entry{Type: registry.Key(pkg, tp.Name), File: name, Hash: registry.Hash(tp.Schema)})
		}
	}
	data, err := registry.MarshalIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	fsys[registry.IndexFile] = &fstest.MapFile{Data: data}
	return fsys
}

func TestPrepare(t *testing.T) {
	Convey("SchemaFor walks the Go types behind a snapshot", t, func() {
		ref := clientIDKey
		ten := int64(10)
		fsys := snapshotOf(t, map[string]apiextv1.JSONSchemaProps{
			registryKey[prepSpec](): {Type: "object", Properties: map[string]apiextv1.JSONSchemaProps{"owner": {Ref: &ref}}},
			clientIDKey:             {Type: "string", MaxLength: &ten},
		})

		Convey("Schema by key uses only the cache: the snapshot of clientID", func() {
			s := newState()
			s.setDERSource(fsys)
			got, err := s.schema(registryKey[prepSpec]())
			So(err, ShouldBeNil)
			So(*got.Properties["owner"].MaxLength, ShouldEqual, 10)
		})

		Convey("SchemaFor registers clientID from its Rule before resolving", func() {
			s := newState()
			s.setDERSource(fsys)
			got, err := s.schemaFor(reflect.TypeFor[prepSpec]())
			So(err, ShouldBeNil)
			So(*got.Properties["owner"].MaxLength, ShouldEqual, 64)

			Convey("and after that Schema by key sees the rule too", func() {
				again, err := s.schema(registryKey[prepSpec]())
				So(err, ShouldBeNil)
				So(*again.Properties["owner"].MaxLength, ShouldEqual, 64)
			})
		})
	})
}
