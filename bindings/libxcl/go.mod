// libxcl is the shared library a language binding loads to drive the xcl
// engine: four C functions over the core service. It is a module of its own
// because it imports "C", and a cgo package has no buildable files when
// CGO_ENABLED=0; keeping it out of the root module keeps the library's
// go build ./... free of any C toolchain. It is released at the same tag as
// the root module, which it requires at that tag; during development the
// replace below builds it from the checkout.
module github.com/jumppad-labs/xcl/bindings/libxcl

go 1.25.0

require (
	github.com/jumppad-labs/xcl v0.0.0
	github.com/stretchr/testify v1.10.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.7.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-hclog v0.14.1 // indirect
	github.com/hashicorp/go-plugin v1.6.3 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/infinytum/raymond/v2 v2.0.5 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/jumppad-labs/xcl => ../..
