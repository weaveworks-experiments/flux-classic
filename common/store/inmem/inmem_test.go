package inmem

import (
	"testing"

	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/test"
)

// Test the in-memory mock Store

type testableInMemStore struct {
	store.Store
}

func (tims *testableInMemStore) Reset(t *testing.T) {
	tims.Store = NewInMem().Store("test session")
}

func TestInMemStore(t *testing.T) {
	test.RunStoreTestSuite(&testableInMemStore{}, t)
}
