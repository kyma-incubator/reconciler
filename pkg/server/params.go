package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
)

type Params struct {
	urlQuery url.Values
	params   map[string]string
}

func NewParams(r *http.Request) *Params {
	params := mux.Vars(r)
	var query url.Values

	if r.URL != nil {
		query = r.URL.Query()
	}

	return &Params{
		urlQuery: query,
		params:   params,
	}
}

func (p *Params) String(name string) (string, error) {
	result, ok := p.params[name]
	if !ok {
		if p.queryHas(name) {
			return p.urlQuery.Get(name), nil
		}
		return "", p.newUndefinedErr(name)
	}
	return result, nil
}

func (p *Params) Int(name string) (int, error) {
	result, err := p.String(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(result)
}

func (p *Params) Int64(name string) (int64, error) {
	result, err := p.String(name)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(result, 10, 64)
}

func (p *Params) StrSlice(name string) ([]string, error) {
	if p.queryHas(name) {
		return p.urlQuery[name], nil
	}
	return nil, p.newUndefinedErr(name)
}

func (p *Params) newUndefinedErr(name string) error {
	return fmt.Errorf("parameter '%s' undefined", name)
}

func (p *Params) queryHas(name string) bool {
	_, ok := p.urlQuery[name]
	return ok
}
