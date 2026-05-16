package main

import (
	"log"

	pin "github.com/semenogka/pinterest-downloader/v2"
)

func main() {
	err := pin.Client().DownloadFullVideo("some url", "video.mp4", "high", false)
	if err != nil {
		log.Println(err)
	}
}