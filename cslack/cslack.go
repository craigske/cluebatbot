package cslack

import (
	"flag"
	"log"
	"os"

	"github.com/nlopes/slack"
)

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

var debugCSlack = flag.Bool("debugCSlack", true, "enable or disable debug in cslack")

// SlackServerManager is the entry point to the cslack lib
func SlackServerManager(slackAPI *slack.Client, server SlackServer, myID string, myTeamID string) {
	rtm := slackAPI.NewRTM()
	go rtm.ManageConnection()
	msgStack := NewItemRefStack()
	for msg := range rtm.IncomingEvents {
		handleSlackEvents(msg, *rtm, *slackAPI, server, msgStack)
	}
	debugText := os.Getenv("CSLACK_DEBUG")
	if debugText == "true" {
		log.Print("cslack debug on")
		*debugCSlack = true
	}
}

func handleSlackEvents(msg slack.RTMEvent, rtm slack.RTM, slackAPI slack.Client, server SlackServer, msgStack *ItemRefStack) {
	if *debugCSlack {
		log.Print("cslack debug on")
	}
	switch ev := msg.Data.(type) {
	case *slack.HelloEvent:
		// Ignore hello
	case *slack.ConnectedEvent:
		if *debugCSlack {
			log.Print("Event info:", ev.Info)
			log.Print("Connection counter:", ev.ConnectionCount)
		}
		rtm.SendMessage(rtm.NewOutgoingMessage("ClueBatBot Connected!", server.CluebatBotChan))
	case *slack.MessageEvent:
		msg := ev.Msg
		found, err := msgStack.Search(msg.Channel, msg.Timestamp)
		if err != nil {
			if *debugCSlack {
				log.Printf("%s\n", err)
			}
		}
		if found {
			if *debugCSlack {
				log.Println(server.Name, " GOT MY OWN MESSAGE")
			}
		} else {
			HandleSlackMessageEvent(*ev, rtm, slackAPI)
		}
		msgStack.Push(slack.NewRefToMessage(ev.Channel, ev.Timestamp))
	case *slack.PresenceChangeEvent:
		if *debugCSlack {
			log.Printf("Presence Change: %v\n", ev)
		}
	case *slack.LatencyReport:
		if *debugCSlack {
			log.Printf("Current latency: %v\n", ev.Value)
		}
	case *slack.RTMError:
		if *debugCSlack {
			log.Printf("Slack RTM Error: %s\n", ev.Error())
		}
	case *slack.InvalidAuthEvent:
		if *debugCSlack {
			log.Fatalln("Invalid credentials for ", server.Name)
		}
	case *slack.UserChangeEvent:
		if *debugCSlack {
			log.Printf("Slack User change event for %s who is %s", ev.User.ID, ev.User.Name)
		}
	default:
		// Ignore other events..
		if *debugCSlack {
			log.Printf("Unhandled event in cslack: %v\n", msg.Data)
		}
	}
}
