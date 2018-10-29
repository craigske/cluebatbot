package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"./redis"
	"github.com/logrusorgru/aurora"
	"github.com/nlopes/slack"
)

// User maps slack.User
type User slack.User

// Channel maps slack.Channel
type Channel slack.Channel

// Users maps a slice of slack.User
type Users []slack.User

// Channels maps a slice of slack.Channel
type Channels []slack.Channel

// ItemRefStack stack of ItemRefs
type ItemRefStack struct {
	lock sync.Mutex // you don't have to do this if you don't want thread safety
	s    []slack.ItemRef
}

// SlackChannelRecord is the json rep of a channel object from the api
type SlackChannelRecord struct {
	GroupConversation struct {
		Conversation struct {
			ID                 string `json:"ID"`
			Created            string `json:"Created"`
			IsOpen             bool   `json:"IsOpen"`
			LastRead           string `json:"LastRead"`
			UnreadCount        int    `json:"UnreadCount"`
			UnreadCountDisplay int    `json:"UnreadCountDisplay"`
			IsGroup            bool   `json:"IsGroup"`
			IsShared           bool   `json:"IsShared"`
			IsIM               bool   `json:"IsIM"`
			IsExtShared        bool   `json:"IsExtShared"`
			IsOrgShared        bool   `json:"IsOrgShared"`
			IsPendingExtShared bool   `json:"IsPendingExtShared"`
			IsPrivate          bool   `json:"IsPrivate"`
			IsMpIM             bool   `json:"IsMpIM"`
			Unlinked           int    `json:"Unlinked"`
			NameNormalized     string `json:"NameNormalized"`
			NumMembers         int    `json:"NumMembers"`
			Priority           int    `json:"Priority"`
		} `json:"conversation"`
		Name       string        `json:"Name"`
		Creator    string        `json:"Creator"`
		IsArchived bool          `json:"IsArchived"`
		Members    []interface{} `json:"Members"`
		Topic      struct {
			Value   string `json:"Value"`
			Creator string `json:"Creator"`
			LastSet string `json:"LastSet"`
		} `json:"Topic"`
		Purpose struct {
			Value   string `json:"Value"`
			Creator string `json:"Creator"`
		} `json:"Purpose"`
	} `json:"groupConversation"`
	IsChannel bool   `json:"IsChannel"`
	IsGeneral bool   `json:"IsGeneral"`
	IsMember  bool   `json:"IsMember"`
	Locale    string `json:"Locale"`
}

// SlackUserRecord is the json rep of a user from the slack api
type SlackUserRecord struct {
	ID       string `json:"ID"`
	TeamID   string `json:"TeamID"`
	Name     string `json:"Name"`
	Deleted  string `json:"Deleted"`
	Color    string `json:"Color"`
	RealName string `json:"RealName"`
	TZ       string `json:"TZ"`
	TZLabel  string `json:"TZLabel"`
	TZOffset string `json:"TZOffset"`
	Profile  struct {
		FirstName             string `json:"FirstName"`
		LastName              string `json:"LastName"`
		RealName              string `json:"RealName"`
		RealNameNormalized    string `json:"RealNameNormalized"`
		DisplayName           string `json:"DisplayName"`
		DisplayNameNormalized string `json:"DisplayNameNormalized"`
		Email                 string `json:"Email"`
		Skype                 string `json:"Skype"`
		Image24               string `json:"Image24"`
		Image32               string `json:"Image32"`
		Image48               string `json:"Image48"`
		Image72               string `json:"Image72"`
		Image192              string `json:"Image192"`
		ImageOriginal         string `json:"ImageOriginal"`
		Title                 string `json:"Title"`
		BotID                 string `json:"BotID"`
		APIAppID              string `json:"ApiAppID"`
		StatusText            string `json:"StatusText"`
		StatusEmoji           string `json:"StatusEmoji"`
		Team                  string `json:"Team"`
		Fields                struct {
			Fields []interface{} `json:"fields"`
		} `json:"Fields"`
	} `json:"Profile"`
	IsBot             bool   `json:"IsBot"`
	IsAdmin           bool   `json:"IsAdmin"`
	IsOwner           bool   `json:"IsOwner"`
	IsPrimaryOwner    bool   `json:"IsPrimaryOwner"`
	IsRestricted      bool   `json:"IsRestricted"`
	IsUltraRestricted bool   `json:"IsUltraRestricted"`
	IsStranger        bool   `json:"IsStranger"`
	IsAppUser         bool   `json:"IsAppUser"`
	Has2FA            bool   `json:"Has2FA"`
	HasFiles          bool   `json:"HasFiles"`
	Presence          string `json:"Presence"`
	Locale            string `json:"Locale"`
}

// SlackServer a server config
type SlackServer struct {
	Name           string `json:"Name"`
	APIKey         string `json:"APIKey"`
	CluebatBotChan string `json:"CluebatBotChan"`
	OwnerID        string `json:"OwnerID"`
}

