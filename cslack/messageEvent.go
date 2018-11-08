package cslack

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/nlopes/slack"
)

// HandleSlackMessageEvent is the entry point to messageEvent for message handling. From here,
// messages are evaluated as commands or a message of the type we can respond to
func HandleSlackMessageEvent(ev slack.MessageEvent, rtm slack.RTM, slackAPI slack.Client, server *SlackServer) {
	if *debugCSlack {
		glog.Infof("handling event for msg: %v", ev.Msg)
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
	case "ping", "Ping":
		if *debugCSlack {
			user := server.Users[ev.User]
			glog.Infof("%s someone named %s pinged me bro. Type: %s", server.Name, user.Name, ev.Type)
		}
		channelID, _, err := sendSlackMessage(ev, "pong", ev.Channel, slackAPI, server)
		if err != nil {
			glog.Errorf("%s got error: \"%s\" sending to %s", server.Name, err, channelID)
		}
	case "bat", "Bat":
		if *debugCSlack {
			glog.Infof("%s got clue for %s\ncmd: %s object: %s predicate: %s", server.Name, tehmsgTokens[1], cmd, object, predicate)
			//apply security
			// TODO: needs to be a redis kept list
			if ev.User != server.OwnerID {
				return
			}
			//find user
			//user, err := slackAPI.GetUserInfo(object)
			userString := strings.Trim(object, "<@")
			userString = strings.Trim(userString, ">")
			user := server.Users[userString]
			// TODO: conversations is what we want here
			var cursor string
			params := slack.GetConversationsForUserParameters{UserID: userString, Cursor: cursor, Types: []string{"public_channel", "private_channel"}, Limit: 100}
			members, cursor, err := slackAPI.GetConversationsForUser(&params)
			if err != nil {
				glog.Errorf("%s error getting conversations for %s was %s", server.Name, userString, err)
				return
			}
			if len(members) == 0 {
				glog.Infof("%s %s has no conversations I can find. Harassment failure", server.Name, userString)
				return
			}

			//join random channel
			r := rand.New(rand.NewSource(time.Now().UnixNano() * 99)) // random seed + salt is probably enough :)
			randomChannelIndex := r.Intn((len(members) - 1))
			for i, member := range members {
				if *debugCSlack {
					glog.Infof("%s %d member %v", server.Name, i, member)
				}
				if randomChannelIndex == i && (member.Name != "announcements") {
					if *debugCSlack {
						glog.Infof("%s found %s to harass %s in", server.Name, member.ID, userString)
					}
					_, err := slackAPI.JoinChannel(member.Name)
					if err != nil {
						glog.Errorf("%s error joining channel %s to harass user %s: %s", server.Name, member.Name, userString, err)
						//return
					}
					//send message to random channel
					_, timestamp, err := sendSlackMessage(ev, clueBatMessage(user, ""), member.ID, slackAPI, server)
					if err != nil {
						glog.Errorf("%s error harassing %s in random channel %s - %s: %s", server.Name, userString, member.ID, member.Name, err)
					}
					timeInSeconds, err := strconv.ParseInt(timestamp, 10, 64)
					if err != nil {
						glog.Errorf("%s error converting %s to int for time conversion. Setting time to Time.now(). Will be wrong. Err is %s", server.Name, timestamp, err)
					}
					timeFromUnix := time.Unix(timeInSeconds, 0)
					msg := fmt.Sprintf("sent <@%s> a cluebat message in <#%s> at %s\n If you join right away, they'll totally know it was you. <GRIN>", user.ID, member.ID, timeFromUnix.String())
					_, _, err = sendSlackMessage(ev, msg, ev.Channel, slackAPI, server)
					if err != nil {
						glog.Errorf("%s error harassing %s in random channel %s - %s: %s", server.Name, userString, member.ID, member.Name, err)
					}
					//leave random channel
					_, err = slackAPI.LeaveChannel(member.Name)
					if err != nil {
						glog.Errorf("%s error leaving channel %s - %s while harassing %s: %s", server.Name, member.ID, member.Name, userString, err)
					}
					glog.Infof("A cluebat was sent on %s to %s by %s in %s at %s",
						server.Name, user.Name, ev.Name, member.Name, timeFromUnix.String())
				}
			}
		}
	case "help":
		useage := "send a message to cluebatbot in any channel (or by DM, hint hint) of the form `bat @user`.\nCluebatbot will find a random channel then hit @user with a cluebat in it. @user will never see it coming"
		_, _, err := sendSlackMessage(ev, useage, ev.Channel, slackAPI, server)
		if err != nil {
			glog.Errorf("%s error sending help in channel %s", server.Name, ev.Channel)
		}
	case "img":
		attachment := slack.Attachment{
			Pretext: "ClueBatBot engage!",
			Text:    "I'm gonna bat you a clue",
			Fields: []slack.AttachmentField{
				slack.AttachmentField{
					Title: "cluebat",
					Value: "a cluebat for you",
				},
			},
			ImageURL: "http://austenblog.files.wordpress.com/2009/04/mycluebat.jpg",
		}
		msgOptionAttachments := slack.MsgOptionAttachments(attachment)
		channelID, timestamp, err := slackAPI.PostMessage(ev.Channel, msgOptionAttachments, slack.MsgOptionText("", false))
		if err != nil {
			glog.Errorf("%s error sending to %s is %s\n", server.Name, ev.Channel, err)
		}
		if *debugCSlack {
			glog.Infof("%s sent img attachment to channelID: %s at %s", server.Name, channelID, timestamp)
		}
	case "die":
		if *debugCSlack {
			glog.Fatalln(server.Name + " got die")
		}
	default:
		if *debugCSlack {
			glog.Infof(server.Name+" ignoring:", tehmsg)
		}
	}
}

