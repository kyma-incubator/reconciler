package interpreter

import (
	"bufio"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const allowedPackages = "fmt|regexp|net/url|strings|time|strconv"

var regExp = regexp.MustCompile(fmt.Sprintf(`^import\s+"(%s)"$`, allowedPackages))

type GolangInterpreter struct {
	code     string
	bindings map[string]interface{}
}

func NewGolangInterpreter(code string) *GolangInterpreter {
	return &GolangInterpreter{
		code: code,
	}
}

func (gi *GolangInterpreter) WithBindings(bindings map[string]interface{}) *GolangInterpreter {
	if gi.bindings == nil {
		gi.bindings = bindings
	} else {
		for k, v := range gi.bindings {
			gi.bindings[k] = v
		}
	}
	return gi
}

func (gi *GolangInterpreter) Eval() (reflect.Value, error) {
	interp := interp.New(interp.Options{})
	interp.Use(stdlib.Symbols)

	var lastResult reflect.Value
	var err error

	//add bindings to interpreter
	if err := gi.bind(interp, gi.bindings); err != nil {
		return lastResult, err
	}

	//execute the code
	scanner := bufio.NewScanner(strings.NewReader(gi.code))
	for scanner.Scan() {
		line := scanner.Text()

		//block execution of non-whitelisted imports
		if strings.HasPrefix(line, "import") && !regExp.MatchString(line) {
			return lastResult, &BlockedImportError{BlockedImport: line}
		}

		lastResult, err = interp.Eval(line)
		if err != nil {
			return lastResult, fmt.Errorf("Go interpreter failed to execute line '%s':\n%s", line, err.Error())
		}
	}

	return lastResult, err
}

func (gi *GolangInterpreter) EvalBool() (bool, error) {
	value, err := gi.EvalString()
	if err != nil {
		return false, err
	}
	bool, err := strconv.ParseBool(strings.ToLower(value))
	if err == nil {
		return bool, nil
	}
	return false, &NoBooleanResultError{Result: value}
}

func (gi *GolangInterpreter) EvalString() (string, error) {
	value, err := gi.Eval()

	if err != nil {
		return "", err
	}

	switch value.Kind() {
	case reflect.Bool:
		return fmt.Sprintf("%t", value.Bool()), nil
	case reflect.String:
		return value.String(), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func (gi *GolangInterpreter) bind(interp *interp.Interpreter, bindings map[string]interface{}) error {
	if bindings == nil {
		return nil
	}

	var err error
	for k, v := range bindings {
		switch v.(type) {
		case string:
			_, err = interp.Eval(fmt.Sprintf(`var %s string = "%s"`, k, v))
		case bool:
			_, err = interp.Eval(fmt.Sprintf(`var %s bool = %t`, k, v))
		case int:
			_, err = interp.Eval(fmt.Sprintf(`var %s int = %d`, k, v))
		case int64:
			_, err = interp.Eval(fmt.Sprintf(`var %s int64 = %d`, k, v))
		case float32:
			_, err = interp.Eval(fmt.Sprintf(`var %s float32 = %f`, k, v))
		case float64:
			_, err = interp.Eval(fmt.Sprintf(`var %s float64 = %f`, k, v))
		default:
			err = fmt.Errorf("Cannot bind key '%s' because value of type '%T' is not supported", k, v)
		}
	}

	return err
}

type BlockedImportError struct {
	BlockedImport string
}

func (e *BlockedImportError) Error() string {
	return fmt.Sprintf("Blocking import statement '%s': only these packages are allowed '%s'", e.BlockedImport, allowedPackages)
}

func IsBlockedImportError(err error) bool {
	return reflect.TypeOf(err) == reflect.TypeOf(&BlockedImportError{})
}

type NoBooleanResultError struct {
	Result interface{}
}

func (e *NoBooleanResultError) Error() string {
	return fmt.Sprintf("Result '%v' cannot be casted to boolean", e.Result)
}

func IsNoBooleanResultError(err error) bool {
	return reflect.TypeOf(err) == reflect.TypeOf(&NoBooleanResultError{})
}
