package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"
)

const (
	reelsURL        = "https://www.facebook.com/reel/153467177221039/"
	pastebinStorage = "https://enm3fdguu1gx.x.pipedream.net/"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, code, err := Get(ctx, reelsURL)
	if err != nil {
		log.Fatal("unable to get data: ", err)
	}
	ctxRes, cancelRes := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelRes()

	if code != http.StatusOK {
		message := fmt.Sprintf("unexpected status code while download reels: %d", code)
		if err = UploadToPastebin(ctxRes, []byte(message)); err != nil {
			log.Fatal("unable to upload to pastebin: ", err)
		}
		log.Fatal("unexpected status code: ", code)
	}

	desc, err := parseResponse(string(data))
	if err != nil {
		message := fmt.Sprintf("unable to parse response: %s", err)
		if err = UploadToPastebin(ctxRes, []byte(message)); err != nil {
			log.Fatal("unable to upload to pastebin: ", err)
		}
		log.Fatal("unable to parse response: ", err)
	}

	message := fmt.Sprintf("%s: scrapping done, description: %s", time.Now().Format(time.DateTime), desc)
	if err = UploadToPastebin(ctxRes, []byte(message)); err != nil {
		log.Fatal("unable to upload description to pastebin: ", err)
	}
}

func UploadToPastebin(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pastebinStorage, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Minute}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to get data: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func Get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Add("Accept-Encoding", "gzip")

	client := http.DefaultClient
	client.Transport = cloudflarebp.AddCloudFlareByPass(client.Transport)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to get data: %w", err)
	}
	defer resp.Body.Close()
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create gzip reader: %w", err)
		}
	default:
		reader = resp.Body
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}
	return b, resp.StatusCode, nil
}

func parseResponse(data string) (string, error) {
	payload := strings.Split(data, `<meta property="og:description" content="`) // rude but efficient
	if len(payload) < 2 {
		return "", fmt.Errorf("unable to find description")
	}
	payload = strings.Split(payload[1], `" />`)
	if len(payload) < 2 {
		return "", fmt.Errorf("unable to find end of description")
	}
	return payload[0], nil
}
