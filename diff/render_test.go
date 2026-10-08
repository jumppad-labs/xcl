package diff_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/highlight"
)

// renderOne renders a diff holding only resource, with summary
func renderOne(resource diff.Resource, summary diff.Summary) string {
	return string(diff.Render(&diff.Diff{
		Summary:   summary,
		Resources: []diff.Resource{resource},
	}))
}

func TestRenderWritesCreatedResourceBlock(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionCreate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("image"), After: "nginx"},
		},
	}, diff.Summary{Create: 1})

	expected := "  # resource.container.web will be created\n" +
		"  + resource \"container\" \"web\" {\n" +
		"      + image = \"nginx\"\n" +
		"    }\n" +
		"\n" +
		"Diff: 1 to create, 0 to update, 0 to replace, 0 to delete, 0 unchanged.\n"

	require.Equal(t, expected, output)
}

func TestRenderWritesDeletedResourceHeaderWithEmptyBody(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.postgres.old",
		Action:  diff.ActionDelete,
	}, diff.Summary{Delete: 1})

	require.Contains(t, output, "  # resource.postgres.old will be deleted\n  - resource \"postgres\" \"old\" {}\n\n")
}

func TestRenderWritesReplacedResourceHeaderAndComment(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.network.app",
		Action:  diff.ActionReplace,
		Reason:  diff.ReplaceFailed,
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "  # resource.network.app will be replaced, its last apply failed\n-/+ resource \"network\" \"app\" {}\n\n")
}

func TestRenderReplaceBecauseDependencyNamesTheDependency(t *testing.T) {
	output := renderOne(diff.Resource{
		Address:      "resource.container.web",
		Action:       diff.ActionReplace,
		Reason:       diff.ReplaceDependency,
		ReplacedDeps: []string{"resource.network.app"},
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "  # resource.container.web will be replaced because resource.network.app is replaced\n-/+ resource \"container\" \"web\" {}\n\n")
}

func TestRenderReplaceBecauseOfSeveralDependenciesNamesThemAll(t *testing.T) {
	output := renderOne(diff.Resource{
		Address:      "resource.container.web",
		Action:       diff.ActionReplace,
		Reason:       diff.ReplaceDependency,
		ReplacedDeps: []string{"resource.network.app", "resource.volume.data"},
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "  # resource.container.web will be replaced because resource.network.app, resource.volume.data are replaced\n-/+ resource \"container\" \"web\" {}\n\n")
}

func TestRenderReplaceBecauseOfUnnamedDependencySaysADependencyIsReplaced(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionReplace,
		Reason:  diff.ReplaceDependency,
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "  # resource.container.web will be replaced because a dependency is replaced\n-/+ resource \"container\" \"web\" {}\n\n")
}

func TestRenderReplaceByProviderSaysItCannotBeUpdatedInPlace(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionReplace,
		Reason:  diff.ReplaceProvider,
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "  # resource.container.web will be replaced, it cannot be updated in place\n-/+ resource \"container\" \"web\" {}\n\n")
}

func TestRenderReplaceWithoutReasonSaysOnlyItWillBeReplaced(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionReplace,
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "  # resource.container.web will be replaced\n-/+ resource \"container\" \"web\" {}\n\n")
}

func TestRenderWritesUpdatedValueAsBeforeAndAfter(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("ports").Index(0).Attribute("host"), Before: float64(8080), After: float64(9090)},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "  # resource.container.api will be updated\n  ~ resource \"container\" \"api\" {\n")
	require.Contains(t, output, "      ~ ports[0].host = 8080 -> 9090\n")
}

func TestRenderWritesUpdateWithoutChangesAsChangedOutsideXCL(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.cache",
		Action:  diff.ActionUpdate,
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "  # resource.container.cache changed outside xcl and will be updated\n  ~ resource \"container\" \"cache\" {}\n\n")
}

func TestRenderWritesAddedElementWithPlusMarker(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("ports").Index(2), After: map[string]any{"host": float64(443), "local": float64(8443)}},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "      + ports[2] = { host = 443, local = 8443 }\n")
}

func TestRenderWritesRemovedElementWithMinusMarker(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("ports").Index(1), Before: map[string]any{"host": float64(80)}},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "      - ports[1] = { host = 80 }\n")
}

func TestRenderWritesUnknownValueInCreatedResourceAsKnownAfterApply(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionCreate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("db_host"), Unknown: true},
		},
	}, diff.Summary{Create: 1})

	require.Contains(t, output, "      + db_host = (known after apply)\n")
}

