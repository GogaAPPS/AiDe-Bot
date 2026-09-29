package maxapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
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
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		return nil, errors.New("поддерживаются только JPEG и PNG")
	}
	return &domain.IncomingImage{Content: content, MIMEType: mimeType}, nil
}
