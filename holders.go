package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bsproul/discord-stock-ticker/utils"
)

// Holders represents the json for holders
type Holders struct {
	Network   string   `json:"network"`
	Address   string   `json:"address"`
	Activity  string   `json:"activity"`
	Nickname  bool     `json:"set_nickname"`
	Frequency int      `json:"frequency"`
	ClientID  string   `json:"client_id"`
	token     string   `json:"-"`
	close     chan int `json:"-"`
}

// NewHolders saves information about the stock and starts up a watcher on it
func NewHolders(clientID string, network string, address string, activity string, token string, nickname bool, frequency int) *Holders {
	h := &Holders{
		Network:   network,
		Address:   address,
		Activity:  activity,
		Nickname:  nickname,
		Frequency: frequency,
		ClientID:  clientID,
		token:     token,
		close:     make(chan int, 1),
	}

	// spin off go routine to watch the price
	h.Start()
	return h
}

// Start begins watching holders
func (h *Holders) Start() {
	go h.watchHolders()
}

// Shutdown sends a signal to shut off the goroutine
func (h *Holders) Shutdown() {
	h.close <- 1
}

func (h *Holders) watchHolders() {

	// create a new discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + h.token)
	if err != nil {
		logger.Errorf("Error creating Discord session: %s\n", err)
		lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": "None"}).Set(0)
		return
	}

	// show as online
	err = dg.Open()
	if err != nil {
		logger.Errorf("error opening discord connection: %s\n", err)
		lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": "None"}).Set(0)
		return
	}

	// set activity as desc
	if h.Nickname {
		err = dg.UpdateGameStatus(0, h.Activity)
		if err != nil {
			logger.Errorf("Unable to set activity: %s\n", err)
		} else {
			logger.Debugf("Set activity")
		}
	}

	// get guides for bot
	guilds, err := dg.UserGuilds(100, "", "")
	if err != nil {
		logger.Errorf("Error getting guilds: %s\n", err)
		h.Nickname = false
	}

	// check for frequency override
	// set to one hour to avoid lockout
	if *frequency != 0 {
		h.Frequency = 3600
	}

	ticker := time.NewTicker(time.Duration(h.Frequency) * time.Second)
	var nickname string

	for {

		select {
		case <-h.close:
			logger.Infof("Shutting down price watching for %s", h.Activity)
			return
		case <-ticker.C:

			nickname = utils.GetHolders(h.Network, h.Address)

			if h.Nickname {

				for _, g := range guilds {

					err = dg.GuildMemberNickname(g.ID, "@me", nickname)
					if err != nil {
						logger.Errorf("Error updating nickname: %s\n", err)
						continue
					} else {
						logger.Debugf("Set nickname in %s: %s\n", g.Name, nickname)
					}
					logger.Infof("Set nickname in %s: %s\n", g.Name, nickname)
					lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": g.Name}).SetToCurrentTime()
					time.Sleep(time.Duration(h.Frequency) * time.Second)
				}
			} else {

				err = dg.UpdateGameStatus(0, nickname)
				if err != nil {
					logger.Errorf("Unable to set activity: %s\n", err)
				} else {
					logger.Debugf("Set activity: %s\n", nickname)
					lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": "None"}).SetToCurrentTime()
				}
			}
		}
	}
}
