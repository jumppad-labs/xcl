package registry

// TypeRegistrar is a registry block types can be declared on, such as Local
// and GitHub.
type TypeRegistrar interface {
	RegisterType(prototype any, name ...string)
}

// SingletonRegistrar is a registry block types written without labels can be
// declared on, such as Local and GitHub.
type SingletonRegistrar interface {
	RegisterSingleton(prototype any, name string)
}

// Register declares the Go type T as a block type on r, as RegisterType does,
// without a value to stand for it:
//
//	registry.Register[Database](local, "database")             // database "main" {}
//	registry.Register[Database](local, "resource", "database") // resource "database" "main" {}
//
// T is the struct itself, not a pointer to it. It panics on the same
// declarations RegisterType panics on, and on a T that is not a struct.
//
// On Go 1.27 and later the same is a method, local.Register[Database]("database").
func Register[T any](r TypeRegistrar, name ...string) {
	register[T](r, name...)
}

// RegisterSingleton declares the Go type T as a block type written without
// labels on r, as the registry's RegisterSingleton does:
//
//	registry.RegisterSingleton[Runner](local, "runner") // runner {}
//
// T is the struct itself, not a pointer to it. There is no method form, as
// there is for Register: RegisterSingleton is already the name of the method
// taking a prototype.
func RegisterSingleton[T any](r SingletonRegistrar, name string) {
	registerSingleton[T](r, name)
}

// register is the one implementation behind Register and the Register
// methods, so the two spellings cannot drift.
func register[T any](r TypeRegistrar, name ...string) {
	r.RegisterType(new(T), name...)
}

// registerSingleton is the one implementation behind RegisterSingleton and
// its methods.
func registerSingleton[T any](r SingletonRegistrar, name string) {
	r.RegisterSingleton(new(T), name)
}
