package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/textproto"
	"strconv"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string { return e.Message }

func encodeMessage(message domain.IncomingMessage) ([]byte, string, error) {
	if message.Image == nil {
		payload, err := json.Marshal(sendMessageRequest{
			MessageID: message.MessageID, Text: message.Text,
			ChatID: message.Target.ChatID, UserID: message.Target.UserID,
		})
		return payload, "application/json", err
	}
	image := message.Image
	if len(image.Content) == 0 || len(image.Content) > domain.MaxImageBytes {
		return nil, "", errors.New("invalid image size")
	}
	if image.MIMEType != "image/jpeg" && image.MIMEType != "image/png" {
		return nil, "", errors.New("unsupported image type")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"message_id": message.MessageID, "text": message.Text,
		"chat_id": strconv.FormatInt(message.Target.ChatID, 10),
		"user_id": strconv.FormatInt(message.Target.UserID, 10),
	} {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="image"`)
	header.Set("Content-Type", image.MIMEType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(image.Content); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}
