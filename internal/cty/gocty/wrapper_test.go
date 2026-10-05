package gocty

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/stretchr/testify/require"
)

// testMark is the mark the test wrapper applies.
type testMarkType struct{}

var testMark any = testMarkType{}

// testBox is a test-only wrapper holding one value of any type.
type testBox[T any] struct {
	value T
}

type testBoxed interface {
	boxedValue() any
}

type testBoxSetter interface {
	setBoxed(any)
}

func (b testBox[T]) boxedValue() any { return b.value }

func (b *testBox[T]) setBoxed(v any) { b.value = v.(T) }

type boxHolder struct {
	Secret testBox[string] `xcl:"secret"`
	Plain  string          `xcl:"plain"`
}

type plainHolder struct {
	Plain string `xcl:"plain"`
}

func init() {
	boxedType := reflect.TypeOf((*testBoxed)(nil)).Elem()

	RegisterWrapper(Wrapper{
		Inner: func(t reflect.Type) (reflect.Type, bool) {
			if t.Kind() != reflect.Struct || !t.Implements(boxedType) {
				return nil, false
			}

			return t.Field(0).Type, true
		},
		Unwrap: func(wrapper reflect.Value) reflect.Value {
			return reflect.ValueOf(wrapper.Interface().(testBoxed).boxedValue())
		},
		Wrap: func(target reflect.Value, inner reflect.Value) {
			target.Addr().Interface().(testBoxSetter).setBoxed(inner.Interface())
		},
		Mark: testMark,
	})
}

func TestWrapperImpliedTypeIsInnerType(t *testing.T) {
	ty, err := ImpliedType(testBox[string]{})
	require.NoError(t, err)

	require.Equal(t, cty.String, ty)
}

func TestWrapperStructFieldImpliesInnerAttributeType(t *testing.T) {
	ty, err := ImpliedType(boxHolder{})
	require.NoError(t, err)

	require.Equal(t, cty.Object(map[string]cty.Type{
		"secret": cty.String,
		"plain":  cty.String,
	}), ty)
}

func TestWrapperToCtyValueIsMarked(t *testing.T) {
	val, err := ToCtyValue(testBox[string]{value: "abc"}, cty.String)
	require.NoError(t, err)

	require.True(t, val.HasMark(testMark))

	unmarked, _ := val.Unmark()
	require.Equal(t, "abc", unmarked.AsString())
}

func TestWrapperStructFieldIsMarkedAndSiblingIsNot(t *testing.T) {
	input := boxHolder{Secret: testBox[string]{value: "abc"}, Plain: "visible"}

	ty, err := ImpliedType(input)
	require.NoError(t, err)

	val, err := ToCtyValue(input, ty)
	require.NoError(t, err)

	secret := val.GetAttr("secret")
	require.True(t, secret.HasMark(testMark))

	plain := val.GetAttr("plain")
	require.False(t, plain.HasMark(testMark))
	require.Equal(t, "visible", plain.AsString())
}

func TestWrapperFromCtyValueReadsMarkedValue(t *testing.T) {
	var target testBox[string]

	err := FromCtyValue(cty.StringVal("abc").Mark(testMark), &target)
	require.NoError(t, err)

	require.Equal(t, "abc", target.value)
}

func TestWrapperFromCtyValueReadsPlainValue(t *testing.T) {
	var target testBox[string]

	err := FromCtyValue(cty.StringVal("abc"), &target)
	require.NoError(t, err)

	require.Equal(t, "abc", target.value)
}

func TestWrapperFromCtyValueReadsMarkedAttributeIntoWrapperField(t *testing.T) {
	var target boxHolder

	val := cty.ObjectVal(map[string]cty.Value{
		"secret": cty.StringVal("abc").Mark(testMark),
		"plain":  cty.StringVal("visible"),
	})

	err := FromCtyValue(val, &target)
	require.NoError(t, err)

	require.Equal(t, "abc", target.Secret.value)
	require.Equal(t, "visible", target.Plain)
}

func TestMarkedValueIntoPlainStringReturnsError(t *testing.T) {
	var target string

	err := FromCtyValue(cty.StringVal("abc").Mark(testMark), &target)
	require.Error(t, err)

	require.Contains(t, err.Error(), "value is sensitive")
}

func TestMarkedAttributeIntoPlainFieldErrorNamesTheAttribute(t *testing.T) {
	var target plainHolder

	val := cty.ObjectVal(map[string]cty.Value{
		"plain": cty.StringVal("abc").Mark(testMark),
	})

	err := FromCtyValue(val, &target)
	require.Error(t, err)

	pathErr, ok := err.(cty.PathError)
	require.True(t, ok, "expected a cty.PathError, got %T", err)

	require.Equal(t, cty.Path{cty.GetAttrStep{Name: "plain"}}, pathErr.Path)
	require.Contains(t, pathErr.Error(), "value is sensitive")
}
