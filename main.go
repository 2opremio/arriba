package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/Sirupsen/logrus"
	humanize "github.com/dustin/go-humanize"
	"github.com/nlopes/slack"
)

const extractMsgGroupName = "msg"

// extract suffixes of Slack messages starting with @bot-name
var extractMsgPattern string = `(?m)^\s*<@%s>:?\s*(?P<` + extractMsgGroupName + `>.*)$`

// A standupMsg is a Slack message directed to the arriba bot (i.e. with a @botname prefix)
type standupMsg struct {
	ts   time.Time
	text string
}

// channelStandup contains the latest standup message of each user in a Slack channel.
type channelStandup map[string]standupMsg

// sortableChannelStandup is a channelStandup, sortable by the timestamp of its messages
// sortableChannelStandup implements sort.Interface to sort the keys of the channelStandup
type sortableChannelStandup struct {
	keys []string
	cs   channelStandup
}

func (s sortableChannelStandup) Swap(i, j int) { s.keys[i], s.keys[j] = s.keys[j], s.keys[i] }
func (s sortableChannelStandup) Len() int      { return len(s.keys) }
func (s sortableChannelStandup) Less(i, j int) bool {
	return s.cs[s.keys[i]].ts.After(s.cs[s.keys[j]].ts)
}

// getKeysByTimestamp returns the userIDs of the standup ordered by their message timestamp (newer first).
func (cs channelStandup) getKeysByTimestamp() []string {
	keys := make([]string, 0, len(cs))
	for k := range cs {
		keys = append(keys, k)
	}
	scs := sortableChannelStandup{
		cs:   cs,
		keys: keys,
	}
	sort.Sort(scs)
	return scs.keys
}

// standups contains the channelStandup of all Slack channels known to the bot.
type standups map[string]channelStandup

// channel provides a generic way to access the IDs, Names and history of both
// slack.Channel and slack.Group. Unfortunately nlopes/slack doesn't expose the
// underlying common type (groupConversation) and we cannot define methods for
// non-local types, which would allow to make things much cleaner ...
type channel struct {
	id, name   string
	getHistory func(*slack.RTM, slack.HistoryParameters) (*slack.History, error)
}

type arriba struct {
	rtm              *slack.RTM
	botID            string
	botName          string
	extractMsgRE     *regexp.Regexp
	historyDaysLimit int
	standups         standups
}

func newArriba(rtm *slack.RTM, historyDaysLimit int) arriba {
	return arriba{
		rtm:              rtm,
		historyDaysLimit: historyDaysLimit,
		standups:         make(standups),
	}
}

func parseSlackTimeStamp(ts string) (time.Time, error) {
	var seconds, milliseconds int64
	_, err := fmt.Sscanf(ts, "%d.%d", &seconds, &milliseconds)
	if err != nil {
		logrus.Warn("Can't parse timestamp ", ts)
		return time.Now(), err
	}
	return time.Unix(seconds, milliseconds*1000), nil
}

// extractStandupMsg parses Slack messages starting with @bot-name
func (a arriba) extractChannelStandupMsg(msg slack.Msg) (standupMsg, bool) {
	if msg.Type != "message" || msg.SubType != "" {
		return standupMsg{}, false
	}
	standupText := a.extractMsgRE.ReplaceAllString(msg.Text, "$"+extractMsgGroupName)
	if len(standupText) == len(msg.Text) {
		// Nothing was extracted
		return standupMsg{}, false
	}
	ts, err := parseSlackTimeStamp(msg.Timestamp)
	if err != nil {
		return standupMsg{}, false
	}
	return standupMsg{ts, standupText}, true
}

func (a arriba) retrieveChannelStandup(c channel) (channelStandup, error) {
	params := slack.NewHistoryParameters()
	params.Count = 1000
	params.Oldest = fmt.Sprintf(
		"%d",
		time.Now().UTC().AddDate(-a.historyDaysLimit, 0, 0).Unix(),
	)
	// It would be way more efficient to use slack.SearchMsgs instead
	// of traversing the whole history, but that's not allowed for bots :(
	cstandup := make(channelStandup)
	for {
		history, error := c.getHistory(a.rtm, params)
		if error != nil {
			return cstandup, error
		}

		for _, msg := range history.Messages {
			if _, ok := cstandup[msg.User]; ok {
				// we already have the latest standup message for this user
				continue
			}
			standupMsg, ok := a.extractChannelStandupMsg(msg.Msg)
			if ok && standupMsg.text != "" {
				cstandup[msg.User] = standupMsg
			}
		}

		if !history.HasMore {
			break
		}
		oldestMsg := history.Messages[len(history.Messages)-1]
		params.Oldest = oldestMsg.Timestamp
		params.Inclusive = false
	}
	return cstandup, nil
}

func (a arriba) retrieveStandups(channels []channel) {
	for _, channel := range channels {
		logrus.Infof("Retrieveing standup for channel #%s (%s)", channel.name, channel.id)
		cstandup, err := a.retrieveChannelStandup(channel)
		if err != nil {
			logrus.Errorf("Can't retrieve channel standup for channel #%s: %s", channel.name, err)
		}
		a.standups[channel.id] = cstandup
		logrus.Infof("Standup for channel #%s (%s) updated to %#v", channel.name, channel.id, cstandup)
	}
}

