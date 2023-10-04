package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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
var deviceId = spotify.ID("")

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
	devices, err := client.PlayerDevices(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range devices {
		if strings.Contains(d.Name, "pi") {
			deviceId = d.ID
		}
	}

	if deviceId.String() == "" {
		fmt.Printf("Device not found.")
	}

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
			if count <= len(tracks)-1 {
				pad.Light(j, i, 3, 3)
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
						pad.Light(j, i, 3, 3)
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
	if index >= len(tracks) {
		return
	}
	track := tracks[index]
	fmt.Printf("Playing track: %v\n", track.Track.Track.Name)
	err := client.QueueSongOpt(ctx, track.Track.Track.ID, &spotify.PlayOptions{DeviceID: &deviceId})
	if err != nil {
		log.Fatal(err)
	}

	queueIndex := -1
	count := 0
	fmt.Printf("Waiting for track to be in queue...\n")
	for queueIndex == -1 {
		queue, err := client.GetQueue(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for i, item := range queue.Items {
			fmt.Printf("-> %v is in queue, position: %v \n", item, index)
			if item.ID == track.Track.Track.ID {
				queueIndex = i
			}
		}
		time.Sleep(100)
		count++
		if count > 10 {
			log.Fatal("Song not in queue after 1 second")
		}
	}

	for i := 0; i <= queueIndex; i++ {
		fmt.Printf("Skipping track.\n")
		client.Next(ctx)
	}

	playback, err := client.PlayerState(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if playback.Playing == false {
		err = client.Play(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}

	// for playback.CurrentlyPlaying.Item.ID != track.Track.Track.ID {
	// 	err = client.Next(ctx)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	playback, err = client.PlayerState(ctx)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// }
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
