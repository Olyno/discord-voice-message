package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"os"
	"path/filepath"
)

type CreateAttachmentResponse struct {
	Attachments []AttachmentResponseAttachment `json:"attachments"`
}

type AttachmentResponseAttachment struct {
	Id             int    `json:"id"`
	UploadUrl      string `json:"upload_url"`
	UploadFilename string `json:"upload_filename"`
}

type CreateAttachmentPayload struct {
	Files []CreateAttachmentFile `json:"files"`
}

type CreateAttachmentFile struct {
	FileName string `json:"filename"`
	FileSize int    `json:"file_size"`
	Id       string `json:"id"`
}

type VoiceMessagePayload struct {
	Content     string                   `json:"content"`
	ChannelId   string                   `json:"channel_id"`
	Type        int                      `json:"type"`
	Flags       int                      `json:"flags"`
	Attachments []VoiceMessageAttachment `json:"attachments"`
	Nonce       string                   `json:"nonce"`
}

type VoiceMessageAttachment struct {
	Id               string  `json:"id"`
	Filename         string  `json:"filename"`
	UploadedFilename string  `json:"uploaded_filename"`
	DurationSecs     float64 `json:"duration_secs"`
	Waveform         string  `json:"waveform"`
}

type DMChannelPayload struct {
	RecipientID string `json:"recipient_id"`
}

type DMChannelResponse struct {
	ID string `json:"id"`
}

var audioFileExtensions = []string{".mp3", ".wav", ".ogg", ".aac", ".flac"}

var mimeDict = map[string]string{
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".aac":  "audio/aac",
	".flac": "audio/flac",
}

type File struct {
	FileName   string
	FileType   string
	FileSize   int
	FileData   string
	UploadUrl  string
	UploadName string
}

// NewFile creates a new file object, takes a path as an argument
// automatically detects the file type and filename, calculates filesize,
// then reads and stores the file data
// returns a pointer to the new file object
func NewFile(path string) (*File, error) {
	var file File

	// First get the file name
	fileName := filepath.Base(path)
	file.FileName = fileName

	// Then get the file type and check if it's a valid audio file
	fileType := filepath.Ext(fileName)
	if !isStringInSlice(fileType, audioFileExtensions) {
		return nil, fmt.Errorf("invalid file type: %s", fileType)
	}
	file.FileType = fileType

	// Then get the file size
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()
	file.FileSize = int(fileSize)

	// Then read the file data
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file.FileData = string(fileData)

	return &file, nil
}

// CreateDMChannel creates or returns an existing DM channel for the given user ID.
// It returns the DM channel ID that can be used with the normal message endpoints.
func CreateDMChannel(token, userID string) (string, error) {
	url := "https://discord.com/api/v9/users/@me/channels"

	payload := DMChannelPayload{
		RecipientID: userID,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", errors.New("error marshalling json payload for create dm channel request")
	}

	headers := returnDiscordMobileCommonHeaders(token)

	client := resty.New()

	resp, err := client.R().
		SetHeaders(headers).
		SetBody(jsonPayload).
		Post(url)

	if err != nil {
		return "", errors.New("error sending create dm channel request to discord api")
	}

	body := resp.Body()
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return "", fmt.Errorf("discord returned HTTP %d: %s", resp.StatusCode(), string(body))
	}

	var dmChannelResponse DMChannelResponse
	err = json.Unmarshal(body, &dmChannelResponse)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling create dm channel response: %w | body: %s", err, string(body))
	}

	if dmChannelResponse.ID == "" {
		return "", fmt.Errorf("discord did not return a dm channel id; body: %s", string(body))
	}

	return dmChannelResponse.ID, nil
}

