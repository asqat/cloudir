package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Client struct {
	Service *drive.Service
}

func NewClient(ctx context.Context, credentialsPath, tokenPath string) (*Client, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	client := getClient(ctx, config, tokenPath)

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %v", err)
	}

	return &Client{Service: srv}, nil
}

func getClient(ctx context.Context, config *oauth2.Config, tokenFile string) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}
	return config.Client(ctx, tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (c *Client) UploadFile(ctx context.Context, name, parentID string, content io.Reader, mimeType string) (*drive.File, error) {
	f := &drive.File{
		Name:     name,
		Parents:  []string{parentID},
		MimeType: mimeType,
	}
	return c.Service.Files.Create(f).Media(content).Do()
}

func (c *Client) CreateDirectory(ctx context.Context, name, parentID string) (*drive.File, error) {
	f := &drive.File{
		Name:     name,
		Parents:  []string{parentID},
		MimeType: "application/vnd.google-apps.folder",
	}
	return c.Service.Files.Create(f).Do()
}

func (c *Client) UpdateFile(ctx context.Context, fileID string, content io.Reader) (*drive.File, error) {
	f := &drive.File{}
	return c.Service.Files.Update(fileID, f).Media(content).Do()
}

func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	return c.Service.Files.Delete(fileID).Do()
}

func (c *Client) ListFiles(ctx context.Context, folderID string) ([]*drive.File, error) {
	query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
	res, err := c.Service.Files.List().Q(query).Fields("files(id, name, md5Checksum, modifiedTime, mimeType)").Do()
	if err != nil {
		return nil, err
	}
	return res.Files, nil
}

func (c *Client) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	res, err := c.Service.Files.Get(fileID).Download()
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}
