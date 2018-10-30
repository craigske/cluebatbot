package cslack

import (
	"fmt"
	"log"
	"strings"

	"github.com/nlopes/slack"
)

// HandleSlackMessageEvent is the entry point to messageEvent for message handling. From here,
// messages are evaluated as commands or a message of the type we can respond to
func HandleSlackMessageEvent(ev slack.MessageEvent, rtm slack.RTM, slackAPI slack.Client, server SlackServer, msgStack *ItemRefStack) {
	if *debugCSlack {
		log.Println("handling event for msg: ", ev.Msg)
	}
	tehmsg := ev.Msg.Text
	tehmsgTokens := strings.Split(tehmsg, " ")
	tokenLength := len(tehmsgTokens)
	cmd := tehmsgTokens[0]
	object := ""
	predicate := ""
	if tokenLength == 2 {
		object = tehmsgTokens[1]
		predicate = ""
	} else if tokenLength > 2 {
		object = tehmsgTokens[1]
		predicate = fmt.Sprintf("%v", tehmsgTokens[2:])
	}
	switch cmd {
	case "ping":
		if *debugCSlack {
			log.Println(server.Name + " someone " + ev.Name + " pinged me bro " + ev.Type)
			sendSlackMessage("pong", ev.Channel, slackAPI, server, msgStack)
		}
	case "clue":
		if *debugCSlack {
			log.Printf("%s got clue for %s\ncmd: %s object: %s predicate: %s", server.Name, tehmsgTokens[1], cmd, object, predicate)
			//apply security
			// TODO: needs to be a redis kept list
			if ev.User != server.OwnerID {
				return
			}
			//find user
			//user, err := slackAPI.GetUserInfo(object)
			userString := strings.Trim(object, "<@")
			userString = strings.Trim(userString, ">")
			_, err := slackAPI.GetUserInfo(userString)
			if err != nil {
				log.Printf("%s error for user %s lookup %s", server.Name, userString, err)
				//return
			}
			// select a random channel that the user is in
			channels, err := slackAPI.GetChannels(false)
			if err != nil {
				fmt.Printf("%s\n", err)
				return
			}
			// TODO: conversations is what we want here

			for _, channel := range channels {
				for _, member := range channel.Members {
					if member == userString {
						log.Printf("%s - %s is a member of %s\n", server.Name, userString, channel.Name)
					} else {
						log.Printf("%s %s is not %s", server.Name, userString, member)
					}
				}
			}
			//join random channel
			//send message to random channel
			//leave random channel
		}
	case "die":
		if *debugCSlack {
			log.Fatalln(server.Name + " got die")
		}
	default:
		if *debugCSlack {
			log.Println(server.Name+" ignoring:", tehmsg)
		}
	}
}

func sendSlackMessage(msg string, chanFrom string, slackAPI slack.Client, server SlackServer, msgStack *ItemRefStack) {
	channelID, timestamp, err := slackAPI.PostMessage(chanFrom, slack.MsgOptionText(msg, false))
	if err != nil {
		log.Printf("%s\n", err)
	}
	if *debugCSlack {
		log.Printf("%s sent %s to channelID: %s at %s", server, msg, channelID, timestamp)
	}
	msgStack.Push(slack.NewRefToMessage(channelID, timestamp))
}
