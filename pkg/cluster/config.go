package cluster

type Configuration struct {
}

type Configurer struct {
}

func (c *Configurer) Get(cluster string) *Configuration {
	panic("not implemented yet")
}
