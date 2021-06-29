package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"

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
	router.HandleFunc("/clusters/{clusterId}", deleteCluster).Methods("DELETE")
	router.HandleFunc("/clusters/{clusterId}/status", getClusterStatus).Methods("GET")

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
	ceRepo, err := cluster.NewInventory(connFac, true)
	if err != nil {
		fmt.Print(err)
	}
	clusterEntity, err := ceRepo.Add(&clusterRequest)
	if err != nil {
		fmt.Print(err)
	}
	url := fmt.Sprintf("%s%s/%d/%s", r.Host, r.URL.RequestURI(), clusterEntity.ID, "status")
	json.NewEncoder(w).Encode(url)
}

func getClusterStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterId, err := strconv.Atoi(vars["clusterId"])
	if err != nil {
		fmt.Print(err)
	}

	configDir := path.Join("configs")
	connFac, err := db.NewConnectionFactory(path.Join(configDir, "reconciler.yaml"), "configManagement")
	if err != nil {
		fmt.Print(err)
	}
	ceRepo, err := cluster.NewRepository(connFac, true)
	if err != nil {
		fmt.Print(err)
	}
	status, err := ceRepo.GetClusterStatus(clusterId)
	if err != nil {
		fmt.Print(err)
	}
	json.NewEncoder(w).Encode(status.Status)
}

func deleteCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runtimeId := vars["clusterId"]

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
