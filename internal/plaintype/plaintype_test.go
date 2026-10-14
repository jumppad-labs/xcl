package plaintype

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// agent is a plain block type: no types.ResourceBase.
type agent struct {
	Enabled *bool    `xcl:"enabled,optional"`
	Models  []string `xcl:"models,optional"`
}

// lookalike has the same fields as agent and is a different type.
type lookalike struct {
	Enabled *bool    `xcl:"enabled,optional"`
	Models  []string `xcl:"models,optional"`
}

// named takes its name from the block's label.
type named struct {
	Name   string   `xcl:"name,label"`
	Models []string `xcl:"models,optional"`
}

// subtyped takes the subtype and the name from a block with two labels.
type subtyped struct {
	Engine string `xcl:"engine,label"`
	Name   string `xcl:"name,label"`
	Port   int    `xcl:"port,optional"`
}

// mirrored sees the disabled flag and dependencies every block has.
type mirrored struct {
	Disabled  bool     `xcl:"disabled,optional"`
	DependsOn []string `xcl:"depends_on,optional"`
	Models    []string `xcl:"models,optional"`
}

// timeouts is composed into composed.
type timeouts struct {
	Connect int `xcl:"connect,optional"`
}

// composed embeds another struct as part of its own body.
type composed struct {
	timeouts `xcl:",remain"`
	Host     string `xcl:"host,optional"`
}

// withUnexported has a field nothing could decode into.
type withUnexported struct {
	Host  string `xcl:"host,optional"`
	cache map[string]string
}

// secret holds a value that must never be shown.
type secret struct {
	Password types.Sensitive[string] `xcl:"password,optional"`
}

// build builds plain, failing the test when it is refused.
func build(t *testing.T, plain any, labels int) *Built {
	t.Helper()

	built, err := Build(reflect.TypeOf(plain), labels)
	require.NoError(t, err)

	return built
}

// entity returns a new engine value of built with its metadata set, as the
// parser would set it.
func entity(t *testing.T, built *Built, subtype, name string) reflect.Value {
	t.Helper()

	value := reflect.ValueOf(built.New()).Elem()
	base := value.Field(0).Addr().Interface().(*types.ResourceBase)
	base.Meta.Subtype = subtype
	base.Meta.Name = name

	return value
}

func TestBuildEmbedsResourceBaseFirst(t *testing.T) {
	built := build(t, agent{}, 1)

	first := built.Engine.Field(0)
	require.Equal(t, "ResourceBase", first.Name)
	require.True(t, first.Anonymous)
	require.Equal(t, `xcl:",remain"`, string(first.Tag))

	_, err := types.GetMeta(built.New())
	require.NoError(t, err)
}

func TestBuildCopiesFieldsWithTheirTags(t *testing.T) {
	built := build(t, agent{}, 1)

	models, ok := built.Engine.FieldByName("Models")
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf([]string{}), models.Type)
	require.Equal(t, `xcl:"models,optional"`, string(models.Tag))
}

func TestBuildIsAValidSchemaType(t *testing.T) {
	built := build(t, agent{}, 1)

	require.NoError(t, gohcl.ValidateSchemaType(built.Engine))
}

func TestBuildKeepsIdenticalPlainTypesApart(t *testing.T) {
	one := build(t, agent{}, 1)
	other := build(t, lookalike{}, 1)

	require.NotEqual(t, one.Engine, other.Engine)
}

func TestBuildLeavesOutUnexportedFields(t *testing.T) {
	built := build(t, withUnexported{}, 1)

	_, ok := built.Engine.FieldByName("cache")
	require.False(t, ok)
}

func TestBuildLiftsTheFieldsOfARemainStruct(t *testing.T) {
	built := build(t, composed{}, 1)

	connect, ok := built.Engine.FieldByName("Connect")
	require.True(t, ok)
	require.Equal(t, `xcl:"connect,optional"`, string(connect.Tag))
	require.NoError(t, gohcl.ValidateSchemaType(built.Engine))
}

func TestBuildKeepsLabelsInTheEngineType(t *testing.T) {
	built := build(t, named{}, 1)

	name, ok := built.Engine.FieldByName("Name")
	require.True(t, ok)
	require.Equal(t, `xcl:"name,label"`, string(name.Tag))
}

