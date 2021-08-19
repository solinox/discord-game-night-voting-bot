package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

const (
	filename = "./game-choices.txt"
)

var draftees []string
var draftUnits map[string]string
var draftIndex int
var draftChannel string

func main() {
	token := os.Getenv("TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	if strings.HasPrefix(m.Content, "/game-night") {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error reading file: %s. But have fun at game night!", err))
			return
		}

		lines := strings.Split(string(data), "\n")
		reactions := make([]string, 0, len(lines))
		msg := ""
		for i, line := range lines {
			fields := strings.SplitN(line, " ", 2)
			if len(fields) != 2 {
				continue
			}
			reactions = append(reactions, fields[0])
			msg += line + "\n"

			if i > 0 && i%18 == 0 {
				newMsg, _ := s.ChannelMessageSend(m.ChannelID, msg)
				for _, reaction := range reactions {
					s.MessageReactionAdd(newMsg.ChannelID, newMsg.ID, reaction)
				}
				msg = ""
				reactions = make([]string, 0, len(lines))
			}
		}

		newMsg, _ := s.ChannelMessageSend(m.ChannelID, msg)
		for _, reaction := range reactions {
			s.MessageReactionAdd(newMsg.ChannelID, newMsg.ID, reaction)
		}
	}

	if strings.HasPrefix(m.Content, "/game-add") {
		fields := strings.SplitN(m.Content, " ", 3)[1:]

		if len(fields) != 2 {
			return
		}

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0766)
		if err != nil {
			return
		}
		defer file.Close()

		file.WriteString(fields[0] + " " + fields[1] + "\n")
		file.Sync()

		s.MessageReactionAdd(m.ChannelID, m.ID, ":+1:")
	}

	if strings.HasPrefix(m.Content, "/game-remove") {
		line := strings.TrimSpace(m.Content[12:])
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0766)
		if err != nil {
			return
		}
		defer file.Close()

		r := bufio.NewScanner(file)
		var w bytes.Buffer
		for r.Scan() {
			l := r.Text()
			if strings.Contains(l, line) {
				continue
			} else {
				w.WriteString(l + "\n")
			}
		}

		file.Seek(0, 0)
		file.Truncate(0)

		io.Copy(file, &w)
		file.Sync()

		s.MessageReactionAdd(m.ChannelID, m.ID, ":+1:")
	}

	if strings.HasPrefix(m.Content, "/draft-start") {
		line := strings.TrimSpace(m.Content[13:])
		names := strings.Fields(line)
		if len(names) <= 1 {
			s.ChannelMessageSend(m.ChannelID, "Not enough participants in the draft. Must be at least 2 people")
			return
		}
		draftees = names
		draftIndex = 0
		draftChannel = m.ChannelID
		handleNextDraft(s)
	}

	if strings.HasPrefix(m.Content, "/draft-end") {
		unitsByOwner := make(map[string][]string)
		for unit, owner := range draftUnits {
			unitsByOwner[owner] = append(unitsByOwner[owner], unit)
		}
		msg := "Draft has ended. Selections are as follows:"
		for owner, units := range unitsByOwner {
			msg += "\n" + owner + ":"
			msg += strings.Join(units, ", ")
		}
		s.ChannelMessageSend(draftChannel, msg)
		draftees, draftUnits, draftIndex, draftChannel = nil, nil, 0, ""
	}

	if strings.HasPrefix(m.Content, "/draft") {
		if draftChannel == "" {
			s.ChannelMessageSend(m.ChannelID, "Draft is not started. Use /draft-start <names of draftees> to start a draft")
			return
		}
		unit := strings.ToLower(strings.TrimSpace(m.Content[7:]))
		currentDraftee := draftees[draftIndex]
		if !strings.HasSuffix(m.Author.Username, currentDraftee) {
			s.ChannelMessageSend(m.ChannelID, "current draftee is "+currentDraftee+", not "+m.Author.Username)
			return
		}
		if owner, ok := draftUnits[unit]; ok && owner != currentDraftee {
			s.ChannelMessageSend(draftChannel, "Try again, that is already owned by "+owner)
			return
		} else if ok && owner == currentDraftee {
			// todo add count
		}
		draftUnits[unit] = currentDraftee
		draftIndex++
		handleNextDraft(s)
	}
}

func handleNextDraft(s *discordgo.Session) {
	if draftIndex >= len(draftees) {
		draftIndex = 0
		// reverse
		for i, j := 0, len(draftees)-1; i < len(draftees)/2; i, j = i+1, j-1 {
			draftees[i], draftees[j] = draftees[j], draftees[i]
		}
	}
	draftee := draftees[draftIndex]
	msg := fmt.Sprintf("It is %s's turn to make a selection. Enter /draft <name> to select. To end the draft, enter /draft-end", draftee)
	s.ChannelMessageSend(draftChannel, msg)
}