// CreateFile creates a new attachment in the specified channel, this is a blank attachment with no data
// Use PutFileData to upload the file data to the attachment
func (f *File) CreateFile(token, channel string) (CreateAttachmentResponse, error) {
	url := "https://discord.com/api/v9/channels/" + channel + "/attachments"

	// Create the payload
	payload := CreateAttachmentPayload{
		Files: []CreateAttachmentFile{
			{
				FileName: f.FileName,
				FileSize: f.FileSize,
				Id:       "0",
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return CreateAttachmentResponse{}, errors.New("error marshalling json payload for create attachment request")
	}

	headers := returnDiscordMobileCommonHeaders(token)

	client := resty.New()

	resp, err := client.R().
		SetHeaders(headers).
		SetBody(jsonPayload).
		Post(url)

	if err != nil {
		return CreateAttachmentResponse{}, errors.New("error sending create attachment request to discord api")
	}

	body := resp.Body()
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return CreateAttachmentResponse{}, fmt.Errorf("discord returned HTTP %d: %s", resp.StatusCode(), string(body))
	}

	var createAttachmentResponse CreateAttachmentResponse
	err = json.Unmarshal(body, &createAttachmentResponse)
	if err != nil {
		return CreateAttachmentResponse{}, fmt.Errorf("error unmarshalling create attachment response: %w | body: %s", err, string(body))
	}

	if len(createAttachmentResponse.Attachments) == 0 {
		return CreateAttachmentResponse{}, fmt.Errorf("discord did not return any attachments; body: %s", string(body))
	}

	f.UploadUrl = createAttachmentResponse.Attachments[0].UploadUrl
	f.UploadName = createAttachmentResponse.Attachments[0].UploadFilename

	return createAttachmentResponse, nil
}

// PutFileData uploads the file data to the attachment
// Use SendFile to send the attachment to the channel
func (f *File) PutFileData() error {
	url := f.UploadUrl

	headers := map[string]string{
		"accept-encoding": "gzip",
		"connection":      "Keep-Alive",
		"content-type":    mimeDict[f.FileType],
		"host":            "discord-attachments-uploads-prd.storage.googleapis.com",
		"user-agent":      "Discord-Android/175016;RNA",
	}

	client := resty.New()

	_, err := client.R().
		SetHeaders(headers).
		SetBody(f.FileData).
		Put(url)

	if err != nil {
		return errors.New("error sending put attachment request to discord api")
	}

	return nil
}

// SendFile sends a voice message file to a Discord channel using the Discord API
// It takes a Discord authentication token and a channel ID as input, encodes a byte array to base64 and creates a JSON payload to send a POST request to the Discord API with the voice message attachment.
// Returns an error if there is an issue with the request or JSON encoding.
func (f *File) SendFile(token, channel string) error {

	url := "https://discord.com/api/v9/channels/" + channel + "/messages"

	// make a byte array of 100 max value bytes
	var waveformBytes [100]byte

	// // FUNNY PENIS BYTES HAHA
	//
	// for i := 0; i < 15; i++ {
	// 	waveformBytes[i] = 255
	// }
	//
	// for i := 15; i < 50; i++ {
	// 	waveformBytes[i] = 126
	// }
	//
	// for i := 50; i < 55; i++ {
	// 	waveformBytes[i] = 255
	// }
	//
	// for i := 55; i < 60; i++ {
	// 	waveformBytes[i] = 200
	// }
	//
	// waveformBytes[60] = 100
	// waveformBytes[61] = 100

	// for i := 0; i < 100; i++ {
	// 	waveformBytes[i] = 255
	// }

	// encode the byte array to base64
	waveform := base64.StdEncoding.EncodeToString(waveformBytes[:])

	// Create the payload
	payload := VoiceMessagePayload{
		Content:   "",
		ChannelId: channel,
		Type:      0,
		Flags:     8192,
		Attachments: []VoiceMessageAttachment{
			{
				Id:               "0",
				Filename:         f.FileName,
				UploadedFilename: f.UploadName,
				DurationSecs:     100000.00,
				Waveform:         waveform,
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return errors.New("error marshalling json payload for send file request")
	}

	//fmt.Println(string(jsonPayload))

	headers := returnDiscordMobileCommonHeaders(token)

	client := resty.New()

	resp, err := client.R().
		SetHeaders(headers).
		SetBody(jsonPayload).
		Post(url)

	if err != nil {
		return errors.New("error sending send file request to discord api")
	}

	body := resp.Body()
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return fmt.Errorf("discord returned HTTP %d: %s", resp.StatusCode(), string(body))
	}

	return nil
}

func returnDiscordMobileCommonHeaders(token string) map[string]string {
	return map[string]string{
		"accept-encoding":    "gzip",
		"authorization":      token,
		"accept-language":    "en-US",
		"connection":         "Keep-Alive",
		"content-type":       "application/json",
		"host":               "discord.com",
		"user-agent":         "Discord-Android/175016;RNA",
		"x-debug-options":    "bugReporterEnabled",
		"x-discord-locale":   "en-US",
		"x-super-properties": "eyJvcyI6IkFuZHJvaWQiLCJicm93c2VyIjoiRGlzY29yZCBBbmRyb2lkIiwiZGV2aWNlIjoidmJveDg2cCIsInN5c3RlbV9sb2NhbGUiOiJlbi1VUyIsImNsaWVudF92ZXJzaW9uIjoiMTc1LjE2IC0gcm4iLCJyZWxlYXNlX2NoYW5uZWwiOiJnb29nbGVSZWxlYXNlIiwiZGV2aWNlX3ZlbmRvcl9pZCI6IjlhMDg5ZmRlLWFlYmUtNDIxZC05MjJlLWRlNDAyOGI1OTM5ZSIsImJyb3dzZXJfdXNlcl9hZ2VudCI6IiIsImJyb3dzZXJfdmVyc2lvbiI6IiIsIm9zX3ZlcnNpb24iOiIzMCIsImNsaWVudF9idWlsZF9udW1iZXIiOjE3NTAxNjAwODQ2MTIsImNsaWVudF9ldmVudF9zb3VyY2UiOm51bGwsImRlc2lnbl9pZCI6MH0=",
	}
}

func isStringInSlice(str string, slice []string) bool {
	for _, element := range slice {
		if element == str {
			return true
		}
	}
	return false
}