func (a arriba) getUserName(userID string) string {
	info, err := a.rtm.GetUserInfo(userID)
	userName := "id" + userID
	if err != nil {
		logrus.Errorf("Couldn't get user information for user %s: %s", userID, err)
	} else {
		userName = info.Name
	}
	return userName
}

func (a arriba) prettyPrintChannelStandup(cstandup channelStandup) string {
	text := "¡Ándale! ¡Ándale! here's the standup status :tada:\n"
	for _, userID := range cstandup.getKeysByTimestamp() {
		standupMsg := cstandup[userID]
		humanTime := humanize.Time(standupMsg.ts)
		userName := a.getUserName(userID)
		text += fmt.Sprintf("*%s*: %s _(%s)_\n", userName, standupMsg.text, humanTime)
	}
	return text
}

func (a arriba) sendStatus(channelID string) {
	var statusText string
	if cstandup, ok := a.standups[channelID]; ok && len(cstandup) > 0 {
		statusText = a.prettyPrintChannelStandup(cstandup)
	} else {
		statusText = fmt.Sprintf("No standup messages found\nType a message starting with *@%s* to record your standup message", a.botName)
	}
	a.rtm.SendMessage(a.rtm.NewOutgoingMessage(statusText, channelID))

}

func (a arriba) updateLastStandup(channelID, userID string, msg standupMsg) {
	if _, ok := a.standups[channelID]; !ok {
		a.standups[channelID] = make(channelStandup)
	}
	a.standups[channelID][userID] = msg
	confirmationText := fmt.Sprintf("<@%s>: ¡Yeppa! standup status recorded :taco:", userID)
	a.rtm.SendMessage(a.rtm.NewOutgoingMessage(confirmationText, channelID))
}

func (a *arriba) handleConnectedEvent(ev *slack.ConnectedEvent) {
	if a.botID != "" {
		logrus.Warn("Received unexpected Connected event")
		return
	}
	logrus.Infof(
		"Connected as user %s (%s) to team %s (%s)",
		ev.Info.User.Name,
		ev.Info.User.ID,
		ev.Info.Team.Name,
		ev.Info.Team.ID,
	)
	a.botID = ev.Info.User.ID
	a.botName = ev.Info.User.Name
	a.extractMsgRE = regexp.MustCompile(fmt.Sprintf(extractMsgPattern, a.botID))

	// Retrieve standups for public channels and private groups
	var channels []channel
	for _, c := range ev.Info.Channels {
		if c.IsMember {
			channel := channel{
				id:   c.ID,
				name: c.Name,
				getHistory: func(r *slack.RTM, params slack.HistoryParameters) (*slack.History, error) {
					return r.GetChannelHistory(c.ID, params)
				},
			}
			channels = append(channels, channel)
		}
	}
	for _, g := range ev.Info.Groups {
		channel := channel{
			id:   g.ID,
			name: g.Name,
			getHistory: func(r *slack.RTM, params slack.HistoryParameters) (*slack.History, error) {
				return r.GetGroupHistory(g.ID, params)
			},
		}
		channels = append(channels, channel)
	}
	a.retrieveStandups(channels)
}

func (a arriba) handleMessageEvent(ev *slack.MessageEvent) {
	logrus.Debugf("Message received %+v", ev)
	if a.botID == "" {
		logrus.Warn("Received message event before finishing initialization")
		return
	}
	if ev.Channel == "" {
		logrus.Warn("Received message with empty channel")
		return
	}
	switch ev.Channel[0] {
	case 'C', 'G':
		// Public and private (group) channels
		smsg, ok := a.extractChannelStandupMsg(ev.Msg)
		if !ok {
			return
		}
		logrus.Infof("Received standup message in channel %s: %+v", ev.Channel, smsg)
		if smsg.text == "" {
			a.sendStatus(ev.Msg.Channel)
		} else {
			a.updateLastStandup(ev.Msg.Channel, ev.Msg.User, smsg)
		}

	case 'D':
		// Direct messages are not supported yet
	}
}

func (a arriba) run() {
	go a.rtm.ManageConnection()

	for {
		select {
		case msg := <-a.rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.ConnectedEvent:
				a.handleConnectedEvent(ev)
			case *slack.MessageEvent:
				a.handleMessageEvent(ev)
			case *slack.RTMError:
				logrus.Error("Invalid credentials", ev.Error())
			case *slack.InvalidAuthEvent:
				logrus.Error("Invalid credentials")
				os.Exit(1)
			}
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [ flags ] <SlackAPItoken>\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "You can obtain <SlackAPItoken> from https://<yourteam>.slack.com/services/new/bot\n")
}

func main() {
	var (
		debug            bool
		historyDaysLimit int
	)

	flag.Usage = usage
	flag.BoolVar(&debug, "debug", false, "Print debug information")
	flag.IntVar(&historyDaysLimit, "history-limit", 7, "History limit (in days)")
	flag.Parse()
	if len(flag.Args()) < 1 || historyDaysLimit < 1 {
		usage()
		os.Exit(1)
	}

	logrus.SetOutput(os.Stderr)
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	api := slack.New(flag.Arg(0))
	api.SetDebug(debug)

	newArriba(api.NewRTM(), historyDaysLimit).run()
}
