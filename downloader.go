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

// Links представляет собой структуру для хранения URL видео и аудио.
type Links struct {
	requestsVideoCmfv []string
	requestsVideoM3U8 []string
	requestAudio      string
	requestVideo      string
}

// NewLinks создает новый экземпляр Links.
func NewLinks() *Links {
	return &Links{}
}

// Download загружает и обрабатывает видео и аудио.
func (ls *Links) Download(link string) error {
	if err := ls.setupNetwork(link); err != nil {
		return fmt.Errorf("ошибка настройки сети: %v", err)
	}

	if err := ls.download(); err != nil {
		return fmt.Errorf("ошибка загрузки и обработки: %v", err)
	}

	log.Println("Видео URL:", ls.requestVideo)
	log.Println("Аудио URL:", ls.requestAudio)

	// Очистка временных файлов
	os.Remove("audio.mp3")
	os.Remove("video.mp4")
	os.Remove("output.ts")

	return nil
}

// setupNetwork перехватывает сетевые запросы через chromedp.
func (ls *Links) setupNetwork(link string) error {
	os.Remove("output.mp4")

	ctx, cancel := c.NewContext(context.Background())
	defer cancel()

	if err := c.Run(ctx, network.Enable()); err != nil {
		return fmt.Errorf("ошибка включения сети: %v", err)
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
				log.Println(event.Request.URL)
			}
		}
	})

	if err := c.Run(ctx, c.Navigate(link)); err != nil {
		return fmt.Errorf("ошибка навигации: %v", err)
	}

	time.Sleep(10 * time.Second)
	return nil
}

// download обрабатывает и объединяет видео и аудио.
func (ls *Links) download() error {
	if len(ls.requestsVideoCmfv) == 0 {
		ls.requestVideo = ls.requestsVideoM3U8[len(ls.requestsVideoM3U8)-1]
		if err := ls.savetsvideo(ls.requestVideo); err != nil {
			return fmt.Errorf("ошибка сохранения TS видео: %v", err)
		}

		if err := ls.convertTSToMP4("output.ts", "output.mp4"); err != nil {
			return fmt.Errorf("ошибка конвертации TS в MP4: %v", err)
		}

	} else {
		ls.requestVideo = ls.requestsVideoCmfv[len(ls.requestsVideoCmfv)-1]
		if err := ls.saveVideo(ls.requestVideo, "video.mp4"); err != nil {
			return fmt.Errorf("ошибка сохранения видео: %v", err)
		}

		downloadPath := "downloaded_audio.m4a"
		outputPath := "audio.mp3"

		if err := ls.saveAudio(ls.requestAudio, downloadPath); err != nil {
			return fmt.Errorf("ошибка сохранения аудио: %v", err)
		}

		if err := ls.convertToMP3(downloadPath, outputPath); err != nil {
			return fmt.Errorf("ошибка конвертации аудио: %v", err)
		}
		os.Remove(downloadPath)

		if err := ls.mergeVideoAndAudio("video.mp4", "audio.mp3", "output.mp4"); err != nil {
			return fmt.Errorf("ошибка объединения видео и аудио: %v", err)
		}
	}

	return nil
}

// saveVideo сохраняет видео по указанному URL.
func (ls *Links) saveVideo(url, outputPath string) error {
	return ls.downloadFile(url, outputPath)
}

// saveAudio сохраняет аудио по указанному URL.
func (ls *Links) saveAudio(url, outputPath string) error {
	return ls.downloadFile(url, outputPath)
}

// convertToMP3 конвертирует аудиофайл в формат MP3.
func (ls *Links) convertToMP3(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, "-vn", "-acodec", "libmp3lame", "-b:a", "192k", outputFile)
	return cmd.Run()
}

// savetsvideo обрабатывает M3U8 плейлист и сохраняет видео в формате TS.
func (ls *Links) savetsvideo(url string) error {
	parts := strings.SplitN(url, "/", 10)
	prefixTsFile := strings.Join(parts[:9], "/") + "/"

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("ошибка загрузки M3U8: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения M3U8: %v", err)
	}

	log.Print(string(body))

	lines := strings.Split(string(body), "\n")
	output, err := os.Create("output.ts")
	if err != nil {
		return fmt.Errorf("ошибка создания output.ts: %v", err)
	}
	defer output.Close()

	for i, line := range lines {
		if strings.HasSuffix(line, ".ts") {
			requestTs := prefixTsFile + line
			log.Println(requestTs)

			tsPart, err := ls.saveTsPart(requestTs, i)
			if err != nil {
				return fmt.Errorf("ошибка сохранения TS части: %v", err)
			}

			part, err := os.Open(tsPart.Name())
			if err != nil {
				return fmt.Errorf("ошибка открытия TS части: %v", err)
			}

			if _, err := io.Copy(output, part); err != nil {
				return fmt.Errorf("ошибка копирования TS части: %v", err)
			}

			tsPart.Close()
		}
	}

	return nil
}

// saveTsPart сохраняет часть TS-файла.
func (ls *Links) saveTsPart(url string, index int) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки TS части: %v", err)
	}
	defer resp.Body.Close()

	nameFile := fmt.Sprintf("tsPart%d.ts", index)
	file, err := os.Create(nameFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания файла: %v", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, fmt.Errorf("ошибка копирования данных: %v", err)
	}

	return file, nil
}

// downloadFile загружает файл по URL и сохраняет его на диск.
func (ls *Links) downloadFile(url, outputPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("ошибка загрузки файла: %v", err)
	}
	defer resp.Body.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("ошибка создания файла: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("ошибка сохранения файла: %v", err)
	}

	return nil
}

// convertTSToMP4 конвертирует TS-файл в MP4.
func (ls *Links) convertTSToMP4(inputFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFile, outputFile)
	return cmd.Run()
}

// mergeVideoAndAudio объединяет видео и аудио в один файл.
func (ls *Links) mergeVideoAndAudio(videoFile, audioFile, outputFile string) error {
	cmd := exec.Command("ffmpeg", "-i", videoFile, "-i", audioFile, outputFile)
	return cmd.Run()
}