func TestBuildWithLabelsIsAValidSchemaType(t *testing.T) {
	built := build(t, subtyped{}, 2)

	require.NoError(t, gohcl.ValidateSchemaType(built.Engine))
}

func TestBuildRefusesALabelNamedAsEveryBlocksOwn(t *testing.T) {
	type labelledMeta struct {
		Label string `xcl:"meta,label"`
	}

	_, err := Build(reflect.TypeOf(labelledMeta{}), 1)
	require.ErrorContains(t, err, `"meta" is held by every block and cannot name a label`)
}

func TestSetLabelsFillsTheName(t *testing.T) {
	built := build(t, named{}, 1)
	entity := built.New()

	built.SetLabels(entity, "", "claude")

	require.Equal(t, "claude", reflect.ValueOf(entity).Elem().FieldByName("Name").String())
}

func TestSetLabelsFillsTheSubtypeAndName(t *testing.T) {
	built := build(t, subtyped{}, 2)
	entity := built.New()

	built.SetLabels(entity, "postgres", "main")

	value := reflect.ValueOf(entity).Elem()
	require.Equal(t, "postgres", value.FieldByName("Engine").String())
	require.Equal(t, "main", value.FieldByName("Name").String())
}

func TestBuildLeavesMirrorFieldsOutOfTheAttributes(t *testing.T) {
	built := build(t, mirrored{}, 1)

	field, _ := built.Engine.FieldByName("Disabled")
	require.Equal(t, []int{0, 1}, field.Index, "disabled is the one types.ResourceBase holds")

	field, _ = built.Engine.FieldByName("DependsOn")
	require.Equal(t, []int{0, 0}, field.Index, "depends_on is the one types.ResourceBase holds")
}

func TestBuildRefusesANonStruct(t *testing.T) {
	_, err := Build(reflect.TypeOf("text"), 1)
	require.ErrorContains(t, err, "is not a struct")
}

func TestBuildRefusesAnAttributeCalledMeta(t *testing.T) {
	type withMeta struct {
		Details string `xcl:"meta,optional"`
	}

	_, err := Build(reflect.TypeOf(withMeta{}), 1)
	require.ErrorContains(t, err, `"meta" is held by every block`)
}

// withMetadata wants its block's metadata.
type withMetadata struct {
	Name   string   `xcl:"name,label"`
	Models []string `xcl:"models,optional"`
	Meta   types.Meta
}

func TestBuildLeavesTheMetaFieldOutOfTheEngineType(t *testing.T) {
	built := build(t, withMetadata{}, 1)

	field, ok := built.Engine.FieldByName("Meta")
	require.True(t, ok)
	require.Equal(t, []int{0, 2}, field.Index, "Meta is the one types.ResourceBase holds")
}

func TestBuildRefusesAMetaFieldOfAnotherType(t *testing.T) {
	type wrongMeta struct {
		Meta map[string]string `xcl:"labels,optional"`
	}

	_, err := Build(reflect.TypeOf(wrongMeta{}), 1)
	require.ErrorContains(t, err, "must be a types.Meta")
}

func TestBuildRefusesATaggedMetaField(t *testing.T) {
	type taggedMeta struct {
		Meta types.Meta `xcl:"meta,optional"`
	}

	_, err := Build(reflect.TypeOf(taggedMeta{}), 1)
	require.ErrorContains(t, err, "takes no xcl tag")
}

func TestBuildRefusesAMetaFieldWithAnotherName(t *testing.T) {
	type renamedMeta struct {
		Source types.Meta
	}

	_, err := Build(reflect.TypeOf(renamedMeta{}), 1)
	require.ErrorContains(t, err, "must be called Meta")
}

func TestToPlainFillsTheMetaField(t *testing.T) {
	built := build(t, withMetadata{}, 1)
	stored := entity(t, built, "", "claude")
	base := stored.Field(0).Addr().Interface().(*types.ResourceBase)
	base.Meta.ID = "agent.claude"
	base.Meta.File = "runner.xcl"
	base.Meta.Line = 7

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)

	meta := plain.(*withMetadata).Meta
	require.Equal(t, "agent.claude", meta.ID)
	require.Equal(t, "runner.xcl", meta.File)
	require.Equal(t, 7, meta.Line)
}

