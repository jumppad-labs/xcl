package resources

import (
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

var typeTestContainer = "container"

type testContainer struct {
	// embedded type holding name, etc
	types.ResourceBase `xcl:",remain"`
}

func TestParseFQRNParsesComponents(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.module2.resource.container.mine.attr.other")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, typeTestContainer, fqrn.Subtype)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "attr.other", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.module2.resource.container.mine.attr.other", sfrqn)
}

func TestParseFQRNParsesComponents2(t *testing.T) {
	fqrn, err := ParseFQRN("module.consul_1.resource.network.onprem.name")
	require.NoError(t, err)

	require.Equal(t, "consul_1", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, "network", fqrn.Subtype)
	require.Equal(t, "onprem", fqrn.Resource)
	require.Equal(t, "name", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.consul_1.resource.network.onprem.name", sfrqn)
}

func TestParseFQRNParsesSplat(t *testing.T) {
	fqrn, err := ParseFQRN("resource.chapter.installation.tasks.*.id")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, "chapter", fqrn.Subtype)
	require.Equal(t, "installation", fqrn.Resource)
	require.Equal(t, "tasks.*.id", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "resource.chapter.installation.tasks.*.id", sfrqn)
}

func TestParseFQRNParsesModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeModule, fqrn.Type)
	require.Equal(t, "module1", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.module1", sfrqn)
}

func TestParseFQRNParsesModuleInModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.container")
	require.NoError(t, err)

	require.Equal(t, "module1", fqrn.Module)
	require.Equal(t, TypeModule, fqrn.Type)
	require.Equal(t, "container", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.container", sfrqn)
}

func TestParseFQDNReturnsErrorOnMissingType(t *testing.T) {
	_, err := ParseFQRN("module.module1.module2.resource.mine")
	require.Error(t, err)
}

func TestParseFQRNReturnsOutputWhenInNestedModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.module2.output.mine")
	require.NoError(t, err)

	require.Equal(t, "module1.module2", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.module2.output.mine", sfrqn)
}

func TestParseFQRNReturnsOutputWhenInModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.consul_1.output.container_resources_cpu")
	require.NoError(t, err)

	require.Equal(t, "consul_1", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "container_resources_cpu", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "module.consul_1.output.container_resources_cpu", sfrqn)
}

func TestParseFQRNReturnsResource(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, "container", fqrn.Subtype)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	// should also reverse
	sfrqn := fqrn.String()
	require.Equal(t, "resource.container.mine", sfrqn)
}

func TestParseFQRNReturnsResourceWithAttr(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine.my.stuff")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, "container", fqrn.Subtype)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "my.stuff", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "resource.container.mine.my.stuff", sfrqn)
}

func TestParseFQRNReturnsOutputInModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	require.Equal(t, "module1", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.module1.output.mine", sfrqn)
}

func TestParseFQRNReturnsOutput(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine", sfrqn)
}

func TestParseResourceFQRNWithIndexReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine.property.0")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, typeTestContainer, fqrn.Subtype)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "property.0", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "resource.container.mine.property.0", sfrqn)
}

func TestParseFQRNWithIndexReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine.0")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "0", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine.0", sfrqn)
}

func TestParseFQRNWithParenthesesIndexReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine[0]")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "0", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine.0", sfrqn)
}

func TestParseFQRNWithParenthesesIndexAndAttributeReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine[0].nic")
	require.NoError(t, err)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "0.nic", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "output.mine.0.nic", sfrqn)
}

func TestParseFQRNWithIndexAndModuleReturnsCorrectData(t *testing.T) {
	fqrn, err := ParseFQRN("module.mine.output.mine.name")
	require.NoError(t, err)

	require.Equal(t, "mine", fqrn.Module)
	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "mine", fqrn.Resource)
	require.Equal(t, "name", fqrn.Attribute)

	sfrqn := fqrn.String()
	require.Equal(t, "module.mine.output.mine.name", sfrqn)
}

