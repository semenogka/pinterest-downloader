package ts

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	goffmpeg "github.com/semenogka/pinterest-downloader/v3/GO_ffmpeg"
)

// saveTsVideo processes the M3U8 playlist and saves the video in TS format.
func SaveTsVideo(url string, outputFile string) error {
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

			tsPart, err := SaveTsPart(requestTs, i)
			if err != nil {
				return fmt.Errorf("error saving TS part: %v", err)
			}
			if _, err := io.Copy(output, tsPart); err != nil {
				return fmt.Errorf("error io.copy TS part: %v", err)
			}
		}
	}

	err = goffmpeg.ConvertTSToMP4("output.ts", outputFile)
	if err != nil {
		return err
	}

	output.Close()
	err = os.Remove("output.ts")
	if err != nil {
		return err
	}

	return nil
}

// saveTsPart saves a part of the TS file.
func SaveTsPart(url string, index int) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("TS part downloading error: %v", err)
	}

	return resp.Body, nil
}
