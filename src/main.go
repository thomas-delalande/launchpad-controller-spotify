package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rakyll/launchpad"
	spotifyauth "github.com/zmb3/spotify/v2/auth"

	"github.com/zmb3/spotify/v2"
)

const redirectURI = "http://localhost:8888/callback"

var (
	auth  = spotifyauth.New(spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopeUserModifyPlaybackState, spotifyauth.ScopeUserReadPlaybackState))
	ch    = make(chan *spotify.Client)
	state = ""
)

var tracks = []spotify.PlaylistItem{}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":8888", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)
	client := <-ch
	updateTracks(ctx, client)

	pad, err := launchpad.Open()
	if err != nil {
		fmt.Printf("Error initializing launchpad: %v", err)
		panic("")
	}
	defer pad.Close()

	pad.Clear()
	count := 0
	for i := 0; i <= 7; i++ {
		for j := 0; j <= 7; j++ {
			if count <= len(tracks) {
				pad.Light(i, j, 3, 3)
				count += 1
			}
		}
	}

	ch := pad.Listen()
	for {
		select {
		case hit := <-ch:
			count := 0
			updateTracks(ctx, client)
			for i := 0; i <= 7; i++ {
				for j := 0; j <= 7; j++ {
					if count <= len(tracks) {
						pad.Light(i, j, 3, 3)
						count += 1
					}
				}
			}
			playTrack(ctx, client, hit.X+8*hit.Y)
			pad.Light(hit.X, hit.Y, 3, 0)
		}
	}
}

func updateTracks(ctx context.Context, client *spotify.Client) {
	trackPage, err := client.GetPlaylistItems(
		ctx,
		spotify.ID("3SNkas6dOc7sA4bTD5zR6q"),
	)
	if err != nil {
		log.Fatal(err)
	}
	for page := 1; ; page++ {
		err = client.NextPage(ctx, trackPage)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}
	tracks = trackPage.Items
}

func playTrack(ctx context.Context, client *spotify.Client, index int) {
	fmt.Printf("Playing track at index %v", index)
	if index > len(tracks) {
		return
	}
	track := tracks[index]
	fmt.Printf("Playing track %v", track)
	devices, err := client.PlayerDevices(ctx)
	if err != nil {
		log.Fatal(err)
	}
	device := spotify.PlayerDevice{ID: ""}
	for _, d := range devices {
		if strings.Contains(d.Name, "pi") {
			device = d
		}
	}
	if device.ID == "" {
		log.Fatal("Raspberry Pi not found.")
	}
	err = client.TransferPlayback(ctx, device.ID, false)
	if err != nil {
		log.Fatal(err)
	}
	err = client.QueueSong(ctx, track.Track.Track.ID)
	if err != nil {
		log.Fatal(err)
	}
	err = client.Next(ctx)
	if err != nil {
		log.Fatal(err)
	}
	err = client.Play(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	// use the token to get an authenticated client
	client := spotify.New(auth.Client(r.Context(), tok))
	fmt.Fprintf(w, "Login Completed!")
	ch <- client
}