func TestToPlainCopiesTheMetaFieldsLinks(t *testing.T) {
	built := build(t, withMetadata{}, 1)
	stored := entity(t, built, "", "claude")
	base := stored.Field(0).Addr().Interface().(*types.ResourceBase)
	base.Meta.Links = []string{"agent.first"}

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	plain.(*withMetadata).Meta.Links[0] = "changed"

	require.Equal(t, "agent.first", base.Meta.Links[0])
}

func TestBuildRefusesADisabledMirrorThatIsNotABool(t *testing.T) {
	type wrongDisabled struct {
		Disabled string `xcl:"disabled,optional"`
	}

	_, err := Build(reflect.TypeOf(wrongDisabled{}), 1)
	require.ErrorContains(t, err, "must be a bool")
}

func TestBuildRefusesADependsOnMirrorThatIsNotAStringSlice(t *testing.T) {
	type wrongDependsOn struct {
		DependsOn string `xcl:"depends_on,optional"`
	}

	_, err := Build(reflect.TypeOf(wrongDependsOn{}), 1)
	require.ErrorContains(t, err, "must be a []string")
}

func TestBuildRefusesMoreLabelsThanTheBlockHas(t *testing.T) {
	_, err := Build(reflect.TypeOf(subtyped{}), 1)
	require.ErrorContains(t, err, "has 2 label fields, a block of its type has 1 labels")
}

func TestBuildRefusesALabelThatIsNotAString(t *testing.T) {
	type numberLabel struct {
		Name int `xcl:"name,label"`
	}

	_, err := Build(reflect.TypeOf(numberLabel{}), 1)
	require.ErrorContains(t, err, "a label is a string")
}

func TestBuildRefusesAnEmbeddedFieldThatIsNotRemain(t *testing.T) {
	type embedsPlainly struct {
		timeouts
	}

	_, err := Build(reflect.TypeOf(embedsPlainly{}), 1)
	require.ErrorContains(t, err, "an embedded field must be a struct tagged")
}

func TestBuildRefusesARemainField(t *testing.T) {
	type takesTheRest struct {
		Rest map[string]string `xcl:",remain"`
	}

	_, err := Build(reflect.TypeOf(takesTheRest{}), 1)
	require.ErrorContains(t, err, "a plain type cannot take them")
}

func TestBuildRefusesTheGoNamesXclUses(t *testing.T) {
	type usesResourceBase struct {
		ResourceBase string `xcl:"resource_base,optional"`
	}

	_, err := Build(reflect.TypeOf(usesResourceBase{}), 1)
	require.ErrorContains(t, err, "the Go name ResourceBase is used by xcl")
}

func TestBuildRefusesTheSameGoNameTwiceOnceLifted(t *testing.T) {
	type clashes struct {
		timeouts `xcl:",remain"`
		Connect  string `xcl:"connect_to,optional"`
	}

	_, err := Build(reflect.TypeOf(clashes{}), 1)
	require.ErrorContains(t, err, "another field has the same Go name")
}

func TestPlainOfNamesThePlainType(t *testing.T) {
	built := build(t, agent{}, 1)

	plain, ok := PlainOf(reflect.TypeOf(built.New()))
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(agent{}), plain)
}

func TestPlainOfIsFalseForATypeItDidNotBuild(t *testing.T) {
	_, ok := PlainOf(reflect.TypeOf(types.Output{}))
	require.False(t, ok)
}

func TestLookupFindsTheBuiltTypeOfAnEntity(t *testing.T) {
	built := build(t, agent{}, 1)

	found, ok := Lookup(reflect.TypeOf(built.New()))
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(agent{}), found.Plain)
}

func TestLookupIsFalseForATypeItDidNotBuild(t *testing.T) {
	_, ok := Lookup(reflect.TypeOf(&types.Output{}))
	require.False(t, ok)
}

func TestIsBuiltForABuiltPlainType(t *testing.T) {
	build(t, agent{}, 1)

	require.True(t, IsBuilt(reflect.TypeOf(&agent{})))
}

func TestIsBuiltIsFalseForATypeNeverBuilt(t *testing.T) {
	type neverBuilt struct {
		Value string `xcl:"value,optional"`
	}

	require.False(t, IsBuilt(reflect.TypeOf(neverBuilt{})))
}