func sendSlackMessage(ev slack.MessageEvent, msg string, chanTo string, slackAPI slack.Client, server *SlackServer) (string, string, error) {
	params := slack.PostMessageParameters{}
	params.Channel = chanTo
	params.User = botID
	// params.AsUser = true
	params.IconURL = "https://avatars.slack-edge.com/2018-10-30/468904459303_65c7fc492ecc467edcbe_192.jpg"
	params.Username = "ClueBatBot"
	params.Markdown = true
	params.UnfurlLinks = true
	// attachment := slack.Attachment{
	// 	Text: msg,
	// }
	// params.Attachments = []slack.Attachment{attachment}
	channelID, timestamp, err := slackAPI.PostMessage(chanTo, slack.MsgOptionText(msg, false), slack.MsgOptionPostMessageParameters(params))
	if err != nil {
		glog.Errorf("%s error sending to %s is %s with params: %v\n", server.Name, chanTo, err, params)
	}
	if *debugCSlack {
		glog.Infof("%s sent %s to channelID: %s at %s", server.Name, msg, channelID, timestamp)
	}
	return channelID, timestamp, err
}

// TODO: name if non-anonymous. Anon for now
func clueBatMessage(target slack.User, name string) string {
	var messages []string
	messages = append(messages, fmt.Sprintf("<@%s> you've been hit with a cluebat, peon", target.ID))
	messages = append(messages, fmt.Sprintf("WHAM. <@%s>, you've been nailed with the cluebat. Hopefully it left a lasting impression", target.ID))
	messages = append(messages, fmt.Sprintf("SHWOK. <@%s>, you've been beaned in the noggin with the cluebat. Hopefully it imparted clue", target.ID))
	messages = append(messages, fmt.Sprintf("THWACK. <@%s>, you've been hit with the cluebat. Clue imprint attempted", target.ID))
	r := rand.New(rand.NewSource(time.Now().UnixNano() * 99)) // random seed + salt is probably enough :)
	randomChannelIndex := r.Intn((len(messages) - 1))
	return messages[randomChannelIndex]
}
