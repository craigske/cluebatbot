package cslack

import (
	"log"
	"strings"

	"github.com/nlopes/slack"
)

// HandleSlackMessageEvent is the entry point to messageEvent for message handling. From here,
// messages are evaluated as commands or a message of the type we can respond to
func HandleSlackMessageEvent(ev slack.MessageEvent, rtm slack.RTM, slackAPI slack.Client) {
	if *debugCSlack {
		log.Println("handling event for msg: ", ev.Msg)
	}
	tehmsg := ev.Msg.Text
	tehmsgTokens := strings.Split(tehmsg, " ")
	// tokenLength := len(tehmsgTokens)
	cmd := tehmsgTokens[0]
	// object := ""
	// predicate := ""
	// if tokenLength == 2 {
	// 	object = tehmsgTokens[1]
	// 	predicate = ""
	// } else if tokenLength > 2 {
	// 	object = tehmsgTokens[1]
	// 	predicate = fmt.Sprintf("%v", tehmsgTokens[2:])
	// }
	switch cmd {
	case "ping":
		if *debugCSlack {
			log.Println("someone " + ev.Name + " pinged me bro " + ev.Type)
			sendSlackMessage("pong", ev.Channel, slackAPI)
		}
	case "die":
		if *debugCSlack {
			log.Println("got die")
		}
	default:
		if *debugCSlack {
			log.Println("Ignoring:", tehmsg)
		}
	}
}

func sendSlackMessage(msg string, chanFrom string, slackAPI slack.Client) {
	channelID, timestamp, err := slackAPI.PostMessage(chanFrom, slack.MsgOptionText(msg, false))
	if err != nil {
		log.Printf("%s\n", err)
	}
	if *debugCSlack {
		log.Printf("Sent %s to channelID: %s at %s", msg, channelID, timestamp)
	}
}