func TestRenderWritesUnknownValueWithBeforeAsChangeToKnownAfterApply(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.client",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("x"), Before: "a", Unknown: true},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "      ~ x = \"a\" -> (known after apply)\n")
}

func TestRenderWritesMaskedSensitiveValueAsPlaceholder(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("env").Key("DB_PASSWORD"), Sensitive: true},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "      ~ env[\"DB_PASSWORD\"] = (sensitive value)\n")
	require.NotContains(t, output, "hunter2-old-secret")
	require.NotContains(t, output, "correct-horse-new-secret")
}

func TestRenderWritesMaskedSensitiveValueInCreatedResourceWithPlusMarker(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionCreate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("password"), Sensitive: true},
		},
	}, diff.Summary{Create: 1})

	require.Contains(t, output, "      + password = (sensitive value)\n")
}

func TestRenderWritesRevealedSensitiveValues(t *testing.T) {
	output := renderOne(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{
				Path:      diff.Path{}.Attribute("env").Key("DB_PASSWORD"),
				Before:    "hunter2-old-secret",
				After:     "correct-horse-new-secret",
				Sensitive: true,
			},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "      ~ env[\"DB_PASSWORD\"] = \"hunter2-old-secret\" -> \"correct-horse-new-secret\"\n")
}

func TestRenderAlignsEqualsSignsWithinEachBlockIndependently(t *testing.T) {
	output := string(diff.Render(&diff.Diff{
		Summary: diff.Summary{Create: 1, Update: 1},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("a"), Before: "x", After: "y"},
					{Path: diff.Path{}.Attribute("longer_name"), Before: "x", After: "y"},
				},
			},
			{
				Address: "resource.container.web",
				Action:  diff.ActionCreate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("b"), After: "z"},
					{Path: diff.Path{}.Attribute("abc"), After: "z"},
				},
			},
		},
	}))

	require.Contains(t, output, "      ~ a           = \"x\" -> \"y\"\n      ~ longer_name = \"x\" -> \"y\"\n")
	require.Contains(t, output, "      + b   = \"z\"\n      + abc = \"z\"\n")
}

func TestRenderWritesResourcesInGivenOrder(t *testing.T) {
	output := string(diff.Render(&diff.Diff{
		Summary: diff.Summary{Create: 1, Delete: 1},
		Resources: []diff.Resource{
			{Address: "resource.zeta.last", Action: diff.ActionDelete},
			{Address: "resource.alpha.first", Action: diff.ActionCreate},
		},
	}))

	zeta := strings.Index(output, "resource.zeta.last")
	alpha := strings.Index(output, "resource.alpha.first")

	require.NotEqual(t, -1, zeta)
	require.NotEqual(t, -1, alpha)
	require.Less(t, zeta, alpha)
}

func TestRenderWritesSummaryWithCounts(t *testing.T) {
	output := string(diff.Render(&diff.Diff{
		Summary: diff.Summary{Create: 2, Update: 1, Replace: 1, Delete: 1, Unchanged: 3},
	}))

	require.True(t, strings.HasSuffix(output, "Diff: 2 to create, 1 to update, 1 to replace, 1 to delete, 3 unchanged.\n"))
}

func TestRenderWritesOnlyNoChangesLineWhenNothingChanges(t *testing.T) {
	output := string(diff.Render(&diff.Diff{
		Summary: diff.Summary{Unchanged: 3},
	}))

	require.Equal(t, "Diff: no changes, 3 unchanged.\n", output)
}

func TestRenderOfNilDiffWritesNoChanges(t *testing.T) {
	require.Equal(t, "Diff: no changes, 0 unchanged.\n", string(diff.Render(nil)))
}

func TestRenderWritesNoEscapeCodes(t *testing.T) {
	output := string(diff.Render(designExampleDiff()))

	require.NotContains(t, output, "\x1b")
}

func TestRenderWritesDesignExample(t *testing.T) {
	expected := `  # resource.container.api will be updated
  ~ resource "container" "api" {
      ~ ports[0].host      = 8080 -> 9090
      + ports[2]           = { host = 443, local = 8443 }
      ~ env["DB_PASSWORD"] = (sensitive value)
    }

  # resource.container.cache changed outside xcl and will be updated
  ~ resource "container" "cache" {}

  # resource.container.web will be created
  + resource "container" "web" {
      + image   = "nginx"
      + db_host = (known after apply)
    }

  # resource.network.app will be replaced, its last apply failed
-/+ resource "network" "app" {}

  # resource.postgres.old will be deleted
  - resource "postgres" "old" {}

Diff: 1 to create, 2 to update, 1 to replace, 1 to delete, 4 unchanged.
`

	require.Equal(t, expected, string(diff.Render(designExampleDiff())))
}

