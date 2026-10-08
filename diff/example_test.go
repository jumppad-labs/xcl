package diff_test

import (
	"fmt"

	"github.com/jumppad-labs/xcl/diff"
)

func ExampleRender() {
	d := &diff.Diff{
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

	fmt.Print(string(diff.Render(d)))
	// Output:
	//   # resource.container.api will be updated
	//   ~ resource "container" "api" {
	//       ~ ports[0].host      = 8080 -> 9090
	//       + ports[2]           = { host = 443, local = 8443 }
	//       ~ env["DB_PASSWORD"] = (sensitive value)
	//     }
	//
	//   # resource.container.cache changed outside xcl and will be updated
	//   ~ resource "container" "cache" {}
	//
	//   # resource.container.web will be created
	//   + resource "container" "web" {
	//       + image   = "nginx"
	//       + db_host = (known after apply)
	//     }
	//
	//   # resource.network.app will be replaced, its last apply failed
	// -/+ resource "network" "app" {}
	//
	//   # resource.postgres.old will be deleted
	//   - resource "postgres" "old" {}
	//
	// Diff: 1 to create, 2 to update, 1 to replace, 1 to delete, 4 unchanged.
}
