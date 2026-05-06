package widget

// registrations holds widget-type registration callbacks contributed by
// per-type packages via init() → MustRegister(...).
var registrations []func(*Registry)

// MustRegister appends a registration callback. Per-type packages call this
// from init() so the main wiring picks them up via RegisterAll.
func MustRegister(fn func(*Registry)) {
	registrations = append(registrations, fn)
}

// RegisterAll runs every registered callback against the given registry.
func RegisterAll(r *Registry) {
	for _, fn := range registrations {
		fn(r)
	}
}