// designExampleDiff builds the diff shown in the design's rendering example
func designExampleDiff() *diff.Diff {
	return &diff.Diff{
		Summary: diff.Summary{Create: 1, Update: 2, Replace: 1, Delete: 1, Unchanged: 4},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("ports").Index(0).Attribute("host"), Before: float64(8080), After: float64(9090)},
					{Path: diff.Path{}.Attribute("ports").Index(2), After: map[string]any{"host": float64(443), "local": float64(8443)}},
					{Path: diff.Path{}.Attribute("env").Key("DB_PASSWORD"), Sensitive: true},
				},
			},
			{Address: "resource.container.cache", Action: diff.ActionUpdate},
			{
				Address: "resource.container.web",
				Action:  diff.ActionCreate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("image"), After: "nginx"},
					{Path: diff.Path{}.Attribute("db_host"), Unknown: true},
				},
			},
			{Address: "resource.network.app", Action: diff.ActionReplace, Reason: diff.ReplaceFailed},
			{Address: "resource.postgres.old", Action: diff.ActionDelete},
		},
	}
}

// sgrCodes matches the SGR escape codes the ANSI renderer writes
var sgrCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// renderMarkerRenderer wraps each labelled piece as [scope]text[/scope] and
// passes unlabelled text through unchanged, so a test can read which scope
// each piece was given
func renderMarkerRenderer() highlight.Renderer {
	return highlight.RendererFunc(func(scope, text string) string {
		if scope == "" {
			return text
		}

		return "[" + scope + "]" + text + "[/" + scope + "]"
	})
}

// renderOneHighlighted renders a diff holding only resource through the
// marker renderer
func renderOneHighlighted(resource diff.Resource, summary diff.Summary) string {
	return string(diff.Render(&diff.Diff{
		Summary:   summary,
		Resources: []diff.Resource{resource},
	}, diff.Highlight(renderMarkerRenderer())))
}

// createdImageResource is a created resource with one added string value
func createdImageResource() diff.Resource {
	return diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionCreate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("image"), After: "nginx"},
		},
	}
}

// updatedPortResource is an updated resource with one changed number value
func updatedPortResource() diff.Resource {
	return diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("port"), Before: float64(8080), After: float64(9090)},
		},
	}
}

func TestRenderHighlightLabelsCreatedHeaderMarkerAsInserted(t *testing.T) {
	output := renderOneHighlighted(createdImageResource(), diff.Summary{Create: 1})

	require.Contains(t, output, "\n  [markup.inserted.diff]+[/markup.inserted.diff] [storage.type.xcl]resource[/storage.type.xcl]")
}

func TestRenderHighlightLabelsAddedValueMarkerAndPathAsInserted(t *testing.T) {
	output := renderOneHighlighted(createdImageResource(), diff.Summary{Create: 1})

	require.Contains(t, output, "      [markup.inserted.diff]+[/markup.inserted.diff] [markup.inserted.diff]image[/markup.inserted.diff] = ")
}

func TestRenderHighlightLabelsDeletedHeaderMarkerAsDeleted(t *testing.T) {
	output := renderOneHighlighted(diff.Resource{
		Address: "resource.postgres.old",
		Action:  diff.ActionDelete,
	}, diff.Summary{Delete: 1})

	require.Contains(t, output, "\n  [markup.deleted.diff]-[/markup.deleted.diff] [storage.type.xcl]resource[/storage.type.xcl]")
}

func TestRenderHighlightLabelsUpdatedHeaderMarkerAsChanged(t *testing.T) {
	output := renderOneHighlighted(updatedPortResource(), diff.Summary{Update: 1})

	require.Contains(t, output, "\n  [markup.changed.diff]~[/markup.changed.diff] [storage.type.xcl]resource[/storage.type.xcl]")
}

func TestRenderHighlightLabelsChangedValueMarkerAndPathAsChanged(t *testing.T) {
	output := renderOneHighlighted(updatedPortResource(), diff.Summary{Update: 1})

	require.Contains(t, output, "      [markup.changed.diff]~[/markup.changed.diff] [markup.changed.diff]port[/markup.changed.diff] = ")
}

