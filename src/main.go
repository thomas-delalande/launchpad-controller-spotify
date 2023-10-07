package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/rakyll/launchpad"
	"golang.org/x/oauth2"
)

var tracks = []PlaylistItem{}
var deviceId = ""
var activeX = -1
var activeY = -1

func main() {
	spotifyId := flag.String("spotifyId", "", "Spotify Client ID")
	spotifySecret := flag.String("spotifySecret", "", "Spotify Secret ID")
	playlistId := flag.String("playlist", "3SNkas6dOc7sA4bTD5zR6q", "The Spotify Playlist ID")
	deviceName := flag.String("device", "Spotifyd@raspberrypi", "Spotify device to play music from")
	flag.Parse()

	if *spotifyId == "" {
		log.Fatalln("-spotifyId must be present")
	}

	if *spotifySecret == "" {
		log.Fatalln("-spotifySecret must be present")
	}

	var client *http.Client
	config := &oauth2.Config{
		ClientID:     *spotifyId,
		ClientSecret: *spotifySecret,
		RedirectURL:  fmt.Sprintf("http://localhost:8888/callback"),
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	urlCode := config.AuthCodeURL("")
	fmt.Printf("Please log in to Spotify by visiting the following page in your browser:\n %v\n", urlCode)

	handler := http.NewServeMux()
	server := http.Server{Addr: ":8888", Handler: handler}
	handler.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		code := values.Get("code")
		client = completeAuth(config, code)
		server.Shutdown(context.Background())
	})
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	fmt.Println("Successfully authenticated...")

	tracks = updateTracks(client, *playlistId)
	devices := getDevices(client)
	for _, d := range devices {
		if d.Name == *deviceName {
			deviceId = d.Id
		}
	}

	if deviceId == "" {
		fmt.Printf("Device not found.")
	}

	runLaunchpad(client, *playlistId)
}

type Track struct {
	Name string
	Id   string
}

type PlaylistItem struct {
	Track Track
}

func updateTracks(client *http.Client, playlistId string) []PlaylistItem {
	log.Printf("Getting latest tracks...")
	type PlayerlistItemsResponse struct {
		Items []PlaylistItem
	}
	response, err := client.Get(
		fmt.Sprintf("https://api.spotify.com/v1/playlists/%v/tracks?limit=50&offset=0", playlistId),
	)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	data := PlayerlistItemsResponse{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("	PlayerListItems[%v]\n", data.Items)
	return data.Items

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
		body, err := io.ReadAll(response.Body)

		data := QueueResponse{}
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("	Queue[%v]\n", data.Queue)
		queue := data.Queue

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

func completeAuth(config *oauth2.Config, code string) *http.Client {
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
	log.Println("Getting devices...")
	type DevicesResponse struct {
		Devices []Device
	}
	response, err := client.Get("https://api.spotify.com/v1/me/player/devices")
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	data := DevicesResponse{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Fetched devices[%v]\n", data.Devices)
	return data.Devices
}

func pause(client *http.Client) {
	log.Println("Requesting pause...")
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
	fmt.Printf("Playing track. name[%v] id[%v]\n", track.Name, track.Id)
	response, err := client.Post(fmt.Sprintf("https://api.spotify.com/v1/me/player/queue?uri=spotify:track:%v", track.Id), "application/json", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status[%v]\n", response.Status)
}

func transferPlayback(client *http.Client, deviceId string) {
	log.Printf("Transfering playback to device[%v]...\n", deviceId)
	body := []byte(fmt.Sprintf("{\"device_ids\":[\"%v\"]}", deviceId))
	request, err := http.NewRequest(http.MethodPut, "https://api.spotify.com/v1/me/player", bytes.NewBuffer(body))
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
	fmt.Println("Transfering playback complete. status[%v]", response.Status)
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

	body, err := io.ReadAll(response.Body)

	data := PlayerResponse{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("IsPlaying[%v]\n", data.IsPlaying)
	return data.IsPlaying
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

func runLaunchpad(client *http.Client, playlistId string) {
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
			tracks = updateTracks(client, playlistId)
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
