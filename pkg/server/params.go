package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type Params struct {
	params map[string]string
}

func NewParams(r *http.Request) *Params {
	params := mux.Vars(r)
	query := r.URL.Query()
	for k := range query {
		params[k] = query.Get(k)
	}
	return &Params{
		params: params,
	}
}

func GetArray(r *http.Request, key string) *[]string {
	query := r.URL.Query()
	value := query[key]
	if len(value) == 0 {
		return nil
	}
	return &value
}
func (p *Params) String(name string) (string, error) {
	result, ok := p.params[name]
	if !ok {
		return "", fmt.Errorf("parameter '%s' undefined", name)
	}
	return result, nil
}

func (p *Params) Int64(name string) (int64, error) {
	strResult, err := p.String(name)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strResult, 10, 64)
}
