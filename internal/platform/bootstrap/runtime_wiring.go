package bootstrap

import "fmt"

var runtimeWiring func()

func SetRuntimeWiring(fn func()) {
	runtimeWiring = fn
}

func applyRuntimeWiring(component string) error {
	if runtimeWiring == nil {
		return fmt.Errorf("%s runtime wiring not configured", component)
	}
	runtimeWiring()
	return nil
}
