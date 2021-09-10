package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

const tblConfiguration string = "inventory_cluster_configs"

type ClusterConfigurationEntity struct {
	Version        int64  `db:"readOnly"`
	Cluster        string `db:"notNull"`
	ClusterVersion int64  `db:"notNull"`
	KymaVersion    string `db:"notNull"`
	KymaProfile    string `db:"notNull"`
	Components     string `db:"notNull,encrypt"`
	Administrators string
	Contract       int64     `db:"notNull"`
	Deleted        bool      `db:"notNull"`
	Created        time.Time `db:"readOnly"`
}

func (c *ClusterConfigurationEntity) String() string {
	return fmt.Sprintf("ClusterConfigurationEntity [Version=%d,Cluster=%s,ClusterVersion=%d]",
		c.Version, c.Cluster, c.ClusterVersion)
}

func (c *ClusterConfigurationEntity) New() db.DatabaseEntity {
	return &ClusterConfigurationEntity{}
}

func (c *ClusterConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterConfigurationEntity) Table() string {
	return tblConfiguration
}

func (c *ClusterConfigurationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterConfigurationEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster &&
			c.ClusterVersion == otherClProp.ClusterVersion &&
			c.KymaVersion == otherClProp.KymaVersion &&
			c.KymaProfile == otherClProp.KymaProfile &&
			c.Components == otherClProp.Components &&
			c.Administrators == otherClProp.Administrators &&
			c.Contract == otherClProp.Contract
	}
	return false
}

func (c *ClusterConfigurationEntity) GetComponents() ([]*keb.Components, error) {
	if c.Components == "" {
		return []*keb.Components{}, nil
	}
	return keb.NewModelFactory(c.Contract).Components([]byte(c.Components))
}

func (c *ClusterConfigurationEntity) GetAdministrators() ([]string, error) {
	if c.Administrators == "" {
		return []string{}, nil
	}
	return keb.NewModelFactory(c.Contract).Administrators([]byte(c.Administrators))
}
