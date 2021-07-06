package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var loggerMutex sync.Mutex
var repoMutex sync.Mutex

type Options struct {
	Verbose           bool
	NonInteractive    bool
	OutputFormat      string
	connectionFactory db.ConnectionFactory
	logger            *zap.Logger
	repository        *kv.Repository
	inventory         cluster.Inventory
}

func (o *Options) Init(dbConnFact db.ConnectionFactory) {
	o.connectionFactory = dbConnFact
}

func (o *Options) Close() error {
	if o.repository != nil {
		return o.repository.Close()
	}
	return nil
}

func (o *Options) String() string {
	return fmt.Sprintf("CLI options: verbose=%t non-interactive=%t dbConnFactory=%t",
		o.Verbose, o.NonInteractive, (o.logger != nil))
}

func (o *Options) Logger() *zap.Logger {
	if o.logger != nil {
		return o.logger
	}
	return o.initLogger()
}

func (o *Options) initLogger() *zap.Logger {
	var err error

	loggerMutex.Lock()
	if o.Verbose {
		o.logger, err = zap.NewDevelopment()
	} else {
		cfg := zap.Config{
			Encoding:         "console",
			Level:            zap.NewAtomicLevelAt(zapcore.WarnLevel),
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
			EncoderConfig: zapcore.EncoderConfig{
				MessageKey:   "message",
				LevelKey:     "level",
				EncodeLevel:  zapcore.CapitalLevelEncoder,
				TimeKey:      "time",
				EncodeTime:   zapcore.ISO8601TimeEncoder,
				CallerKey:    "caller",
				EncodeCaller: zapcore.ShortCallerEncoder,
			},
		}
		o.logger, err = cfg.Build()
	}
	if err != nil {
		panic(err)
	}
	loggerMutex.Unlock()

	return o.logger
}

func (o *Options) Repository() *kv.Repository {
	if o.repository != nil {
		return o.repository
	}
	return o.initRepository()
}

func (o *Options) initRepository() *kv.Repository {
	var err error

	repoMutex.Lock()
	if o.connectionFactory == nil {
		o.Logger().Fatal("Failed to create configuration entry repository because connection factory is undefined")
	}
	o.repository, err = kv.NewRepository(o.connectionFactory, o.Verbose)
	if err != nil {
		o.Logger().Fatal(fmt.Sprintf("Failed to create configuration entry repository: %s", err))
	}
	repoMutex.Unlock()

	return o.repository
}

func (o *Options) Inventory() cluster.Inventory {
	if o.inventory != nil {
		return o.inventory
	}
	return o.initInventory()
}

func (o *Options) initInventory() cluster.Inventory {
	var err error

	repoMutex.Lock()
	if o.connectionFactory == nil {
		o.Logger().Fatal("Failed to create cluster inventory because connection factory is undefined")
	}
	o.inventory, err = cluster.NewInventory(o.connectionFactory, o.Verbose)
	if err != nil {
		o.Logger().Fatal(fmt.Sprintf("Failed to create cluster inventory: %s", err))
	}
	repoMutex.Unlock()

	return o.inventory
}

func (o *Options) Validate() error {
	for _, supportedFormat := range SupportedOutputFormats {
		if supportedFormat == o.OutputFormat {
			return nil
		}
	}
	return fmt.Errorf("Output format '%s' not supported - choose between '%s'", o.OutputFormat, strings.Join(SupportedOutputFormats, "', '"))
}
