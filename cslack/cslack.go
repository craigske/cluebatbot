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
	debugText := os.Getenv("CSLACK_DEBUG")
	if debugText == "true" {
		log.Print("cslack debug on")
		*debugCSlack = true
	}

	rtm := slackAPI.NewRTM()
	go rtm.ManageConnection()

	// store all the channels and users on startup
	getSlackUsers(slackAPI, server)
	getSlackChannels(slackAPI, server)

	msgStack := NewItemRefStack()
	for msg := range rtm.IncomingEvents {
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
	log.Printf("added %d users\n", counter)
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
	log.Printf("added %d channels\n", counter)
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
		rtm.SendMessage(rtm.NewOutgoingMessage("ClueBatBot Connected!", server.CluebatBotChan))
	case *slack.MessageEvent:
		msg := ev.Msg
		found, err := msgStack.Search(msg.Channel, msg.Timestamp)
		if err != nil {
			if *debugCSlack {
				log.Printf("%s message stack search error %s\n", server.Name, err)
			}
		}
		if found {
			if *debugCSlack {
				log.Printf("%s found my own message in the stack", server.Name)
			}
		} else {
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
			log.Printf("%s found an unhandled event %v\n", server.Name, msg.Data)
		}
	}
}
