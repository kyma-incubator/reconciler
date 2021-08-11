package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

const tblCluster string = "inventory_clusters"

type ClusterEntity struct {
	Version    int64     `db:"readOnly"`
	Cluster    string    `db:"notNull"`
	Runtime    string    `db:"notNull"`
	Metadata   string    `db:"notNull"`
	Kubeconfig string    `db:"notNull"`
	Contract   int64     `db:"notNull"`
	Deleted    bool      `db:"notNull"`
	Created    time.Time `db:"readOnly"`
}

func (c *ClusterEntity) String() string {
	return fmt.Sprintf("ClusterEntity [Cluster=%s,Version=%d]",
		c.Cluster, c.Version)
}

func (c *ClusterEntity) New() db.DatabaseEntity {
	return &ClusterEntity{}
}

func (c *ClusterEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterEntity) Table() string {
	return tblCluster
}

func (c *ClusterEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster &&
			c.Runtime == otherClProp.Runtime &&
			c.Metadata == otherClProp.Metadata &&
			c.Contract == otherClProp.Contract &&
			c.Kubeconfig == otherClProp.Kubeconfig
	}
	return false
}

func (c *ClusterEntity) GetRuntime() (*keb.RuntimeInput, error) {
	if c.Runtime == "" {
		return &keb.RuntimeInput{}, nil
	}
	return keb.NewModelFactory(c.Contract).Runtime([]byte(c.Runtime))
}

func (c *ClusterEntity) GetMetadata() (*keb.Metadata, error) {
	if c.Metadata == "" {
		return &keb.Metadata{}, nil
	}
	return keb.NewModelFactory(c.Contract).Metadata([]byte(c.Metadata))
}
