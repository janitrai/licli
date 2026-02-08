package cmd

import (
	"fmt"
	"os"

	"github.com/janitrai/bragcli/internal/api"
	"github.com/janitrai/bragcli/internal/auth"
	"github.com/janitrai/bragcli/internal/config"
)

func loadConfig() (config.Config, string, error) {
	path := cfgPath
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return config.Config{}, "", err
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, path, err
	}
	return cfg, path, nil
}

func saveConfig(path string, cfg config.Config) error {
	return config.Save(path, cfg)
}

func newBragnet(cfg config.Config) (*api.Bragnet, error) {
	cookies := auth.Cookies{
		LiAt:       cfg.Auth.LiAt,
		JSessionID: cfg.Auth.JSessionID,
	}
	if !cookies.Valid() {
		return nil, fmt.Errorf("not logged in (missing li_at/JSESSIONID). Run `li auth login`")
	}

	var opts []api.Option
	if debug {
		opts = append(opts, api.WithDebug(os.Stderr))
	}
	client, err := api.NewClient(cookies, opts...)
	if err != nil {
		return nil, err
	}
	li := api.NewBragnet(client)
	li.SearchQueryID = cfg.SearchQueryID
	li.ConversationsQueryID = cfg.ConversationsQueryID
	li.MessagesQueryID = cfg.MessagesQueryID
	return li, nil
}