func TestRenderHighlightLabelsReplacedHeaderMarkerAsChanged(t *testing.T) {
	output := renderOneHighlighted(diff.Resource{
		Address: "resource.network.app",
		Action:  diff.ActionReplace,
	}, diff.Summary{Replace: 1})

	require.Contains(t, output, "\n[markup.changed.diff]-/+[/markup.changed.diff] [storage.type.xcl]resource[/storage.type.xcl]")
}

func TestRenderHighlightLabelsRemovedValueMarkerAndPathAsDeleted(t *testing.T) {
	output := renderOneHighlighted(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("image"), Before: "old"},
		},
	}, diff.Summary{Update: 1})

	require.Contains(t, output, "      [markup.deleted.diff]-[/markup.deleted.diff] [markup.deleted.diff]image[/markup.deleted.diff] = ")
}

func TestRenderHighlightLabelsCommentLineAsHashComment(t *testing.T) {
	output := renderOneHighlighted(createdImageResource(), diff.Summary{Create: 1})

	require.True(t, strings.HasPrefix(output, "  [comment.line.number-sign.xcl]# resource.container.web will be created[/comment.line.number-sign.xcl]\n"))
}

func TestRenderHighlightLabelsNumberValuesWithNumberScope(t *testing.T) {
	output := renderOneHighlighted(updatedPortResource(), diff.Summary{Update: 1})

	require.Contains(t, output, "= [constant.numeric.xcl]8080[/constant.numeric.xcl] -> [constant.numeric.xcl]9090[/constant.numeric.xcl]\n")
}

func TestRenderHighlightLabelsStringValueWithStringScope(t *testing.T) {
	output := renderOneHighlighted(createdImageResource(), diff.Summary{Create: 1})

	require.Contains(t, output, "= [string.quoted.double.xcl]\"nginx\"[/string.quoted.double.xcl]\n")
}

func TestRenderHighlightLabelsBlockHeaderWithGrammarScopes(t *testing.T) {
	output := renderOneHighlighted(createdImageResource(), diff.Summary{Create: 1})

	require.Contains(t, output, "[storage.type.xcl]resource[/storage.type.xcl] [entity.name.type.xcl]\"container\"[/entity.name.type.xcl] [entity.name.tag.xcl]\"web\"[/entity.name.tag.xcl] {\n")
}

func TestRenderHighlightPassesPlaceholdersAndSummaryUnlabelled(t *testing.T) {
	output := renderOneHighlighted(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionCreate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("db_host"), Unknown: true},
		},
	}, diff.Summary{Create: 1})

	require.Contains(t, output, "[/markup.inserted.diff] = (known after apply)\n    }\n")
	require.True(t, strings.HasSuffix(output, "\nDiff: 1 to create, 0 to update, 0 to replace, 0 to delete, 0 unchanged.\n"))
}

func TestRenderHighlightWithANSIRendererWritesEscapeCodes(t *testing.T) {
	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	output := string(diff.Render(designExampleDiff(), diff.Highlight(renderer)))

	require.Contains(t, output, "\x1b[")
}

func TestRenderHighlightWithANSIRendererStripsBackToPlainOutput(t *testing.T) {
	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	output := string(diff.Render(designExampleDiff(), diff.Highlight(renderer)))

	require.Equal(t, string(diff.Render(designExampleDiff())), sgrCodes.ReplaceAllString(output, ""))
}

func TestRenderHighlightWithThemeWithoutMarkupRulesLeavesMarkersUncoloured(t *testing.T) {
	theme := `{"tokenColors": [{"scope": "comment", "settings": {"fontStyle": "italic"}}]}`
	renderer, err := highlight.NewANSIRenderer(highlight.WithTheme(strings.NewReader(theme)))
	require.NoError(t, err)

	output := string(diff.Render(designExampleDiff(), diff.Highlight(renderer)))

	require.Contains(t, output, "\n      + image   = ")
	require.Contains(t, output, "\n  ~ resource ")
	require.Contains(t, output, "\x1b[3m# resource.container.api will be updated\x1b[0m")
}

func TestRenderHighlightWithNilRendererWritesPlainOutput(t *testing.T) {
	output := string(diff.Render(designExampleDiff(), diff.Highlight(nil)))

	require.Equal(t, string(diff.Render(designExampleDiff())), output)
}

func TestRenderHighlightWritesMaskedSensitiveValueAsPlaceholder(t *testing.T) {
	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	output := string(diff.Render(&diff.Diff{
		Summary: diff.Summary{Update: 1},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("password"), Sensitive: true},
				},
			},
		},
	}, diff.Highlight(renderer)))

	require.Contains(t, output, " = (sensitive value)\n")
}
