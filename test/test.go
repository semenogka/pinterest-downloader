package main

import (
	"log"

	pin "github.com/semenogka/pinterest-downloader/v2"
)

func main() {
	err := pin.Client().SaveAudio("https://pin.it/3XI0InE3D", "video.mp4")
	if err != nil {
		log.Println(err)
	}
}