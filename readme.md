# Toddler Jukebox

Play Spotify music with a Launchpad and a Raspberry Pi.

## Why

I want a way for my children to pick their own music, while still playing it
from a Spotify playlist that I can update as their favourite songs change.

## Hardware

- Raspberry Pi (or any computer)
- Novation Launchpad S (haven't tried with any others)
- Aux cord
- Speakers

Connect the Launchpad and speakers to the Raspberry Pi. The Raspberry Pi acts as
the Spotify client (this makes it available as a device which can play music)
and also calles the

## How to run

1. Go to the [Spotify developer dashboard](https://developer.spotify.com/dashboard)
   and create a new app.
2. If your host computer (Raspberry Pi in my case), doesn't have a Spotify
   client. Download and run [Spotifyd](https://github.com/Spotifyd/spotifyd)
3. Install `portmidi` on the host computer to allow communication with the
   launchpad.
4. Clone this repository
5. Build with `go build src/app.go`
6. Run the app with

```
   ./app \
        -spotifyId={Client ID from Developer Dashboard} \
        -spotifySecret={Client Secret from Developer Dashboard} \
        -playlist={Playlist ID of your selected playlist}
        -device={Device name that will be playing the music}
```

6. The app will then print a URL that you can use to authenticate
7. If you authenticate on a different computer to your host one, copy the
   redirected url and execute `curl {redirected-url}` on the host computer.
8. Done!

This app is very simple and has very few abstractions (there's only one file).
It may be easier to just clone and edit to better suit your needs.
