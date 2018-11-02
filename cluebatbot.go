package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"./cslack"
	"./redis"
	"github.com/logrusorgru/aurora"
	"github.com/nlopes/slack"
)

/* imported types we'll need */
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

/* Globals */
var credsFile = "./cluebatbot-config.json" // set in init. Future refactor for external creds
var debug = flag.Bool("debug", true, "enable or disable debug")
var verbose = flag.Bool("verbose", false, "enable or disable verbose logging")
var colors = flag.Bool("colors", true, "enable or disable colors")
var stopChan = make(chan os.Signal, 2)
var au aurora.Aurora
var tickCounter = 0
var users Users
var channels Channels
var slackServers []cslack.SlackServer
var podID string
var podName string

func init() {
	au = aurora.NewAurora(*colors)
	fmt.Println("INIT ClueBatBot")

	writeConfig := os.Getenv("WRITE_EXAMPLE_CONFIG")
	if writeConfig == "true" {
		writeExampleCredsFile()
		log.Fatalln("Wrote example config. Unset WRITE_EXAMPLE_CONFIG to stop doing this.")
	}

	podID = os.Getenv("MY_POD_IP")
	if podID == "" {
		podID = string(os.Getpid())
	}
	podName = os.Getenv("MY_POD_NAME")
	if podName == "" {
		podName = string(os.Getpid())
	}

	readCredsFile() // TODO: refactor to some config system

	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost:6379"
	}
}

/* MAIN */
func main() {
	err := redis.Ping()
	if err != nil {
		fmt.Printf("Error pinging redis: %s\n", err)
		return
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// am i master or hot spare?
	for {
		if amIMaster() {
			// initialize a slack server chan for each server
			for _, server := range slackServers {
				if *debug {
					log.Printf("Creating server named %s \n", server.Name)
				}
				currentSlackAPI := slack.New(server.APIKey)
				authTest, err := currentSlackAPI.AuthTest()
				if err != nil {
					fmt.Printf("Error in auth: %s\n", err)
					return
				}
				// start the server manager
				go cslack.SlackServerManager(currentSlackAPI, server, authTest.UserID, authTest.TeamID)
			}
			code := <-stopChan
			sigInt, err := strconv.Atoi(code.String())
			if err != nil {
				log.Println(au.Red("Err getting the singal int value"))
			}
			log.Println(au.Green("Stopping cluebatbot"))
			redis.Delete("cluster-id-master")
			os.Exit(sigInt)
		} else {
			log.Printf("I %s am not master, waiting...", podID+podName)
			time.Sleep(time.Minute)
		}
	}
}

func amIMaster() bool {
	myID := podID + podName
	master, err := redis.Get("cluster-id-master")
	if err != nil {
		log.Printf("amIMaster redis Exists Err: %v", err)
	}
	if string(master) != myID {
		log.Printf("I %s am master", myID)
		redis.Set("cluster-id-master", []byte(myID))
		return true
	}
	return false
}

func readCredsFile() {
	data, err := ioutil.ReadFile(credsFile)
	if err != nil {
		die("failed to open the creds file", err)
	}

	err = json.Unmarshal(data, &slackServers)
	if err != nil {
		die("failed read slack sever json", err)
	}
	if *debug {
		log.Println(fmt.Sprintf("SlackServers Structs: \n%#v", slackServers))
	}

}

func writeExampleCredsFile() {
	server1 := cslack.SlackServer{"server1", "apikey1", "control channel D111111", "owner id U1111111"}
	server2 := cslack.SlackServer{"server2", "apikey2", "control channel D222222", "owner id U2222222"}
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
	n2, err := f.Write([]byte(jsonData))
	if err != nil {
		die("failed to write to example config", err)
	}
	fmt.Printf("wrote %d bytes\n", n2)
}

func die(msg string, err error) {
	log.Fatalln(au.Red(msg), au.Cyan(err))
}
