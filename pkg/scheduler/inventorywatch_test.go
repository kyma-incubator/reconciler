package scheduler

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/stretchr/testify/require"
)

func TestInventoryWatch(t *testing.T) {
	inventory := &cluster.MockInventory{}
	inventoryWatch, err := NewInventoryWatch(inventory, nil)
	require.NoError(t, err)
	_ = inventoryWatch
}
