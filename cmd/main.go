package main

import (
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/gorilla/mux"
	cmd "github.com/kyma-incubator/reconciler/cmd/config"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/vrischmann/envconfig"
)

type config struct {
	Address string `envconfig:"default=127.0.0.1:4000"`
}

func main() {
	command := cmd.NewCmd(&cli.Options{})
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}

	cfg := config{}
	err := envconfig.InitWithPrefix(&cfg, "APP")
	if err != nil {
		fmt.Print(err)
	}
	router := mux.NewRouter()
	router.HandleFunc("/clusters", registerNewCluster).Methods("POST")
	router.HandleFunc("/clusters/{runtimeId}", deleteCluster).Methods("DELETE")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	// TODO
	//go func() {
	//	defer wg.Done()
	if err := http.ListenAndServe(cfg.Address, router); err != nil {
		//log.Errorf("Error starting server: %s", err.Error())
		fmt.Print("Error starting server: %s")
		fmt.Print(err.Error())
	}
	//}()
}

func registerNewCluster(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var clusterRequest model.Cluster
	json.Unmarshal(reqBody, &clusterRequest)

	configDir := path.Join("configs")
	connFac, err := db.NewConnectionFactory(path.Join(configDir, "reconciler.yaml"), "configManagement")
	if err != nil {
		fmt.Print(err)
	}
	ceRepo, err := cluster.NewRepository(connFac, true)
	if err != nil {
		fmt.Print(err)
	}
	err = ceRepo.Add(&clusterRequest)
	if err != nil {
		fmt.Print(err)
	}
}

func deleteCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runtimeId := vars["runtimeId"]

	configDir := path.Join("configs")
	connFac, err := db.NewConnectionFactory(path.Join(configDir, "reconciler.yaml"), "configManagement")
	if err != nil {
		fmt.Print(err)
	}
	ceRepo, err := cluster.NewRepository(connFac, true)
	if err != nil {
		fmt.Print(err)
	}
	err = ceRepo.Delete(runtimeId)
	if err != nil {
		fmt.Print(err)
	}

}
