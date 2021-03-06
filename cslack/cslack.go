package cslack

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"github.com/nlopes/slack"
)

// SlackServer a server config
type SlackServer struct {
	Name           string `json:"Name"`
	APIKey         string `json:"APIKey"`
	CluebatBotChan string `json:"CluebatBotChan"`
	OwnerID        string `json:"OwnerID"`
	LatencyCounter int
	LatencySlice   []int64
	Channels       map[string]slack.Channel
	Users          map[string]slack.User
}

var (
	debugCSlack      = flag.Bool("debugCSlack", false, "enable or disable debug in cslack")
	debugLatencyTick = flag.Bool("debugLatencyTick", false, "tick every time a latency message is processed. Talkative")
	botID            string
)

// SlackServerManager is the entry point to the cslack lib
func SlackServerManager(slackAPI *slack.Client, server SlackServer, myID string, myTeamID string) {
	debugText := os.Getenv("CSLACK_DEBUG")
	if debugText == "true" {
		glog.Infof("init %s cslack debug on", server.Name)
		*debugCSlack = true
	}
	debugLTick := os.Getenv("CSLACK_DEBUG_LATENCY_TICK")
	if debugLTick == "true" {
		*debugLatencyTick = true
	}

	botID = myID
	rtm := slackAPI.NewRTM()
	go rtm.ManageConnection()

	// init maps
	server.Users = make(map[string]slack.User)
	server.Channels = make(map[string]slack.Channel)

	// store all the channels and users on startup
	getSlackUsers(slackAPI, &server)
	getSlackChannels(slackAPI, &server)

	// stack of messages for the win...
	for msg := range rtm.IncomingEvents {
		handleSlackEvents(msg, *rtm, *slackAPI, &server)
		glog.Flush()
	}
}

func handleSlackEvents(msg slack.RTMEvent, rtm slack.RTM, slackAPI slack.Client, server *SlackServer) {
	switch ev := msg.Data.(type) {
	case *slack.HelloEvent:
		// Ignore hello
	case *slack.ConnectedEvent:
		botID = ev.Info.User.ID
		rtm.SendMessage(rtm.NewOutgoingMessage("ClueBatBot Connected!", server.CluebatBotChan))
	case *slack.MessageEvent:
		if ev.User != botID {
			HandleSlackMessageEvent(*ev, rtm, slackAPI, server)
		}
	case *slack.PresenceChangeEvent:
		// Ignoring PresenceChangeEvent
	case *slack.LatencyReport:
		handleLatency(ev.Value, server)
	case *slack.RTMError:
		if *debugCSlack {
			glog.Infof("%s got slack RTM Error: %v\n", server.Name, ev)
		}
	case *slack.InvalidAuthEvent:
		if *debugCSlack {
			glog.Fatalf("%s failed due to invalid credentials. It is likely that this api key is bad.", server.Name)
		}
	case *slack.UserChangeEvent:
		// Ignoring UserChangeEvents
	default:
		// Ignore other events..
		if *debugCSlack {
			glog.Infof("%s found an unhandled event %v\n", server.Name, ev)
		}
	}
}
