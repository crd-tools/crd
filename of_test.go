package crd

import (
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type ofTag struct {
	Name string `json:"name"`
}

func (s *ofTag) CRD(b Builder) error {
	b.String(&s.Name).MinLength(1)
	return nil
}

// ofPlain без Describer, описывается выводом из Go-типа
type ofPlain struct {
	V int `json:"v"`
}

type ofSpec struct {
	Tags  map[string][]ofTag `json:"tags"`
	Plain []ofPlain          `json:"plain"`
}

func (s *ofSpec) CRD(b Builder) error {
	b.Map(&s.Tags).MaxProperties(5).Values(NewArray(Of[ofTag]()).MinItems(1))
	b.Array(&s.Plain).Items(Of[ofPlain]())
	return nil
}

type ofLoop struct {
	Next []ofLoop `json:"next"`
}

func (s *ofLoop) CRD(b Builder) error {
	b.Array(&s.Next).Items(Of[ofLoop]())
	return nil
}

func TestOf(t *testing.T) {
	Convey("Of references a type as a value rule", t, func() {
		s, err := newStateWithStd(t).schemaFor(reflect.TypeFor[ofSpec]())
		So(err, ShouldBeNil)
		So(structural(s), ShouldBeNil)

		tags := s.Properties["tags"].AdditionalProperties.Schema
		So(*tags.MinItems, ShouldEqual, 1)
		So(*tags.Items.Schema.Properties["name"].MinLength, ShouldEqual, 1)
		So(s.Properties["plain"].Items.Schema.Properties["v"].Type, ShouldEqual, "integer")
	})

	Convey("recursion through Of is reported", t, func() {
		_, err := newStateWithStd(t).schemaFor(reflect.TypeFor[ofLoop]())
		So(Code(err), ShouldEqual, RecursiveType)
	})
}

// newStateWithStd подменяет общее состояние на время теста, потому что Of пишет в std
func newStateWithStd(t *testing.T) *state {
	old := std
	std = newState()
	t.Cleanup(func() { std = old })
	return std
}
