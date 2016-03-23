package daemon

import (
	"flag"
	"fmt"
	"os"
)

type Dependencies struct {
	*flag.FlagSet
	slots map[DependencyKey]depSlots
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
	MakeValue() (interface{}, error)
}

type depSlots struct {
	config DependencyConfig
	slots  []DependencySlot
}

func (deps *Dependencies) Dependency(slot DependencySlot) {
	key := slot.Key()
	slots, found := deps.slots[key]
	if !found {
		slots.config = key.MakeConfig()
		slots.config.Populate(deps)
	}

	slots.slots = append(slots.slots, slot)
	deps.slots[key] = slots
}

func ConfigsMain(configs ...Config) {
	sfs, err := configsToStartFuncs(configs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	Main(Aggregate(sfs...))
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

	// Make dependency values, and assign them to slots
	for _, slots := range deps.slots {
		val, err := slots.config.MakeValue()
		if err != nil {
			return nil, err
		}

		for _, d := range slots.slots {
			d.Assign(val)
		}
	}

	var res []StartFunc
	for _, c := range configs {
		sf, err := c.Prepare()
		if err != nil {
			return nil, err
		}

		res = append(res, sf)
	}

	return res, nil
}
