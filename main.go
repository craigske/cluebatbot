package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/craigske/cluebatbot/cslack"
	"github.com/craigske/cluebatbot/redis_wrapper"
	"github.com/golang/glog"
	"github.com/logrusorgru/aurora"
	"github.com/nlopes/slack"

	// // may be required later for google API auth
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
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
	err := redis_wrapper.Ping()
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
