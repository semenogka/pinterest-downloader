package main

import (
	"log"

	pin "github.com/semenogka/pinterest-downloader/v2"
)

func main() {
	err := pin.Client().DownloadFullVideo("some url", "video.mp4", "high", true)
	if err != nil {
		log.Println(err)
	}

	err = pin.Client().SaveOnlyAudio("some url", "audio.mp3")
	if err != nil {
		log.Println(err)
	}
}
