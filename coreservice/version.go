package coreservice

// Version is the engine's version, reported by GetVersion. A release build of
// xcl-core or libxcl stamps it with the xcl tag through
// -ldflags "-X github.com/jumppad-labs/xcl/coreservice.Version=v1.2.3", and a
// binding refuses to run against an engine whose version is not its own.
// A development build reports "dev".
var Version = "dev"

// ABIVersion is the version of the C ABI libxcl implements, which also
// covers the frames xcl-core exchanges. It changes only when one of the four
// C functions, a frame field or a status code changes; adding a CoreService
// method or field does not change it.
const ABIVersion uint32 = 1
