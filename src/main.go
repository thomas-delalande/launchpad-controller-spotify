package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/rakyll/launchpad"
	spotifyauth "github.com/zmb3/spotify/v2/auth"

	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {
	ctx := context.Background()
	config := &clientcredentials.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		TokenURL:     spotifyauth.TokenURL,
	}
	token, err := config.Token(ctx)
	if err != nil {
		log.Fatalf("couldn't get token: %v", err)
	}
	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)
	msg, page, err := client.FeaturedPlaylists(ctx)
	if err != nil {
		log.Fatalf("couldn't get features playlists: %v", err)
	}

	fmt.Println(msg)
	for _, playlist := range page.Playlists {
		fmt.Println("  ", playlist.Name)
	}

	pad, err := launchpad.Open()
	if err != nil {
		fmt.Printf("Error initializing launchpad: %v", err)
		panic("")
	}
	defer pad.Close()

	pad.Clear()
	ch := pad.Listen()
	for {
		select {
		case hit := <-ch:
			pad.Light(hit.X, hit.Y, 3, 3)
		}
	}
}
