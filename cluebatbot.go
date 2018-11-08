package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"./cslack"
	"./redis"
	"github.com/golang/glog"
	"github.com/logrusorgru/aurora"
	"github.com/nlopes/slack"
	"golang.org/x/oauth2/google"
	container "google.golang.org/api/container/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
)

// User maps slack.User
type User slack.User

// Channel maps slack.Channel
type Channel slack.Channel

// Users maps a slice of slack.User
type Users []slack.User

// Channels maps a slice of slack.Channel
type Channels []slack.Channel

// SlackServer the cslack SlackServer type
type SlackServer cslack.SlackServer

// flags
var debug = flag.Bool("debug", true, "enable or disable debug")
var verbose = flag.Bool("verbose", false, "enable or disable verbose logging")
var colors = flag.Bool("colors", true, "enable or disable colors")
var serviceDNS = flag.String("port", "2000", "app port")
var port = flag.String("serviceDNS", "localhost", "app service DNS name")
var credsFile = flag.String("credsFile", "./cluebatbot-config.json", "credentials file")
var makeMasterOnError = flag.Bool("makeMasterOnError", false, "make this node master if unable to connect to the cluster ip provided.")

// Globals
var stopChan = make(chan os.Signal, 2)
var au aurora.Aurora
var tickCounter = 0
var users Users
var channels Channels
var slackServers []cslack.SlackServer
var runningInK8s bool
var nodeName string

func init() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")
	glog.Infoln("INIT ClueBatBot")

	writeConfig := os.Getenv("WRITE_EXAMPLE_CONFIG")
	if writeConfig == "true" {
		writeExampleCredsFile()
		glog.Fatalln("Wrote example config. Unset WRITE_EXAMPLE_CONFIG to stop doing this.")
	}

	readCredsFile() // TODO: refactor to some config system

	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost:6379"
	}

	nodeName = os.Getenv("MY_POD_NAME")
	if len(nodeName) == 0 {
		rand.Seed(time.Now().UnixNano())
		nodeName = string(rand.Uint32())
	} else {
		runningInK8s = true
	}
}

/* MAIN */
func main() {
	err := redis.Ping()
	if err != nil {
		glog.Errorf("Error pinging redis: %s\n", err)
		return
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// google connect
	ctx := context.Background()
	hc, err := google.DefaultClient(ctx, container.CloudPlatformScope)
	if err != nil {
		glog.Errorf("Could not get authenticated client: %v", err)
	} else {
		svc, err := container.New(hc)
		if err != nil {
			glog.Errorf("Could not initialize gke client: %v", err)
		} else {
			if err := listClusters(svc, "craigskelton-com", "us-central1-a"); err != nil {
				glog.Errorf("list clusters error: %s", err)
			} else {
				runningInK8s = true
			}
		}
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Errorln(err.Error())
	} else {
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			glog.Errorln(err.Error())
		} else {
			pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
			if err != nil {
				glog.Errorln(err.Error())
			}
			glog.Infof("There are %d pods in the cluster\n", len(pods.Items))
			// Examples for error handling:
			// - Use helper functions like e.g. errors.IsNotFound()
			// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
			_, err = clientset.CoreV1().Pods("default").Get("example-xxxxx", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				glog.Infof("Pod not found\n")
			} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
				glog.Infof("Error getting pod %v\n", statusError.ErrStatus.Message)
			} else if err != nil {
				glog.Errorf(err.Error())
			} else {
				glog.Errorf("Found pod\n")
			}
		}
	}

	// initialize a slack server chan for each server
	for _, server := range slackServers {
		if *debug {
			glog.Infof("Creating server named %s \n", server.Name)
		}
		currentSlackAPI := slack.New(server.APIKey)
		authTest, err := currentSlackAPI.AuthTest()
		if err != nil {
			glog.Infof("Error in auth: %s\n", err)
			return
		}
		// start the server manager
		go cslack.SlackServerManager(currentSlackAPI, server, authTest.UserID, authTest.TeamID)
	}

	code := <-stopChan
	sigInt, err := strconv.Atoi(code.String())
	if err != nil {
		glog.Errorf("Err getting the singal int value")
	}
	glog.Info("Stopping cluebatbot")
	glog.Flush()
	os.Exit(sigInt)
}

func listClusters(svc *container.Service, projectID, zone string) error {
	list, err := svc.Projects.Zones.Clusters.List(projectID, zone).Do()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %v", err)
	}
	for _, v := range list.Clusters {
		glog.Infof("Cluster %q (%s) master_version: v%s", v.Name, v.Status, v.CurrentMasterVersion)

		poolList, err := svc.Projects.Zones.Clusters.NodePools.List(projectID, zone, v.Name).Do()
		if err != nil {
			return fmt.Errorf("failed to list node pools for cluster %q: %v", v.Name, err)
		}
		for _, np := range poolList.NodePools {
			glog.Infof("  -> Pool %q (%s) machineType=%s node_version=v%s autoscaling=%v", np.Name, np.Status,
				np.Config.MachineType, np.Version, np.Autoscaling != nil && np.Autoscaling.Enabled)
		}
	}
	return nil
}

func amIMaster(k8sclientset kubernetes.Clientset) bool {
	pods, err := k8sclientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		glog.Infof("%s not running in k8s. Clientset err: %s", nodeName, err)
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
	for pod := range pods.Items {
		glog.Infof("%v", pod)
	}
	return false
}

func readCredsFile() {
	data, err := ioutil.ReadFile(*credsFile)
	if err != nil {
		die("failed to open the creds file", err)
	}

	err = json.Unmarshal(data, &slackServers)
	if err != nil {
		die("failed read slack sever json", err)
	}
	if *debug {
		glog.Infof("SlackServers structs: \n%#v", slackServers)
	}

}

// SlackServer a server config
// type SlackServer struct {
// 	Name           string  `json:"Name"`
// 	APIKey         string  `json:"APIKey"`
// 	CluebatBotChan string  `json:"CluebatBotChan"`
// 	OwnerID        string  `json:"OwnerID"`
// 	LatencyCounter int     `json:"latencyCounter"`
// 	LatencySlice   []int64 `json:",string"`
// }

func writeExampleCredsFile() {
	var tempLatencySlice []int64
	tempLatencySlice = append(tempLatencySlice, 0)
	tempLatencySlice = append(tempLatencySlice, 1)
	server1 := cslack.SlackServer{
		Name:           "example-server1-human-name",
		APIKey:         "apikey1",
		CluebatBotChan: "control channel D111111",
		OwnerID:        "owner id U1111111",
		LatencyCounter: 0,
		LatencySlice:   tempLatencySlice}
	server2 := cslack.SlackServer{
		Name:           "example-server2-human-name",
		APIKey:         "apikey2",
		CluebatBotChan: "control channel D222222",
		OwnerID:        "owner id U2222222",
		LatencyCounter: 0,
		LatencySlice:   tempLatencySlice}
	slackServers = []cslack.SlackServer{server1, server2}
	log.Println(slackServers)
	f, err := os.Create("example.json")
	if err != nil {
		die("failed to open example config", err)
	}
	jsonData, err := json.Marshal(slackServers)
	if err != nil {
		die("failed to create json for example config", err)
	}
	_, err = f.Write([]byte(jsonData))
	if err != nil {
		die("failed to write to example config", err)
	}
}

func die(msg string, err error) {
	glog.Fatalln(au.Red(msg), au.Cyan(err))
}
