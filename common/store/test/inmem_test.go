package test

import (
	"testing"

	"github.com/squaremo/ambergreen/common/store"
)

// Test the in-memory mock Store

type testableInMemStore struct {
	store.Store
}

func (tims *testableInMemStore) Reset(t *testing.T) {
	tims.Store = store.NewInMemStore()
}

func TestInMemStore(t *testing.T) {
	RunStoreTestSuite(&testableInMemStore{}, t)
}