func TestFromPlainCopiesTheFieldsBack(t *testing.T) {
	built := build(t, agent{}, 1)
	stored := entity(t, built, "", "claude")

	err := built.FromPlain(stored.Addr().Interface(), &agent{Models: []string{"haiku"}})
	require.NoError(t, err)
	require.Equal(t, []string{"haiku"}, stored.FieldByName("Models").Interface())
}

func TestFromPlainKeepsTheEntitysMetaWithoutAMetaField(t *testing.T) {
	built := build(t, agent{}, 1)
	stored := entity(t, built, "", "claude")
	base := stored.Field(0).Addr().Interface().(*types.ResourceBase)
	base.Meta.ID = "agent.claude"

	err := built.FromPlain(stored.Addr().Interface(), agent{})
	require.NoError(t, err)
	require.Equal(t, "agent.claude", base.Meta.ID)
}

func TestFromPlainCopiesTheMetaFieldBack(t *testing.T) {
	built := build(t, withMetadata{}, 1)
	stored := entity(t, built, "", "claude")
	base := stored.Field(0).Addr().Interface().(*types.ResourceBase)

	plain := &withMetadata{Meta: types.Meta{ID: "agent.claude", Properties: map[string]any{"id": "abc"}}}

	err := built.FromPlain(stored.Addr().Interface(), plain)
	require.NoError(t, err)
	require.Equal(t, "abc", base.Meta.Properties["id"])
}

func TestFromPlainRefusesAnotherType(t *testing.T) {
	built := build(t, agent{}, 1)
	stored := entity(t, built, "", "claude")

	err := built.FromPlain(stored.Addr().Interface(), &lookalike{})
	require.Error(t, err)
}

func TestToPlainCopiesTheFields(t *testing.T) {
	built := build(t, agent{}, 1)
	stored := entity(t, built, "", "claude")
	stored.FieldByName("Models").Set(reflect.ValueOf([]string{"opus"}))

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	require.Equal(t, []string{"opus"}, plain.(*agent).Models)
}

func TestToPlainFillsTheNameLabel(t *testing.T) {
	built := build(t, named{}, 1)
	stored := entity(t, built, "", "claude")

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	require.Equal(t, "claude", plain.(*named).Name)
}

func TestToPlainFillsTheSubtypeAndNameLabels(t *testing.T) {
	built := build(t, subtyped{}, 2)
	stored := entity(t, built, "postgres", "main")

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	require.Equal(t, "postgres", plain.(*subtyped).Engine)
	require.Equal(t, "main", plain.(*subtyped).Name)
}

func TestToPlainFillsTheMirrorFields(t *testing.T) {
	built := build(t, mirrored{}, 1)
	stored := entity(t, built, "", "claude")
	base := stored.Field(0).Addr().Interface().(*types.ResourceBase)
	base.Disabled = true
	base.DependsOn = []string{"agent.ollama"}

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	require.True(t, plain.(*mirrored).Disabled)
	require.Equal(t, []string{"agent.ollama"}, plain.(*mirrored).DependsOn)
}

func TestToPlainFillsTheFieldsOfARemainStruct(t *testing.T) {
	built := build(t, composed{}, 1)
	stored := entity(t, built, "", "db")
	stored.FieldByName("Connect").SetInt(10)

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	require.Equal(t, 10, plain.(*composed).Connect)
}

func TestToPlainCopiesASensitiveValueExactly(t *testing.T) {
	built := build(t, secret{}, 1)
	stored := entity(t, built, "", "db")
	stored.FieldByName("Password").Set(reflect.ValueOf(types.NewSensitive("hunter2")))

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	require.Equal(t, "hunter2", plain.(*secret).Password.Reveal())
}

func TestToPlainReturnsACopy(t *testing.T) {
	built := build(t, mirrored{}, 1)
	stored := entity(t, built, "", "claude")
	base := stored.Field(0).Addr().Interface().(*types.ResourceBase)
	base.DependsOn = []string{"agent.ollama"}

	plain, err := built.ToPlain(stored.Addr().Interface())
	require.NoError(t, err)
	plain.(*mirrored).DependsOn[0] = "changed"

	require.Equal(t, "agent.ollama", base.DependsOn[0])
}

func TestToPlainRefusesAnEntityOfAnotherType(t *testing.T) {
	built := build(t, agent{}, 1)

	_, err := built.ToPlain(&types.Output{})
	require.ErrorContains(t, err, "is not an entity of")
}
