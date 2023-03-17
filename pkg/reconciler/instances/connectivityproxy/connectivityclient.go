package connectivityproxy

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"time"
)

type ConnectivityCAClient struct {
	url    string
	client *http.Client
}

func NewConnectivityCAClient(task *reconciler.Task) (*ConnectivityCAClient, error) {
	configs := task.Configuration

	url, ok := configs["global.binding.url"]

	if ok == false {
		return nil, fmt.Errorf("missing configuration value global.binding.url")
	}

	caPath, ok := configs["global.binding.CAs_path"]

	if ok == false {
		return nil, fmt.Errorf("missing configuration value global.binding.CAs_path")
	}

	return &ConnectivityCAClient{
		url: fmt.Sprintf("%v%v", url, caPath),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (clientCA ConnectivityCAClient) GetCA() ([]byte, error) {

	req, err := http.NewRequest("GET", clientCA.url, nil)

	if err != nil {
		return nil, errors.New("cannot create request")
	}

	resp, err := clientCA.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("URL not found: %q", clientCA.url)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error response code %d from: %q", resp.StatusCode, clientCA.url)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bytes, err
}
