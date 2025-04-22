package regofunc

import (
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/rego"
)

type RegoFunc func(*rego.Rego)

var (
	muRegistry       sync.RWMutex
	regoFuncRegistry = make(map[string]RegoFunc)
)

// Register an extension function.
func Register(name string, fn RegoFunc) {
	if fn == nil {
		panic(fmt.Errorf("cannot register nil Rego function: %s", name))
	}

	if _, registered := regoFuncRegistry[name]; !registered {
		regoFuncRegistry[name] = fn
	}
}

// List returns all registered extension functions.
func List() []RegoFunc {
	list := make([]RegoFunc, 0)
	muRegistry.RLock()
	for _, fn := range regoFuncRegistry {
		list = append(list, fn)
	}
	muRegistry.RUnlock()
	return list
}