func TestFQRNFromResourceReturnsCorrectData(t *testing.T) {
	dt := DefaultResources()
	dt[typeTestContainer] = &testContainer{}

	r, err := dt.CreateResource(typeTestContainer, "mytest")
	require.NoError(t, err)

	// CreateResource sets the axes from the name alone, which is only right
	// for a single label builtin. A variety bearing type is corrected by
	// PluginRegistry.CreateResource, so set the axes here the same way
	meta, _ := types.GetMeta(r)
	meta.Module = "mymodule"
	meta.Type = types.TypeResource
	meta.Subtype = typeTestContainer

	fqrn := FQRNFromResource(r)

	require.Equal(t, "mymodule", fqrn.Module)
	require.Equal(t, types.TypeResource, fqrn.Type)
	require.Equal(t, typeTestContainer, fqrn.Subtype)
	require.Equal(t, "mytest", fqrn.Resource)

	sfrqn := fqrn.String()
	require.Equal(t, "module.mymodule.resource.container.mytest", sfrqn)
}

func TestFQRNFromVariableReturnsCorrectData(t *testing.T) {
	dt := DefaultResources()

	r, err := dt.CreateResource(TypeVariable, "mytest")
	require.NoError(t, err)

	//r.Metadata().Module = "mymodule"

	fqrn := FQRNFromResource(r)

	require.Equal(t, "", fqrn.Module)
	require.Equal(t, TypeVariable, fqrn.Type)
	require.Equal(t, "mytest", fqrn.Resource)

	sfrqn := fqrn.String()
	require.Equal(t, "variable.mytest", sfrqn)
}

func TestFQRNFromVariableInModuleReturnsCorrectData(t *testing.T) {
	dt := DefaultResources()

	r, err := dt.CreateResource(TypeVariable, "mytest")
	require.NoError(t, err)

	meta, _ := types.GetMeta(r)
	meta.Module = "mymodule"

	fqrn := FQRNFromResource(r)

	require.Equal(t, "mymodule", fqrn.Module)
	require.Equal(t, TypeVariable, fqrn.Type)
	require.Equal(t, "mytest", fqrn.Resource)

	sfrqn := fqrn.String()
	require.Equal(t, "module.mymodule.variable.mytest", sfrqn)
}

func TestFQRNAppendsParentCorrectlyWhenNoModule(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("parent")
	require.Equal(t, "module.parent.output.mine", new.String())
}

func TestFQRNAppendsParentCorrectlyWhenExistingModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("parent")
	require.Equal(t, "module.parent.module1.output.mine", new.String())
}

func TestFQRNAppedDoesNothingWhenNoParent(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("")
	require.Equal(t, "module.module1.output.mine", new.String())
}

