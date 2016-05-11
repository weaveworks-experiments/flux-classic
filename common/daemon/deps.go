package daemon

import (
	log "github.com/Sirupsen/logrus"

	"flag"
	"fmt"
	"os"

	"github.com/weaveworks/flux/common/version"
)

type Dependencies struct {
	*flag.FlagSet
	deps    map[DependencyKey]*dependency
	current *dependency
}

type Config interface {
	Populate(*Dependencies)
	Prepare() (StartFunc, error)
}

type DependencySlot interface {
	Key() DependencyKey
	Assign(value interface{})
}

type DependencyKey interface {
	MakeConfig() DependencyConfig
}

type DependencyConfig interface {
	Populate(*Dependencies)
	MakeValue() (interface{}, StartFunc, error)
}

type dependency struct {
	config DependencyConfig
	// the slots to which the value of this dependency gets assigned
	slots []depSlot
}

type depSlot struct {
	slot  DependencySlot
	owner *dependency
}

func (deps *Dependencies) Dependency(slot DependencySlot) {
	key := slot.Key()
	dep, found := deps.deps[key]
	if !found {
		dep = &dependency{config: key.MakeConfig()}
		deps.deps[key] = dep

		old := deps.current
		deps.current = dep
		dep.config.Populate(deps)
		deps.current = old
	}

	dep.slots = append(dep.slots, depSlot{slot, deps.current})
}

// Do a topological sort of depenencies, to give the order in which we
// should MakeValue them.
func (deps *Dependencies) sort() []*dependency {
	// The number of unprocessed dependencies of each dep
	counts := make(map[*dependency]int)
	for _, dep := range deps.deps {
		for _, slot := range dep.slots {
			if slot.owner != nil {
				counts[slot.owner] += 1
			}
		}
	}

	// "ready" dependencies: those with zero counts
	var ready []*dependency
	for _, dep := range deps.deps {
		if counts[dep] == 0 {
			ready = append(ready, dep)
		}
	}

	var res []*dependency
	for len(ready) != 0 {
		// pop the last ready dep
		dep := ready[len(ready)-1]
		ready = ready[:len(ready)-1]

		// process it, decrementing counts and identifying
		// newly ready deps
		res = append(res, dep)
		for _, slot := range dep.slots {
			if slot.owner != nil {
				count := counts[slot.owner] - 1
				counts[slot.owner] = count
				if count == 0 {
					ready = append(ready, slot.owner)
				}
			}
		}
	}

	for _, count := range counts {
		if count != 0 {
			panic("Circular dependencies")
		}
	}

	return res
}

func Main(configs ...Config) {
	log.Println(version.Banner())

	sfs, err := configsToStartFuncs(configs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	run(Aggregate(sfs...))
}

func configsToStartFuncs(configs []Config) ([]StartFunc, error) {
	deps := &Dependencies{
		FlagSet: flag.NewFlagSet(os.Args[0], flag.ContinueOnError),
		deps:    make(map[DependencyKey]*dependency),
	}

	for _, c := range configs {
		c.Populate(deps)
	}

	if err := deps.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	if deps.NArg() > 0 {
		return nil, fmt.Errorf("excess command line arguments")
	}

	// Fill slots
	var startFuncs []StartFunc
	for _, dep := range deps.sort() {
		val, startFunc, err := dep.config.MakeValue()
		if err != nil {
			return nil, err
		}

		if startFunc != nil {
			startFuncs = append(startFuncs, startFunc)
		}

		for _, s := range dep.slots {
			s.slot.Assign(val)
		}
	}

	for _, c := range configs {
		startFunc, err := c.Prepare()
		if err != nil {
			return nil, err
		}

		startFuncs = append(startFuncs, startFunc)
	}

	return startFuncs, nil
}
