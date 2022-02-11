package mock

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"math/rand"
	"time"
)

const (
	sleepTime = 4 * time.Minute
)

// CustomAction for mock component reconciliation.
type CustomAction struct {
	name string
	generator MockedActionGenerator
}

func (a *CustomAction) Run(context *service.ActionContext) error {
	context.Logger.Infof("Action '%s' started (passed version was '%s')", a.name, context.Task.Version)

	generatedAction := a.generator.Generate()
	err := generatedAction.Execute(context)
	if err != nil {
		return err
	}

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)

	return nil
}

type MockedActionGenerator interface {
	Generate() MockedActionExecution
}

type RandomMockedActionGenerator struct {}

func (c *RandomMockedActionGenerator) Generate() MockedActionExecution {
	rand.Seed(time.Now().UnixNano())

	generatedNumber := rand.Intn(10)

	if generatedNumber < 2 {
		return &Fail{}
	} else if generatedNumber < 4 {
		return &Timeout{}
	}  else if generatedNumber < 5 {
		return &SleepAndFail{}
	}  else if generatedNumber < 7 {
		return &SleepAndSuccess{}
	} else {
		return &Success{}
	}
}

type MockedActionExecution interface {
	Execute(context *service.ActionContext) error
}

type Success struct {}

func (c *Success) Execute(context *service.ActionContext) error {
	return nil
}

type SleepAndSuccess struct {}

func (c *SleepAndSuccess) Execute(context *service.ActionContext) error {
	return nil
}

type Fail struct {}

func (c *Fail) Execute(context *service.ActionContext) error {
	return nil
}

type SleepAndFail struct {}

func (c *SleepAndFail) Execute(context *service.ActionContext) error {
	return nil
}

type Timeout struct {}

func (c *Timeout) Execute(context *service.ActionContext) error {
	return nil
}


