package db

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type DbTestSuite struct{ *ContainerTestSuite }

func TestDbSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &DbTestSuite{LeaseSharedContainerTestSuite(
		t, DefaultSharedContainerSettings, true, false,
	)})
	ReturnLeasedSharedContainerTestSuite(t, DefaultSharedContainerSettings)
}
