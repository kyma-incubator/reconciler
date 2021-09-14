package keb

type Cluster struct {
	Cluster      string       `json:"runtimeID"`
	RuntimeInput RuntimeInput `json:"runtimeInput"`
	KymaConfig   KymaConfig   `json:"kymaConfig"`
	Metadata     Metadata     `json:"metadata"`
	Kubeconfig   string       `json:"kubeconfig"`
}

type RuntimeInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Configuration struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Secret bool        `json:"secret"`
}

type Component struct {
	Component     string          `json:"component"`
	Namespace     string          `json:"namespace"`
	Configuration []Configuration `json:"configuration"`
}

type KymaConfig struct {
	Version        string      `json:"version"`
	Profile        string      `json:"profile"`
	Components     []Component `json:"components"`
	Administrators []string    `json:"administrators"`
}

type Metadata struct {
	GlobalAccountID string `json:"globalAccountID"`
	SubAccountID    string `json:"subAccountID"`
	ServiceID       string `json:"serviceID"`
	ServicePlanID   string `json:"servicePlanID"`
	ShootName       string `json:"shootName"`
	InstanceID      string `json:"instanceID"`
}
