package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rakyll/launchpad"
	"golang.org/x/oauth2"
)

var tracks = []PlaylistItem{}
var deviceId = ""
var activeX = -1
var activeY = -1

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		code := values.Get("code")
		client = completeAuth2(config, code)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":8888", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	var client *http.Client
	config := &oauth2.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		RedirectURL:  "http://localhost:8888/callback",
		Scopes: []string{
			"user-read-private",
			"user-read-playback-state",
			"user-modify-playback-state",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}

	urlCode := config.AuthCodeURL("")
	fmt.Printf("Please log in to Spotify by visiting the following page in your browser: %v", urlCode)
	fmt.Scanln()

	tracks = updateTracks(client)
	devices := getDevices(client)
	for _, d := range devices {
		if strings.Contains(d.Name, "pi") {
			deviceId = d.Id
		}
	}

	if deviceId == "" {
		fmt.Printf("Device not found.")
	}

	pad, err := launchpad.Open()
	if err != nil {
		log.Fatalf("Error initializing launchpad: %v\n", err)
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
			tracks = updateTracks(client)
			for i := 0; i <= 7; i++ {
				for j := 0; j <= 7; j++ {
					if count <= len(tracks)-1 {
						pad.Light(j, i, 3, 3)
						count += 1
					}
				}
			}
			if hit.X == activeX && hit.Y == activeY {
				pause(client)
				activeX = -1
				activeY = -1
			} else {
				playTrack(client, hit.X+8*hit.Y)
				activeX = hit.X
				activeY = hit.Y
				pad.Light(hit.X, hit.Y, 3, 0)
			}
		}
	}
}

type Track struct {
	Name string
	Id   string
}

type PlaylistItem struct {
	Track Track
}

func updateTracks(client *http.Client) []PlaylistItem {
	type PlayerlistItemsResponse struct {
		items []PlaylistItem
	}
	response, err := client.Get(
		fmt.Sprintf("https://api.spotify.com/v1/playlists/%v/tracks?limit=50&offset=0", "3SNkas6dOc7sA4bTD5zR6q"),
	)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer response.Body.Close()

	var playlistItemsResponse PlayerlistItemsResponse
	json.NewDecoder(response.Body).Decode(playlistItemsResponse)
	return playlistItemsResponse.items

}

func playTrack(client *http.Client, index int) {
	if index >= len(tracks) {
		return
	}
	track := tracks[index]
	play(client, track.Track, deviceId)

	queueIndex := -1
	count := 0
	fmt.Printf("Waiting for track to be in queue...\n")
	for queueIndex == -1 {
		response, err := client.Get("https://api.spotify.com/v1/me/player/queue")
		if err != nil {
			log.Fatal(err)
		}
		type QueueResponse struct {
			Queue []Track
		}
		var queueResponse QueueResponse
		json.NewDecoder(response.Body).Decode(queueResponse)
		queue := queueResponse.Queue

		for i, item := range queue {
			fmt.Printf("-> %v is in queue, position: %v \n", item, i)
			if item.Id == track.Track.Id {
				queueIndex = i
			}
		}
		time.Sleep(200)
		count++
		if count >= 5 {
			fmt.Printf("Song not found in queue, transfering playback...\n")
			transferPlayback(client, deviceId)
			play(client, track.Track, deviceId)
		}
		if count > 20 {
			log.Fatal("Song not in queue after 2 seconds")
		}
	}

	for i := 0; i <= queueIndex; i++ {
		fmt.Printf("Skipping track.\n")
		next(client)
	}

	if !isPlaying(client) {
		startPlaying(client)

	}
}

func completeAuth2(config *oauth2.Config, code string) *http.Client {
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatal(err)
	}
	client := config.Client(context.Background(), token)
	return client
}

type Device struct {
	Id   string
	Name string
}

func getDevices(client *http.Client) []Device {
	type DevicesResponse struct {
		devices []Device
	}
	response, err := client.Get("https://api.spotify.com/v1/me/player/devices")
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	var deviceResponse DevicesResponse
	json.NewDecoder(response.Body).Decode(deviceResponse)
	return deviceResponse.devices
}

func pause(client *http.Client) {
	request, err := http.NewRequest(http.MethodPut, "https://api.spotify.com/v1/me/player/pause", nil)
	if err != nil {
		log.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
}

func play(client *http.Client, track Track, deviceId string) {
	fmt.Printf("Playing track: %v\n", track.Name)
	_, err := client.Post(fmt.Sprintf("https://api.spotify.com/v1/me/player/queue?uri=spotify:track:%v&device_id=%v", track.Id, deviceId), "application/json", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func transferPlayback(client *http.Client, deviceId string) {
	body := []byte(fmt.Sprintf("{\"device_ids\":[\"%v\"]}", deviceId))
	request, err := http.NewRequest(http.MethodPut, "https://api.spotify.com/v1/me/player/pause", bytes.NewBuffer(body))
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
}

func next(client *http.Client) {
	_, err := client.Post("https://api.spotify.com/v1/me/player/next", "application/json", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func isPlaying(client *http.Client) bool {
	type PlayerResponse struct {
		IsPlaying bool
	}

	response, err := client.Get("https://api.spotify.com/v1/me/player")
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	var body PlayerResponse
	json.NewDecoder(response.Body).Decode(body)
	return body.IsPlaying
}

func startPlaying(client *http.Client) {
	request, err := http.NewRequest(http.MethodPut, "https://api.spotify.com/v1/me/player/play", nil)
	if err != nil {
		log.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
}