// Globals
// Flags
var credsFile = "C:/Users/cskel/cluebatbot/cluebatbot-config.json" // set in init. Future refactor for external creds
var debug = flag.Bool("debug", true, "enable or disable debug")
var verbose = flag.Bool("verbose", false, "enable or disable verbose logging")
var colors = flag.Bool("colors", true, "enable or disable colors")

// message ItemRefStack
var msgStack = NewItemRefStack()

// channel fer the dying
var stopChan = make(chan os.Signal, 2)

// regular globals
var au aurora.Aurora
var tickCounter = 0
var users Users
var channels Channels
var slackServers []SlackServer

func init() {
	au = aurora.NewAurora(*colors)
	fmt.Println("init.....", au.Magenta("Aurora"))

	// the randoms
	rand.Seed(time.Now().UnixNano())

	writeConfig := os.Getenv("WRITE_EXAMPLE_CONFIG")
	if writeConfig == "true" {
		writeExampleCredsFile()
		log.Fatalln("Wrote example config")
	}

	// TODO: refactor to some config system
	// reads slack servers into slackServers
	readCredsFile()

	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost:6379"
	}
}

/* MAIN */
func main() {
	// state sync
	// var mutex = &sync.Mutex{}

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

	// //slack integration
	// slackAPI := slack.New("xoxb-274713070916-G2mo9Ts9Z2gotzEB94XmRp31")

	// //check slack
	// authTest, err := slackAPI.AuthTest()
	// if err != nil {
	// 	fmt.Printf("Error getting channels: %s\n", err)
	// 	return
	// }

	// dorkbotSlackID = authTest.UserID

	// // TODO: refactor out
	// go func(slackAPI *slack.Client) {
	// 	sig := <-stopChan
	// 	if *debug {
	// 		log.Println("got signal " + sig.String())
	// 	}
	// 	msg := sig.String() + " caught, terminating"
	// 	switch sig {
	// 	case syscall.SIGHUP:
	// 		mutex.Lock()
	// 		logToSlack("SIGHP Recieved", slackChannelDorkbotToCraig, slackAPI)
	// 		mutex.Unlock()
	// 		log.Println(au.Red(msg))
	// 	default: // none caught signals
	// 		mutex.Lock()
	// 		logToSlack(msg, slackChannelDorkbotToCraig, slackAPI)
	// 		mutex.Unlock()
	// 		log.Println(au.Red(msg))
	// 		stopChan <- syscall.SIGINT
	// 	}
	// }(slackAPI)

	// // update the global slice of users then
	// // watch slack for messages sent to me
	// getSlackUsers(slackAPI)
	// getSlackChannels(slackAPI)
	// go slackRTMManager(slackAPI)

	fmt.Printf("%+v", slackServers)

	code := <-stopChan
	sigInt, err := strconv.Atoi(code.String())
	if err != nil {
		log.Println(au.Red("Err getting the singal int value"))
	}
	os.Exit(sigInt)
}

func readCredsFile() {
	data, err := ioutil.ReadFile(credsFile)
	if err != nil {
		die("failed to open the creds file", err)
	}

	log.Println(string(data))

	err = json.Unmarshal(data, &slackServers)
	if err != nil {
		die("failed read slack sever json", err)
	}
	if *debug {
		log.Println(fmt.Sprintf("%#v", slackServers))
	}

}

func writeExampleCredsFile() {
	server1 := SlackServer{"server1", "apikey1", "control channel D111111", "owner id U1111111"}
	server2 := SlackServer{"server2", "apikey2", "control channel D222222", "owner id U2222222"}
	slackServers = []SlackServer{server1, server2}
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

/* ***** SLACK MESSAGE STACK ***** */

// NewItemRefStack returns stack of slack.ItemRef
func NewItemRefStack() *ItemRefStack {
	return &ItemRefStack{sync.Mutex{}, make([]slack.ItemRef, 0)}
}

// Push pushes onto stack
func (s *ItemRefStack) Push(v slack.ItemRef) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.s = append(s.s, v)
}

// Pop pops from stack returns ItemRef
func (s *ItemRefStack) Pop() (slack.ItemRef, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	l := len(s.s)
	if l == 0 {
		emptyRef := slack.NewRefToMessage("", "")
		return emptyRef, errors.New("Empty Stack")
	}

	res := s.s[l-1]
	s.s = s.s[:l-1]
	return res, nil
}

// Search return true if Channel and Timestamp match an ItemRef in the stack
func (s *ItemRefStack) Search(c string, ts string) (bool, error) {
	l := len(s.s)
	if l == 0 {
		return false, nil
	}
	for _, value := range s.s {
		// if *debug {
		// 	log.Println("searched " + value.Channel + " for " + c + " and  timestamp " + value.Timestamp + " for " + ts)
		// }
		if value.Channel == c && value.Timestamp == ts {
			if *debug {
				log.Println(au.Green("FOUND " + value.Channel + " for " + c + " and  timestamp " + value.Timestamp + " for " + ts))
			}
			// found
			return true, nil
		}
	}
	return false, nil
}
