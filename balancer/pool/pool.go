package pool

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/balancer/model"
)

const (
	retry_initial_interval  = 1
	retry_backoff_factor    = 4
	retry_abandon_threshold = 256 // ~4min
)

type InstancePool interface {
	ReactivateRetries(t time.Time)
	UpdateInstances(instances []model.Instance)
	PickActiveInstance() PooledInstance
	PickInstance() PooledInstance
}

type PooledInstance interface {
	Instance() *model.Instance
	Keep()
	Fail()
}

type poolEntry struct {
	instance      *model.Instance
	pool          *instancePool
	retryInterval int
}

type retryEntry struct {
	*poolEntry
	retryTime time.Time
}

type retryQueue struct {
	retries []*retryEntry
}

type instancePool struct {
	members map[string]struct{}
	active  []*poolEntry
	retry   *retryQueue
	lock    sync.Mutex
}

func NewInstancePool() InstancePool {
	pool := &instancePool{
		members: make(map[string]struct{}),
		retry:   &retryQueue{},
	}
	heap.Init(pool.retry)
	return pool
}

// Make any instances that are due for a retry available again
func (p *instancePool) ReactivateRetries(t time.Time) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for p.retry.beforeOrAt(t) {
		entry := p.retry.take1()
		if entry.retryInterval < retry_abandon_threshold {
			log.Infof("Giving instance %s another chance", entry.instance.Name)
			entry.retryInterval *= retry_backoff_factor
			p.active = append(p.active, entry)
		} else {
			delete(p.members, entry.instance.Name)
			log.Infof("Abandoning instance %s after %d retries",
				entry.instance.Name,
				1+int(math.Log(float64(entry.retryInterval))/
					math.Log(float64(retry_backoff_factor))))
		}
	}
}

func (p *instancePool) UpdateInstances(instances []model.Instance) {
	p.lock.Lock()
	defer p.lock.Unlock()
	newActive := []*poolEntry{}
	remainder := p.members
	p.members = map[string]struct{}{}

	for i, inst := range instances {
		p.members[inst.Name] = struct{}{}
		if _, found := remainder[inst.Name]; found {
			delete(remainder, inst.Name)
		} else {
			newActive = append(newActive, &poolEntry{
				pool:          p,
				instance:      &instances[i],
				retryInterval: retry_initial_interval,
			})
		}
	}
	p.removeMembers(remainder)
	p.active = append(p.active, newActive...)
}

func (p *instancePool) removeMembers(names map[string]struct{}) {
	newActive := []*poolEntry{}
	for _, entry := range p.active {
		if _, found := names[entry.instance.Name]; !found {
			newActive = append(newActive, entry)
		}
	}
	p.active = newActive

	newRetries := []*retryEntry{}
	for _, entry := range p.retry.retries {
		if _, found := names[entry.instance.Name]; !found {
			newRetries = append(newRetries, entry)
		}
	}
	p.retry.retries = newRetries
	heap.Init(p.retry)
}

// Pick an instance from amongst the active instances; return nil if
// there are none.
func (p *instancePool) PickActiveInstance() PooledInstance {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.pickActiveInstance()
}

func (p *instancePool) pickActiveInstance() PooledInstance {
	n := len(p.active)
	if n > 0 {
		return p.active[rand.Intn(n)]
	}
	return nil
}

// Pick an instance from the pool; ideally, from amongst the active
// instances, but failing that, from those waiting to be retried.
func (p *instancePool) PickInstance() PooledInstance {
	p.lock.Lock()
	defer p.lock.Unlock()

	// NB it is an invariant that the instance returned must be
	// present in the set of active instances, so that if `Keep` is
	// called, it does not need to be (conditionally) moved.
	inst := p.pickActiveInstance()
	if inst != nil {
		return inst
	}
	// Ruh-roh, no active instances. Raid the retry queue.
	if p.retry.Len() > 0 {
		entry := p.retry.take1()
		p.active = []*poolEntry{entry}
		return entry
	}
	return nil
}

func (entry *poolEntry) Keep() {
	entry.retryInterval = retry_initial_interval
}

func (entry *poolEntry) Fail() {
	log.Infof("Scheduling instance %s for retry in %d sec", entry.instance.Name, entry.retryInterval)
	p := entry.pool

	p.lock.Lock()
	defer p.lock.Unlock()

	for i, e := range p.active {
		if e == entry {
			p.active = append(p.active[0:i], p.active[i+1:]...)
			p.retry.scheduleRetry(entry, time.Now().Add(time.Duration(entry.retryInterval)*time.Second))
			return
		}
	}
}

func (entry *poolEntry) Instance() *model.Instance {
	return entry.instance
}

// =====

// heap.Interface
func (q *retryQueue) Len() int {
	return len(q.retries)
}

func (q *retryQueue) Less(i, j int) bool {
	return q.retries[i].retryTime.Before(q.retries[j].retryTime)
}

func (q *retryQueue) Swap(i, j int) {
	q.retries[i], q.retries[j] = q.retries[j], q.retries[i]
}

func (q *retryQueue) Push(r interface{}) {
	q.retries = append(q.retries, r.(*retryEntry))
}

func (q *retryQueue) Pop() interface{} {
	last := len(q.retries) - 1
	r := q.retries[last]
	q.retries = q.retries[0:last]
	return r
}

// End heap.Interface

func (q *retryQueue) beforeOrAt(t time.Time) bool {
	if len(q.retries) == 0 {
		return false
	}
	return !q.retries[len(q.retries)-1].retryTime.After(t)
}

func (q *retryQueue) take1() *poolEntry {
	return heap.Pop(q).(*retryEntry).poolEntry
}

func (q *retryQueue) scheduleRetry(entry *poolEntry, t time.Time) {
	r := &retryEntry{entry, t}
	heap.Push(q, r)
}
