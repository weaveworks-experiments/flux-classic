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
	slots map[DependencyKey]depSlots

	// The dependency graph between DependencyConfigs is implicit.
	// So we have to record the order in which they were created,
	// in order to call MakeValue on them in the reverse order.
	keysOrder []DependencyKey
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

type depSlots struct {
	config DependencyConfig
	slots  []DependencySlot
}

func (deps *Dependencies) Dependency(slot DependencySlot) {
	key := slot.Key()
	slots, found := deps.slots[key]
	if !found {
		deps.keysOrder = append(deps.keysOrder, key)
		slots.config = key.MakeConfig()
		slots.config.Populate(deps)
	}

	slots.slots = append(slots.slots, slot)
	deps.slots[key] = slots
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
		slots:   make(map[DependencyKey]depSlots),
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

	var res []StartFunc

	// Make dependency values, and assign them to slots
	for i := len(deps.keysOrder) - 1; i >= 0; i-- {
		slots := deps.slots[deps.keysOrder[i]]

		val, startFunc, err := slots.config.MakeValue()
		if err != nil {
			return nil, err
		}

		if startFunc != nil {
			res = append(res, startFunc)
		}

		for _, d := range slots.slots {
			d.Assign(val)
		}
	}

	for _, c := range configs {
		startFunc, err := c.Prepare()
		if err != nil {
			return nil, err
		}

		res = append(res, startFunc)
	}

	return res, nil
}
