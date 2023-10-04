package main

import (
	"github.com/rakyll/launchpad"
	"fmt"
)

func main() {
	pad, err := launchpad.Open();
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
