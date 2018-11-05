package cslack

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"../redis"
	"github.com/golang/glog"
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
	debugCSlack    = flag.Bool("debugCSlack", false, "enable or disable debug in cslack")
	botID          string
	latencyCounter int
	latencySlice   []int64
)

// SlackServerManager is the entry point to the cslack lib
func SlackServerManager(slackAPI *slack.Client, server SlackServer, myID string, myTeamID string) {
	debugText := os.Getenv("CSLACK_DEBUG")
	if debugText == "true" {
		glog.Infof("init %s cslack debug on", server.Name)
		*debugCSlack = true
	}

	botID = myID
	rtm := slackAPI.NewRTM()
	go rtm.ManageConnection()

	// store all the channels and users on startup
	getSlackUsers(slackAPI, server)
	getSlackChannels(slackAPI, server)

	// stack of messages for the win...
	for msg := range rtm.IncomingEvents {
		handleSlackEvents(msg, *rtm, *slackAPI, server)
		glog.Flush()
	}
}

func getSlackUsers(slackAPI *slack.Client, server SlackServer) {
	var counter int
	users, err := slackAPI.GetUsers()
	if err != nil {
		glog.Errorln("Error initializing slack users")
	}
	// prints the range of users we just snarfed
	for index, user := range users {
		counter++
		if *debugCSlack {
			glog.Infoln(server.Name + " found " + strconv.Itoa(index) + " is " + user.ID + " name:" + user.Name + " realname: " + user.Profile.RealName + " email: " + user.Profile.Email)
		}
		// Already in redis?
		exists, err := redis.Exists(server.Name + "-user-" + user.ID)
		if err != nil {
			glog.Errorf("%s redis Exists Err", server.Name)
		}
		if !exists {
			stringUser := fmt.Sprintf("%#v", user)
			redis.Set(server.Name+"-user-"+user.ID, []byte(stringUser))
		}
	}
	glog.Infof("%s added %d users\n", server.Name, counter)
}

func getSlackChannels(slackAPI *slack.Client, server SlackServer) {
	var counter int
	channels, err := slackAPI.GetChannels(false)
	if err != nil {
		glog.Errorf("%s\n", err)
		return
	}
	for index, channel := range channels {
		counter++
		if *debugCSlack {
			glog.Infoln(server.Name + " found " + strconv.Itoa(index) + " is " + channel.ID + " name:" + channel.Name)
		}
		// Already in redis?
		exists, err := redis.Exists(server.Name + "-channel-" + channel.ID)
		if err != nil {
			glog.Errorf("%s redis Exists Err", server.Name)
		}
		if !exists {
			stringChannel := fmt.Sprintf("%#v", channel)
			redis.Set(server.Name+"-channel-"+channel.ID, []byte(stringChannel))
		}
	}
	glog.Infof("%s added %d channels\n", server.Name, counter)
}

func handleSlackEvents(msg slack.RTMEvent, rtm slack.RTM, slackAPI slack.Client, server SlackServer) {
	switch ev := msg.Data.(type) {
	case *slack.HelloEvent:
		// Ignore hello
	case *slack.ConnectedEvent:
		if *debugCSlack {
			glog.Infof("%s Event info: %v", server.Name, ev.Info)
			glog.Infof("%s Connection counter: %v", server.Name, ev.ConnectionCount)
		}
		botID = ev.Info.User.ID
		rtm.SendMessage(rtm.NewOutgoingMessage("ClueBatBot Connected!", server.CluebatBotChan))
	case *slack.MessageEvent:
		if ev.User != botID {
			HandleSlackMessageEvent(*ev, rtm, slackAPI, server)
		}
	case *slack.PresenceChangeEvent:
		if *debugCSlack {
			glog.Infof("%s got presence Change: %v\n", server.Name, ev)
		}
	case *slack.LatencyReport:
		handleLatency(ev.Value, server)
	case *slack.RTMError:
		if *debugCSlack {
			glog.Infof("%s got slack RTM Error: %s\n", server.Name, ev.Error())
		}
	case *slack.InvalidAuthEvent:
		if *debugCSlack {
			glog.Fatalf("%s failed due to invalid credentials", server.Name)
		}
	case *slack.UserChangeEvent:
		if *debugCSlack {
			glog.Infof("%s got slack User change event for %s who is %s", server.Name, ev.User.ID, ev.User.Name)
		}
	default:
		// Ignore other events..
		if *debugCSlack {
			glog.Infof("%s found an unhandled event %v\n", server.Name, ev)
		}
	}
}

func handleLatency(latency time.Duration, server SlackServer) {
	// report only high latency
	var latencyThreshold = time.Duration(1 * time.Second)
	if int64(latency) > int64(latencyThreshold) {
		glog.Errorf("%s latency over threshold(%s): %s", server.Name, latencyThreshold, latency)
	}
	latencySlice = append(latencySlice, latency.Nanoseconds())
	if latencyCounter%100 == 0 {
		var total int64 = 0
		for _, l := range latencySlice {
			total += l
		}
		avg := total / int64(len(latencySlice))
		// save to redis
		now := time.Now()
		jsonNow, err := now.MarshalJSON()
		if err != nil {
			glog.Error("time conversion error: " + err.Error())
		}
		key := server.Name + "-latency-" + string(jsonNow)
		jsonLatency := string(avg)
		redis.Set(key, []byte(jsonLatency))
		glog.Infof("%s avg latency now %s", server.Name, time.Duration(avg))
	}
	latencyCounter++
	glog.Flush()
}

/* TODO: write me
func reportLatency(server SlackServer) {

}
*/
