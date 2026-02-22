package main

import (
	"log"

	pin "github.com/semenogka/pinterest-downloader"
)

func main() {
	err := pin.Client().DownloadFullVideo("https://pin.it/6NgLdOHrQ", "video.mp4", "high", false)
	if err != nil {
		log.Println(err)
	}
}