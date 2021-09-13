package db

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Validator struct {
	blockQueries bool
	logger       *zap.SugaredLogger
}

func NewValidator(blockQueries bool, logger *zap.SugaredLogger) *Validator {
	return &Validator{blockQueries, logger}
}

func (v *Validator) Validate(query string) error {
	matchSelect, err := regexp.MatchString("SELECT.*(FROM\\s*\\w+\\s*)+(WHERE (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)*)?(\\w*\\s+IN\\s+[^;]+)?(\\w*\\s+ORDER BY\\s+[^;]+)?(\\w*\\s+GROUP BY\\s+[^;]+)?$", query)
	if err != nil {
		return errors.Wrap(err, "Regex validation failed")
	}
	matchInsert, err := regexp.MatchString("INSERT.*VALUES \\((\\$\\d+)(\\s*,\\s*\\$\\d+)*\\)[^;]+$", query)
	if err != nil {
		return errors.Wrap(err, "Regex validation failed")
	}
	matchUpdate, err := regexp.MatchString("UPDATE.*SET (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?)+(\\s*WHERE\\s*(\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)+)?$", query)
	if err != nil {
		return errors.Wrap(err, "Regex validation failed")
	}
	matchDelete, err := regexp.MatchString("DELETE FROM.*WHERE (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)*(\\w*\\s+IN\\s+[^;]+)?$", query)
	if err != nil {
		return errors.Wrap(err, "Regex validation failed")
	}

	matchOthers := strings.Contains(query, "CREATE TABLE") || strings.Contains(query, "SHOW TRANSACTION")

	if !matchSelect && !matchInsert && !matchUpdate && !matchDelete && !matchOthers {
		msg := fmt.Sprintf("Found potential SQL injection for query: %s", query)
		if v.blockQueries {
			return errors.New(msg)
		}
		v.logger.Warn(msg)
	}

	return nil
}
