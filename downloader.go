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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	c "github.com/chromedp/chromedp"
)

// urls represents a structure for video and audio URLs.
// There are two arrays: cmfv and m3u8. In them, all requests that can have ownership of the video are created.
// requestVideo and requestAudio are the final links to audio and video.
type URLS struct {
	requestsVideoCmfv []string
	requestsVideoM3U8 []string
	requestAudio      string
	requestVideo      string
}

// Client creates a new Links instance.
func Client() *URLS {
	return &URLS{}
}

// DownloadFullVideo downloads a video with audio. The simplest way to download a video in one function.
// collects all requests, downloads video and audio and combines them into one separate file.
// then deletes unnecessary files.
//video quality: high, mid or low.
//logs for urls in console
func (ls *URLS) DownloadFullVideo(url string, outputFile string, quality string, logs bool) error {
	
	err := ls.TakeRequests(url, quality)
	if err != nil {
		return err
	}
	if logs{
		log.Println(ls.requestAudio)
		log.Println(ls.requestsVideoCmfv)
		log.Println(ls.requestsVideoM3U8)
		log.Println(ls.requestVideo)
	}
	
	err = ls.SaveVideo(ls.requestVideo, "temporaryVideoFile.mp4")
	if err != nil {
		return err
	}

	err = ls.SaveAudio(ls.requestAudio, "temporaryAudioFile.mp3")
	if err != nil {
		return err
	}


	err = MergeVideoAndAudio("temporaryVideoFile.mp4", "temporaryAudioFile.mp3", outputFile) 
	if err != nil {
		return err
	}

	return nil
}

// setupNetwork intercepts network requests using chromedp.
// thats all, i think.
func (ls *URLS) setupNetwork(url string) error {
	cmfvMap := make(map[string]int)
	m3u8Map := make(map[string]int)
	// FIX: Add options to disable problematic features
	opts := append(c.DefaultExecAllocatorOptions[:],
		c.Flag("disable-features", "PrivateNetworkAccess"),
		c.Flag("disable-web-security", true),
		c.Flag("ignore-certificate-errors", true),
		c.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := c.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := c.NewContext(allocCtx)
	defer cancel()

	// Enable network
	if err := c.Run(ctx, network.Enable()); err != nil {
		return fmt.Errorf("network problem: %v", err)
	}

	// Listen for requests
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
			case *network.EventRequestWillBeSent:
				url := e.Request.URL
				if strings.HasSuffix(url, "cmfa") || strings.Contains(url, ".cmfa") {
					ls.requestAudio = url
				}
				if strings.HasSuffix(url, "cmfv") || strings.Contains(url, ".cmfv") {
					// ls.requestsVideoCmfv = append(ls.requestsVideoCmfv, url)
					num, err := strconv.Atoi(url[len(url)-9:len(url)-6]) 
					if err != nil {
						return
					}
					cmfvMap[url] = num
				}
				if strings.HasSuffix(url, "m3u8") || strings.Contains(url, ".m3u8") {
					num, err := strconv.Atoi(url[len(url)-9:len(url)-6]) 
					if err != nil {
						return
					}
					// ls.requestsVideoM3U8 = append(ls.requestsVideoM3U8, url)
					m3u8Map[url] = num
				}
		}
	})

	// Navigate to the page
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second), // Wait for page to load
	); err != nil {
		return err
	}

	ls.UniqueUrls(cmfvMap, m3u8Map)

	// Additional wait for resources
	time.Sleep(2 * time.Second)
	return nil
}

//make array of url with different image quality:low, mid, high
func (ls *URLS) UniqueUrls(cmfvMap map[string]int, m3u8Map map[string]int) {
	//u-url, q-quality
	type uq struct {
		Url   string
		Quality int
	}

	var cmfv []uq
	var m3u8 []uq
	for u, q := range cmfvMap {
		cmfv = append(cmfv, uq{u, q})
	}
	for u, q := range m3u8Map {
		m3u8 = append(m3u8, uq{u, q})
	}

	sort.Slice(cmfv, func(i, j int) bool  {
		return cmfv[i].Quality < cmfv[j].Quality
	})
	sort.Slice(m3u8, func(i, j int) bool  {
		return m3u8[i].Quality < m3u8[j].Quality
	})

	for _, cmfvStruct := range cmfv{
		ls.requestsVideoCmfv = append(ls.requestsVideoCmfv, cmfvStruct.Url)
	}
	for _, m3u8Struct := range m3u8{
		ls.requestsVideoM3U8 = append(ls.requestsVideoM3U8, m3u8Struct.Url)
	}

}


// TakeRequests finds video and audio links.
func (ls *URLS) TakeRequests(url string, quality string) error {
	if err := ls.setupNetwork(url); err != nil {
		return err
	}

	if len(ls.requestsVideoCmfv) == 0 {
		if quality == "high" {
			ls.requestVideo =  ls.requestsVideoM3U8[len(ls.requestsVideoM3U8)-1]
		} else if quality == "mid" {
			ls.requestVideo = ls.requestsVideoM3U8[len(ls.requestsVideoM3U8)/2]
		}else if quality == "low" {
			ls.requestVideo = ls.requestsVideoM3U8[0]
		}
	} else {
		if quality == "high" {
			ls.requestVideo =  ls.requestsVideoCmfv[len(ls.requestsVideoCmfv)-1]
		} else if quality == "mid" {
			ls.requestVideo = ls.requestsVideoCmfv[len(ls.requestsVideoCmfv)/2]
		}else if quality == "low" {
			ls.requestVideo = ls.requestsVideoCmfv[0]
		}
	}

	return nil
}

// SaveVideo saves only the video.
func (ls *URLS) SaveVideo(url, outputFile string) error {
	if len(ls.requestsVideoCmfv) == 0 {
		err := saveTsVideo(ls.requestVideo)
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}
	 } else {
		err := downloadFile(url, outputFile)
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}
	}

	return nil
}

// SaveAudio saves only the audio from the video.
func (ls *URLS) SaveAudio(url, outputFile string) error {
	err := downloadFile(url, "temporaryAudioM4A.m4a")
	if err != nil {
		return fmt.Errorf("error while downloading: %v", err)
	}

	err = convertToMP3("temporaryAudioM4A.m4a", outputFile)
	if err != nil {
		return  fmt.Errorf("error converting to mp3: %v", err)
	}
	os.Remove("temporaryAudioM4A.m4a")

	return nil
}

// convertToMP3 converts an audio file to MP3 format.
func convertToMP3(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, "-vn", "-acodec", "libmp3lame", "-b:a", "192k", outputFile)
	return cmd.Run()
}

// saveTsVideo processes the M3U8 playlist and saves the video in TS format.
func saveTsVideo(url string) error {
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

			tsPart, err := saveTsPart(requestTs, i)
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
func saveTsPart(url string, index int) (*os.File, error) {
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
func downloadFile(url, outputFile string) error {
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
func convertTSToMP4(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, outputFile)
	cmd.Run()

	os.Remove(inputFile)
	return nil 
}

// MergeVideoAndAudio merges video and audio into one file.
func MergeVideoAndAudio(videoFile, audioFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", videoFile, "-i", audioFile, outputFile)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Run() error: %v", err)
	}
	
	os.Remove(videoFile)
	os.Remove(audioFile)

	return nil 
}