func TestFQRNAppendsParentKeepsSubtype(t *testing.T) {
	fqrn, err := ParseFQRN("resource.container.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("parent")

	require.Equal(t, types.TypeResource, new.Type)
	require.Equal(t, "container", new.Subtype)
	require.Equal(t, "module.parent.resource.container.mine", new.String())
}

func TestFQRNAppendsParentKeepsSubtypeWhenExistingModule(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.resource.container.mine")
	require.NoError(t, err)

	new := fqrn.AppendParentModule("parent")

	require.Equal(t, types.TypeResource, new.Type)
	require.Equal(t, "container", new.Subtype)
	require.Equal(t, "module.parent.module1.resource.container.mine", new.String())
}

func TestParseFQRNRoundTripsResourceAddress(t *testing.T) {
	address := "resource.container.mine"

	fqrn, err := ParseFQRN(address)
	require.NoError(t, err)

	require.Equal(t, address, fqrn.String())
}

func TestParseFQRNRoundTripsResourceAddressWithAttribute(t *testing.T) {
	address := "resource.container.mine.attr.other"

	fqrn, err := ParseFQRN(address)
	require.NoError(t, err)

	require.Equal(t, address, fqrn.String())
}

func TestParseFQRNRoundTripsModuleRelativeResourceAddress(t *testing.T) {
	address := "module.module1.module2.resource.container.mine"

	fqrn, err := ParseFQRN(address)
	require.NoError(t, err)

	require.Equal(t, address, fqrn.String())
}

func TestFQRNFromResourceFormatsAnAddressThatParsesBackToTheSameParts(t *testing.T) {
	r := &testContainer{}

	meta, err := types.GetMeta(r)
	require.NoError(t, err)

	meta.Type = types.TypeResource
	meta.Subtype = typeTestContainer
	meta.Name = "mytest"
	meta.Module = "module1.module2"

	fqrn := FQRNFromResource(r)
	require.NotNil(t, fqrn)

	address := fqrn.String()
	require.Equal(t, "module.module1.module2.resource.container.mytest", address)

	parsed, err := ParseFQRN(address)
	require.NoError(t, err)

	require.Equal(t, fqrn.Module, parsed.Module)
	require.Equal(t, fqrn.Type, parsed.Type)
	require.Equal(t, fqrn.Subtype, parsed.Subtype)
	require.Equal(t, fqrn.Resource, parsed.Resource)
	require.Equal(t, fqrn.Attribute, parsed.Attribute)
}

func TestParseFQRNKeepsVariableAddressForm(t *testing.T) {
	fqrn, err := ParseFQRN("variable.mytest")
	require.NoError(t, err)

	require.Equal(t, TypeVariable, fqrn.Type)
	require.Equal(t, "", fqrn.Subtype)
	require.Equal(t, "variable.mytest", fqrn.String())
}

func TestParseFQRNKeepsOutputAddressForm(t *testing.T) {
	fqrn, err := ParseFQRN("output.mine")
	require.NoError(t, err)

	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "", fqrn.Subtype)
	require.Equal(t, "output.mine", fqrn.String())
}

func TestParseFQRNKeepsModuleAddressForm(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1")
	require.NoError(t, err)

	require.Equal(t, TypeModule, fqrn.Type)
	require.Equal(t, "", fqrn.Subtype)
	require.Equal(t, "module.module1", fqrn.String())
}

func TestParseFQRNKeepsOutputInModuleAddressForm(t *testing.T) {
	fqrn, err := ParseFQRN("module.module1.output.mine")
	require.NoError(t, err)

	require.Equal(t, TypeOutput, fqrn.Type)
	require.Equal(t, "", fqrn.Subtype)
	require.Equal(t, "module.module1.output.mine", fqrn.String())
}

func TestParseFQRNKeepsLocalAddressForm(t *testing.T) {
	fqrn, err := ParseFQRN("local.x")
	require.NoError(t, err)

	require.Equal(t, "local", fqrn.Type)
	require.Equal(t, "", fqrn.Subtype)

	// the formatter emits the resource. prefix only for the resource kind, so a
	// local renders through its own kind and round-trips as local.x. This falls
	// out of opening the address grammar to every single label kind, it was not
	// a fix that went looking for this address
	require.Equal(t, "local.x", fqrn.String())
}

// A module relative address cannot be split by position alone: in
// module.m1.container.nics, "container" begins the body only when something is
// registered under that name. AddressParser exists to settle that, and is the
// only reason it exists, because a body is otherwise read positionally.

func TestAddressParserSplitsTheModulePrefixOfAKnownType(t *testing.T) {
	parser := NewAddressParser([]types.TypeInfo{{Name: typeTestContainer, Bare: true}})

	fqrn, err := parser.Parse("module.m1.container.nics")
	require.NoError(t, err)

	require.Equal(t, "m1", fqrn.Module)
	require.Equal(t, typeTestContainer, fqrn.Type)
	require.Equal(t, "", fqrn.Subtype)
	require.Equal(t, "nics", fqrn.Resource)
}

func TestParseFQRNReadsAnUnknownTypeSegmentAsPartOfTheModulePath(t *testing.T) {
	// ParseFQRN resolves the structural keywords only, so nothing in
	// module.m1.container.nics names a type and the whole prefix is a module
	fqrn, err := ParseFQRN("module.m1.container.nics")
	require.NoError(t, err)

	require.Equal(t, "m1.container", fqrn.Module)
	require.Equal(t, TypeModule, fqrn.Type)
	require.Equal(t, "nics", fqrn.Resource)
}
