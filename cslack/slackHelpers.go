package cslack

import (
	"fmt"
	"strconv"

	"../redis"
	"github.com/golang/glog"
	"github.com/nlopes/slack"
)

func getSlackUsers(slackAPI *slack.Client, server *SlackServer) {
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
		//add to map
		server.Users[user.ID] = user
		// Already in redis?
		exists, err := redis.Exists(server.Name + ":user:" + user.ID)
		if err != nil {
			glog.Errorf("%s redis Exists Err", server.Name)
		}
		if !exists {
			stringUser := fmt.Sprintf("%#v", user)
			redis.Set(server.Name+":user:"+user.ID, []byte(stringUser))
		}
	}
	glog.Infof("%s added %d users\n", server.Name, counter)
}

func getSlackChannels(slackAPI *slack.Client, server *SlackServer) {
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
		server.Channels[channel.ID] = channel
		// Already in redis?
		exists, err := redis.Exists(server.Name + ":channel:" + channel.ID)
		if err != nil {
			glog.Errorf("%s redis Exists Err", server.Name)
		}
		if !exists {
			stringChannel := fmt.Sprintf("%#v", channel)
			redis.Set(server.Name+":channel:"+channel.ID, []byte(stringChannel))
		}
	}
	glog.Infof("%s added %d channels\n", server.Name, counter)
}
