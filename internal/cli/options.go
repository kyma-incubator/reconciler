package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/config"
	"github.com/kyma-incubator/reconciler/pkg/db"
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
	repository        *config.KeyValueRepository
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

func (o *Options) Repository() *config.KeyValueRepository {
	if o.repository != nil {
		return o.repository
	}
	return o.initRepository()
}

func (o *Options) initRepository() *config.KeyValueRepository {
	var err error

	repoMutex.Lock()
	if o.connectionFactory == nil {
		o.Logger().Error("Failed to create configuration entry repository because connection factory is undefined")
	}
	o.repository, err = config.NewKeyValueRepository(o.connectionFactory, o.Verbose)
	if err != nil {
		o.Logger().Error(fmt.Sprintf("Failed to create configuration entry repository: %s", err))
	}
	repoMutex.Unlock()

	return o.repository
}

func (o *Options) Validate() error {
	for _, supportedFormat := range SupportedOutputFormats {
		if supportedFormat == o.OutputFormat {
			return nil
		}
	}
	return fmt.Errorf("Output format '%s' not supported - choose between '%s'", o.OutputFormat, strings.Join(SupportedOutputFormats, "', '"))
}
