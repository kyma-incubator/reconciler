package mock

import (
	"errors"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"math/rand"
	"time"
)

// CustomAction for mock component reconciliation.
type CustomAction struct {
	name      string
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

// MockedActionGenerator for generating MockedActionExecution.
type MockedActionGenerator interface {
	Generate() MockedActionExecution
}

// RandomMockedActionGenerator provides an implementation of MockedActionGenerator that returns random action.
type RandomMockedActionGenerator struct{}

func (c *RandomMockedActionGenerator) Generate() MockedActionExecution {
	rand.Seed(time.Now().UnixNano())

	//nolint:gosec // Not security relevant
	generatedNumber := rand.Intn(100)

	if generatedNumber < 2 {
		return &Fail{}
	} else if generatedNumber < 4 {
		return &Timeout{}
	} else if generatedNumber < 5 {
		return &SleepAndFail{}
	} else if generatedNumber < 7 {
		return &SleepAndSuccess{}
	} else {
		return &Success{}
	}
}

// MockedActionExecution defines how mocked action should execute.
type MockedActionExecution interface {
	Execute(context *service.ActionContext) error
}

type Success struct{}

func (c *Success) Execute(context *service.ActionContext) error {
	context.Logger.Infof("Success action execution")
	context.Logger.Infof("Sleeping for 1 second...")
	time.Sleep(1 * time.Second)
	return nil
}

type SleepAndSuccess struct{}

func (c *SleepAndSuccess) Execute(context *service.ActionContext) error {
	context.Logger.Infof("SleepAndSuccess action execution")
	context.Logger.Infof("Sleeping for 1 minutes...")
	time.Sleep(1 * time.Minute)
	return nil
}

type Fail struct{}

func (c *Fail) Execute(context *service.ActionContext) error {
	context.Logger.Infof("Fail action execution")
	return errors.New("error from Fail action")
}

type SleepAndFail struct{}

func (c *SleepAndFail) Execute(context *service.ActionContext) error {
	context.Logger.Infof("SleepAndFail action execution")
	context.Logger.Infof("Sleeping for 1 minute...")
	time.Sleep(1 * time.Minute)
	return errors.New("error from SleepAndFail action")
}

type Timeout struct{}

func (c *Timeout) Execute(context *service.ActionContext) error {
	context.Logger.Infof("Timeout action execution")
	context.Logger.Infof("Sleeping for 11 minutes...")
	time.Sleep(11 * time.Minute)
	return nil
}
