package connectivityclient

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

//go:generate mockery --name=ConnectivityClient --output=mocks --case=underscore
type ConnectivityClient interface {
	GetCA() ([]byte, error)
}

type ConnectivityCAClient struct {
	url    string
	client *http.Client
}

func NewConnectivityCAClient(configs map[string]interface{}) (*ConnectivityCAClient, error) {

	if configs == nil {
		return nil, fmt.Errorf("missing configuration")
	}

	url := configs["global.binding.url"]

	if url == nil || url == "" {
		return nil, fmt.Errorf("missing configuration value global.binding.url")
	}

	caPath := configs["global.binding.CAs_path"]

	if caPath == nil || caPath == "" {
		return nil, fmt.Errorf("missing configuration value global.binding.CAs_path")
	}

	return &ConnectivityCAClient{
		url: fmt.Sprintf("%v%v", url, caPath),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (clientCA *ConnectivityCAClient) GetCA() ([]byte, error) {

	req, err := http.NewRequest("GET", clientCA.url, nil)

	if err != nil {
		return nil, errors.New("cannot create request")
	}

	resp, err := clientCA.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error response code %d from: %q", resp.StatusCode, clientCA.url)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(bytes) == 0 {
		return nil, errors.New("empty CA root string read")
	}

	return bytes, err
}
