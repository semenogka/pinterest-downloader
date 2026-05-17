package goffmpeg

import (
	"fmt"
	"os"
	"os/exec"
)

// convertTSToMP4 converts a TS file to MP4.
func ConvertTSToMP4(inputFile, outputFile string) error {
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

// convertToMP3 converts an audio file to MP3 format.
func ConvertToMP3(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, "-vn", "-acodec", "libmp3lame", "-b:a", "192k", outputFile)
	return cmd.Run()
}

