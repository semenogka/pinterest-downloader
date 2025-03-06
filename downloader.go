//Pinterest videos are stored in two formats: m3u8 and cmfv
//I still haven't fully understood the principle by which some videos are given m3u8 or cmfv.
//If anyone doesn't know, m3u8 is like a dictionary that contains a video split into several parts.
//this code implements the installation of these parts and the connection into one.
//conversion and joining of files occurs via ffmpeg, so you must have it on your computer.

package pin

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	c "github.com/chromedp/chromedp"
)

// Links represents a structure for video and audio URLs.
// There are two arrays: cmfv and m3u8. In them, all requests that can have ownership of the video are created.
// requestVideo and requestAudio are the final links to audio and video.
type Links struct {
	requestsVideoCmfv []string
	requestsVideoM3U8 []string
	requestAudio      string
	requestVideo      string
}

// Client creates a new Links instance.
func Client() *Links {
	return &Links{}
}

// DownloadFullVideo downloads a video with audio. The simplest way to download a video in one function.
// collects all requests, downloads video and audio and combines them into one separate file.
// then deletes unnecessary files.
func (ls *Links) DownloadFullVideo(link string, videoFile string, audioFile string, outputFile string) error {
	
	err := ls.TakeRequests(link)
	if err != nil {
		return err
	}
	
	err = ls.SaveVideo(ls.requestVideo, videoFile)
	if err != nil {
		return err
	}

	err = ls.SaveAudio(ls.requestAudio, audioFile)
	if err != nil {
		return err
	}


	ls.MergeVideoAndAudio(videoFile, audioFile, outputFile)


	// Cleaning temporary files
	os.Remove(audioFile)
	os.Remove(videoFile)
	os.Remove("output.ts")
	os.Remove("audio.m4a")

	return nil
}

// setupNetwork intercepts network requests using chromedp.
// thats all, i think.
func (ls *Links) setupNetwork(link string) error {

	ctx, cancel := c.NewContext(context.Background())
	defer cancel()

	if err := c.Run(ctx, network.Enable()); err != nil {
		return fmt.Errorf("network problem: %v", err)
	}

	c.ListenTarget(ctx, func(ev any) {
		event, ok := ev.(*network.EventRequestWillBeSent)
		if ok {
			if strings.HasSuffix(event.Request.URL, "cmfa") {
				ls.requestAudio = event.Request.URL
			}
			if strings.HasSuffix(event.Request.URL, "cmfv") {
				ls.requestsVideoCmfv = append(ls.requestsVideoCmfv, event.Request.URL)
			}
			if strings.HasSuffix(event.Request.URL, "m3u8") {
				ls.requestsVideoM3U8 = append(ls.requestsVideoM3U8, event.Request.URL)
			}
		}
	})

	if err := c.Run(ctx, c.Navigate(link)); err != nil {
		return fmt.Errorf("navigation error: %v", err)
	}

	time.Sleep(3 * time.Second)
	return nil
}

// TakeRequests finds video and audio links.
func (ls *Links) TakeRequests(link string) error {
	if err := ls.setupNetwork(link); err != nil {
		return err
	}

	if len(ls.requestsVideoCmfv) == 0 {
		ls.requestVideo = ls.requestsVideoM3U8[len(ls.requestsVideoM3U8)-1]
	} else {
		ls.requestVideo = ls.requestsVideoCmfv[len(ls.requestsVideoCmfv)-1]
	}

	return nil
}

// SaveVideo saves only the video.
func (ls *Links) SaveVideo(url, outputFile string) error {
	if len(ls.requestsVideoCmfv) == 0 {
		err := ls.saveTsVideo(ls.requestVideo)
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}
	 } else {
		err := ls.downloadFile(url, outputFile)
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}
	}

	return nil
}

// SaveAudio saves only the audio from the video.
func (ls *Links) SaveAudio(url, outputFile string) error {
	err := ls.downloadFile(url, "audio.m4a")
	if err != nil {
		return fmt.Errorf("error while downloading: %v", err)
	}

	err = ls.convertToMP3("audio.m4a", outputFile)
	if err != nil {
		return  fmt.Errorf("error converting to mp3: %v", err)
	}
	os.Remove("audio.m4a")
	return nil
}

// convertToMP3 converts an audio file to MP3 format.
func (ls *Links) convertToMP3(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, "-vn", "-acodec", "libmp3lame", "-b:a", "192k", outputFile)
	return cmd.Run()
}

// saveTsVideo processes the M3U8 playlist and saves the video in TS format.
func (ls *Links) saveTsVideo(url string) error {
	parts := strings.SplitN(url, "/", 10)
	prefixTsFile := strings.Join(parts[:9], "/") + "/"

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("M3U8 downloading error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("M3U8 read error: %v", err)
	}

	log.Print(string(body))

	lines := strings.Split(string(body), "\n")
	output, err := os.Create("output.ts")
	if err != nil {
		return fmt.Errorf("error creating output.ts: %v", err)
	}
	defer output.Close()
	//output.ts is a summary file with all the segments 

	for i, line := range lines {
		if strings.HasSuffix(line, ".ts") {
			requestTs := prefixTsFile + line
			log.Println(requestTs)

			tsPart, err := ls.saveTsPart(requestTs, i)
			if err != nil {
				return fmt.Errorf("error saving TS part: %v", err)
			}

			part, err := os.Open(tsPart.Name())
			if err != nil {
				return fmt.Errorf("error opening TS part: %v", err)
			}

			if _, err := io.Copy(output, part); err != nil {
				return fmt.Errorf("error io.copy TS part: %v", err)
			}

			tsPart.Close()
		}
	}

	return nil
}

// saveTsPart saves a part of the TS file.
func (ls *Links) saveTsPart(url string, index int) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("TS part downloading error: %v", err)
	}
	defer resp.Body.Close()

	nameFile := fmt.Sprintf("tsPart%d.ts", index)
	file, err := os.Create(nameFile)
	if err != nil {
		return nil, fmt.Errorf("file creation error: %v", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, fmt.Errorf("io.copy error: %v", err)
	}

	return file, nil
}

// downloadFile downloads a file from a URL and saves it to disk.
func (ls *Links) downloadFile(url, outputFile string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("cant get url: %v", err)
	}
	defer resp.Body.Close()

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creation file: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("io.copy error: %v", err)
	}

	return nil
}

// convertTSToMP4 converts a TS file to MP4.
func (ls *Links) convertTSToMP4(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, outputFile)
	return cmd.Run()
}

// MergeVideoAndAudio merges video and audio into one file.
func (ls *Links) MergeVideoAndAudio(videoFile, audioFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", videoFile, "-i", audioFile, outputFile)
	return cmd.Run()
}


