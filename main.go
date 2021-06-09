package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	token string
)

const (
	filename = "game-choices.txt"
)

func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
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
}
