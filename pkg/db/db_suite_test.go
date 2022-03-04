package db

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

//goland:noinspection ALL
type DbTestSuite struct{ *ContainerTestSuite }

func TestDbSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &DbTestSuite{SharedContainerTestSuite(
		t, true, Default,
	)})
}
