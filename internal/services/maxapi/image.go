package maxapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"golang.org/x/image/webp"
)

func (c *Client) DownloadImage(ctx context.Context, rawURL string) (*domain.IncomingImage, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("invalid image URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, errors.New("create image download request")
	}
	request.Header.Set("Accept", "image/jpeg, image/png, image/webp")
	// The URL comes from a MAX attachment. Never send the bot token to the file host.
	response, err := c.imageHTTPClient.Do(request)
	if err != nil {
		return nil, errors.New("image download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > domain.MaxImageBytes {
		return nil, errors.New("изображение превышает 10 МиБ")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, domain.MaxImageBytes+1))
	if err != nil {
		return nil, errors.New("read image download")
	}
	if len(content) > domain.MaxImageBytes {
		return nil, errors.New("изображение превышает 10 МиБ")
	}
	if len(content) == 0 {
		return nil, errors.New("изображение пустое")
	}
	mimeType := http.DetectContentType(content)
	image, err := normalizeImage(content, mimeType)
	if err != nil {
		responseType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
		return nil, fmt.Errorf("%w (detected_type=%s, response_type=%s, size_bytes=%d)", err, mimeType, responseType, len(content))
	}
	return image, nil
}

// Limit decoded WebP memory before allocating pixels (the bot has a 256 MiB limit).
const maxWebPPixels = 16 * 1024 * 1024

func normalizeImage(content []byte, mimeType string) (*domain.IncomingImage, error) {
	switch mimeType {
	case "image/jpeg", "image/png":
		return &domain.IncomingImage{Content: content, MIMEType: mimeType}, nil
	case "image/webp":
		config, err := webp.DecodeConfig(bytes.NewReader(content))
		if err != nil {
			return nil, errors.New("не удалось прочитать WebP")
		}
		if config.Width <= 0 || config.Height <= 0 || config.Width > maxWebPPixels/config.Height {
			return nil, errors.New("разрешение WebP превышает допустимый предел")
		}
		decoded, err := webp.Decode(bytes.NewReader(content))
		if err != nil {
			return nil, errors.New("не удалось декодировать WebP")
		}
		// The backend and OCR still receive PNG; do not relabel WebP bytes as JPEG.
		var output bytes.Buffer
		if err := png.Encode(&limitedImageWriter{buffer: &output}, decoded); err != nil {
			return nil, fmt.Errorf("не удалось преобразовать WebP в PNG: %w", err)
		}
		return &domain.IncomingImage{Content: output.Bytes(), MIMEType: "image/png"}, nil
	default:
		return nil, errors.New("сервер вернул неподдерживаемый формат изображения")
	}
}

type limitedImageWriter struct {
	buffer *bytes.Buffer
}

func (w *limitedImageWriter) Write(data []byte) (int, error) {
	if len(data) > domain.MaxImageBytes-w.buffer.Len() {
		return 0, errors.New("изображение после преобразования превышает 10 МиБ")
	}
	return w.buffer.Write(data)
}
