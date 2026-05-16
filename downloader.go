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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	c "github.com/chromedp/chromedp"
	goffmpeg "github.com/semenogka/pinterest-downloader/v2/GO_ffmpeg"
	ts "github.com/semenogka/pinterest-downloader/v2/TS"
)

// urls represents a structure for video and audio URLs.
// There are two arrays: cmfv and m3u8. In them, all requests that can have ownership of the video are created.
// requestVideo and requestAudio are the final links to audio and video.
type INFO struct {
	requestsVideoCmfv []string
	requestsVideoM3U8 []string
	requestAudio      string
	requestVideo      string
	taked 			  bool
}

// Client creates a new Links instance.
func Client() *INFO {
	return &INFO{}
}

// DownloadFullVideo downloads a video with audio. The simplest way to download a video in one function.
// collects all requests, downloads video and audio and combines them into one separate file.
// then deletes unnecessary files.
//video quality: high, mid or low.
//logs for urls in console
func (ls *INFO) DownloadFullVideo(url string, outputFile string, quality string, logs bool) error {
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

	if len(ls.requestsVideoCmfv) != 0{
		err = ls.SaveVideo(ls.requestVideo, "temporaryVideoFile.mp4", quality)
		if err != nil {
			return err
		}

		if len(ls.requestAudio) != 0 {
			err = ls.SaveAudio("None","temporaryAudioFile.mp3")
			if err != nil {
				return err
			}
		}

		err = goffmpeg.MergeVideoAndAudio("temporaryVideoFile.mp4", "temporaryAudioFile.mp3", outputFile) 
		if err != nil {
			return err
		}
	} else {
		err = ls.SaveVideo(ls.requestVideo, outputFile, quality)
		if err != nil {
			return err
		}
	}
	return nil
}

// setupNetwork intercepts network requests using chromedp.
// thats all, i think.
func (ls *INFO) SetupNetwork(url string) error {
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
func (ls *INFO) UniqueUrls(cmfvMap map[string]int, m3u8Map map[string]int) {
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
func (ls *INFO) TakeRequests(url string, quality string) error {
	if err := ls.SetupNetwork(url); err != nil {
		return err
	}
	
	if len(ls.requestsVideoCmfv) == 0 {
		ls.requestVideo = checkQuality(ls.requestsVideoM3U8, quality)
	} else {
		ls.requestVideo = checkQuality(ls.requestsVideoCmfv, quality)
	}
	ls.taked = true
	return nil
}


func checkQuality(requestsVideo []string, quality string) string {
	var requestVideo string
	if quality == "high" {
		requestVideo =  requestsVideo[len(requestsVideo)-1]
	} else if quality == "mid" {
		requestVideo = requestsVideo[len(requestsVideo)/2]
	}else if quality == "low" {
		requestVideo = requestsVideo[0]
	}
	return requestVideo
}

// SaveVideo saves only the video.
func (ls *INFO) SaveVideo(url, outputFile, quality string) error {
	if !ls.taked{
		err := ls.TakeRequests(url, quality)
		if err != nil {
			return err
		}
	}
	if len(ls.requestsVideoCmfv) == 0 {
		err := ts.SaveTsVideo(ls.requestVideo, outputFile)
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}
	 } else {
		err := downloadFile(ls.requestVideo, outputFile)
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}
	}

	return nil
}

//saves the audio from the video.
func (ls *INFO) SaveAudio(url, outputFile string) error {
	if !ls.taked {
		err := ls.SetupNetwork(url)
		if err != nil {
			return err
		}
	}
	if len(ls.requestAudio) != 0 {
		err := downloadFile(ls.requestAudio, "temporaryAudioM4A.m4a")
		if err != nil {
			return fmt.Errorf("error while downloading: %v", err)
		}

		err = goffmpeg.ConvertToMP3("temporaryAudioM4A.m4a", outputFile)
		if err != nil {
			return err
		}

		err = os.Remove("temporaryAudioM4A.m4a")
		if err == nil {
			return err
		}

	}
	return nil
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

