package cslack

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"../redis"
	"github.com/nlopes/slack"
)

// SlackServer a server config
type SlackServer struct {
	Name           string `json:"Name"`
	APIKey         string `json:"APIKey"`
	CluebatBotChan string `json:"CluebatBotChan"`
	OwnerID        string `json:"OwnerID"`
}

var (
	debugCSlack = flag.Bool("debugCSlack", false, "enable or disable debug in cslack")
	botID       string
)

// SlackServerManager is the entry point to the cslack lib
func SlackServerManager(slackAPI *slack.Client, server SlackServer, myID string, myTeamID string) {
	debugText := os.Getenv("CSLACK_DEBUG")
	if debugText == "true" {
		log.Printf("init %s cslack debug on", server.Name)
		*debugCSlack = true
	}

	botID = myID
	rtm := slackAPI.NewRTM()
	go rtm.ManageConnection()

	// store all the channels and users on startup
	getSlackUsers(slackAPI, server)
	getSlackChannels(slackAPI, server)

	// stack of messages for the win...
	msgStack := NewItemRefStack()
	for msg := range rtm.IncomingEvents {
		// if debugText == "true" {
		// 	log.Printf("%s got event type %v with data %v", server.Name, msg.Type, msg.Data)
		// }
		handleSlackEvents(msg, *rtm, *slackAPI, server, msgStack)
	}
}

func getSlackUsers(slackAPI *slack.Client, server SlackServer) {
	var counter int
	users, err := slackAPI.GetUsers()
	if err != nil {
		log.Println("Error initializing slack users")
	}
	// prints the range of users we just snarfed
	for index, user := range users {
		counter++
		if *debugCSlack {
			log.Println(server.Name + " found " + strconv.Itoa(index) + " is " + user.ID + " name:" + user.Name + " realname: " + user.Profile.RealName + " email: " + user.Profile.Email)
			//log.Printf("%d - %+v\n", index, user)
		}
		// Already in redis?
		exists, err := redis.Exists(server.Name + "-user-" + user.ID)
		if err != nil {
			log.Printf("%s redis Exists Err", server.Name)
		}
		if !exists {
			stringUser := fmt.Sprintf("%#v", user)
			redis.Set(server.Name+"-user-"+user.ID, []byte(stringUser))
		}
	}
	log.Printf("%s added %d users\n", server.Name, counter)
}

func getSlackChannels(slackAPI *slack.Client, server SlackServer) {
	var counter int
	channels, err := slackAPI.GetChannels(false)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}
	for index, channel := range channels {
		counter++
		if *debugCSlack {
			log.Println(server.Name + " found " + strconv.Itoa(index) + " is " + channel.ID + " name:" + channel.Name)
			//log.Printf("%d - %+v\n", index, user)
		}
		// Already in redis?
		exists, err := redis.Exists(server.Name + "-channel-" + channel.ID)
		if err != nil {
			log.Printf("%s redis Exists Err", server.Name)
		}
		if !exists {
			stringChannel := fmt.Sprintf("%#v", channel)
			redis.Set(server.Name+"-channel-"+channel.ID, []byte(stringChannel))
		}
	}
	log.Printf("%s added %d channels\n", server.Name, counter)
}

func handleSlackEvents(msg slack.RTMEvent, rtm slack.RTM, slackAPI slack.Client, server SlackServer, msgStack *ItemRefStack) {
	if *debugCSlack {
		log.Printf("%s cslack debug on", server.Name)
	}
	switch ev := msg.Data.(type) {
	case *slack.HelloEvent:
		// Ignore hello
	case *slack.ConnectedEvent:
		if *debugCSlack {
			log.Printf("%s Event info: %v", server.Name, ev.Info)
			log.Printf("%s Connection counter: %v", server.Name, ev.ConnectionCount)
		}
		botID = ev.Info.User.ID
		rtm.SendMessage(rtm.NewOutgoingMessage("ClueBatBot Connected!", server.CluebatBotChan))
	case *slack.MessageEvent:
		if ev.User != botID {
			HandleSlackMessageEvent(*ev, rtm, slackAPI, server, msgStack)
		}
	case *slack.PresenceChangeEvent:
		if *debugCSlack {
			log.Printf("%s got presence Change: %v\n", server.Name, ev)
		}
	case *slack.LatencyReport:
		if *debugCSlack {
			log.Printf("%s current latency: %v\n", server.Name, ev.Value)
		}
	case *slack.RTMError:
		if *debugCSlack {
			log.Printf("%s got slack RTM Error: %s\n", server.Name, ev.Error())
		}
	case *slack.InvalidAuthEvent:
		if *debugCSlack {
			log.Fatalf("%s failed due to invalid credentials", server.Name)
		}
	case *slack.UserChangeEvent:
		if *debugCSlack {
			log.Printf("%s got slack User change event for %s who is %s", server.Name, ev.User.ID, ev.User.Name)
		}
	default:
		// Ignore other events..
		if *debugCSlack {
			log.Printf("%s found an unhandled event %v\n", server.Name, ev)
		}
	}
}
