// backdrive uploads files to and downloads files from Google Drive folders.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const appName = "backdrive"

type configFile struct {
	Credentials json.RawMessage `json:"credentials"`
	Token       *oauth2.Token   `json:"token"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx := context.Background()
	var err error
	switch os.Args[1] {
	case "auth":
		if len(os.Args) < 2 || len(os.Args) > 3 {
			usage()
			os.Exit(2)
		}
		credentialsPath := ""
		if len(os.Args) == 3 {
			credentialsPath = os.Args[2]
		}
		err = authenticate(ctx, credentialsPath)
	case "dl":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		err = download(ctx, os.Args[2], os.Args[3])
	case "ul":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		err = upload(ctx, os.Args[2], os.Args[3])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "backdrive:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  backdrive auth [CLIENT_SECRET.json]
  backdrive dl FOLDER_ID FILE_PATTERN
  backdrive ul FOLDER_ID FILE_PATTERN

Quote FILE_PATTERN to prevent your shell from expanding it, e.g. '*.mkv'.
Run "backdrive auth" once before uploading or downloading.`)
}

// configPath is always the XDG config location. The XDG standard uses
// ~/.config when XDG_CONFIG_HOME is not set.
func configPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName, "config"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", appName, "config"), nil
}

// oauthClientCredentials obtains the client registration needed by Google to
// identify the application requesting Drive access.
func oauthClientCredentials(credentialsPath string) ([]byte, error) {
	if credentialsPath != "" {
		data, err := os.ReadFile(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("read OAuth client file: %w", err)
		}
		return data, nil
	}

	fmt.Fprintln(os.Stderr, "Create a Desktop OAuth client in Google Cloud Console (see README).")
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, "Google OAuth client ID: ")
	clientID, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read client ID: %w", err)
	}
	fmt.Fprint(os.Stderr, "Google OAuth client secret: ")
	clientSecret, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read client secret: %w", err)
	}
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" {
		return nil, errors.New("Google OAuth client ID cannot be empty")
	}
	return json.Marshal(map[string]any{"installed": map[string]any{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"auth_uri":      "https://accounts.google.com/o/oauth2/auth",
		"token_uri":     "https://oauth2.googleapis.com/token",
		"redirect_uris": []string{"http://localhost"},
	}})
}

// authenticate runs an installed-application OAuth flow and stores both the
// client configuration and refresh token in a file readable only by its owner.
func authenticate(ctx context.Context, credentialsPath string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	credentials, err := oauthClientCredentials(credentialsPath)
	if err != nil {
		return err
	}
	conf, err := google.ConfigFromJSON(credentials, drive.DriveScope)
	if err != nil {
		return fmt.Errorf("parse saved OAuth client configuration: %w", err)
	}
	token, err := getToken(ctx, conf)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(configFile{Credentials: credentials, Token: token}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Println("Authorization saved to", path)
	return nil
}

func getToken(ctx context.Context, conf *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start OAuth callback listener: %w", err)
	}
	defer listener.Close()

	state, err := randomState()
	if err != nil {
		return nil, err
	}
	// Do not modify the caller's configuration while installing the callback URI.
	localConf := *conf
	conf = &localConf
	conf.RedirectURL = "http://" + listener.Addr().String() + "/"
	url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Open this URL in a browser to authorize backdrive:\n" + url)
	if err := openBrowser(url); err != nil {
		fmt.Fprintln(os.Stderr, "Could not open a browser; open the URL above manually.")
	}

	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid OAuth state", http.StatusBadRequest)
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			http.Error(w, "authorization failed", http.StatusForbidden)
			resultCh <- result{err: fmt.Errorf("authorization failed: %s", e)}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			resultCh <- result{err: errors.New("OAuth callback did not include a code")}
			return
		}
		fmt.Fprintln(w, "Authorization complete. You may close this window.")
		resultCh <- result{code: code}
	})}
	go server.Serve(listener) // listener is closed below once a result arrives.

	select {
	case r := <-resultCh:
		_ = server.Close()
		if r.err != nil {
			return nil, r.err
		}
		token, err := conf.Exchange(ctx, r.code)
		if err != nil {
			return nil, fmt.Errorf("exchange authorization code: %w", err)
		}
		return token, nil
	case <-ctx.Done():
		_ = server.Close()
		return nil, ctx.Err()
	}
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "linux":
		command, args = "xdg-open", []string{url}
	default:
		return errors.New("unsupported platform")
	}
	return exec.Command(command, args...).Start()
}

func service(ctx context.Context) (*drive.Service, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not authorized; run: backdrive auth")
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var saved configFile
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(saved.Credentials) == 0 || saved.Token == nil {
		return nil, errors.New("config is missing credentials or token; run auth again")
	}
	conf, err := google.ConfigFromJSON(saved.Credentials, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("parse saved credentials: %w", err)
	}
	return drive.NewService(ctx, option.WithTokenSource(conf.TokenSource(ctx, saved.Token)))
}

func download(ctx context.Context, folderID, pattern string) error {
	svc, err := service(ctx)
	if err != nil {
		return err
	}
	files, err := remoteFiles(svc, folderID, pattern)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.MimeType == "application/vnd.google-apps.folder" {
			continue
		}
		if strings.HasPrefix(file.MimeType, "application/vnd.google-apps.") {
			return fmt.Errorf("%q is a Google Workspace document; native documents must be exported first", file.Name)
		}
		if err := downloadOne(svc, file); err != nil {
			return err
		}
	}
	fmt.Printf("Downloaded %d file(s).\n", len(files))
	return nil
}

func remoteFiles(svc *drive.Service, folderID, pattern string) ([]*drive.File, error) {
	query := fmt.Sprintf("'%s' in parents and trashed = false", strings.ReplaceAll(folderID, "'", "\\'"))
	call := svc.Files.List().Q(query).Fields("nextPageToken,files(id,name,mimeType,size)").SupportsAllDrives(true).IncludeItemsFromAllDrives(true).PageSize(1000)
	var found []*drive.File
	err := call.Pages(context.Background(), func(page *drive.FileList) error {
		for _, file := range page.Files {
			ok, err := filepath.Match(pattern, file.Name)
			if err != nil {
				return fmt.Errorf("invalid file pattern: %w", err)
			}
			if ok {
				found = append(found, file)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list folder: %w", err)
	}
	return found, nil
}

func downloadOne(svc *drive.Service, file *drive.File) error {
	name := filepath.Base(file.Name)
	if name == "." || name == string(filepath.Separator) {
		return fmt.Errorf("unsafe Drive file name %q", file.Name)
	}
	fmt.Printf("Downloading %s\n", name)
	response, err := svc.Files.Get(file.Id).SupportsAllDrives(true).Download()
	if err != nil {
		return fmt.Errorf("download %q: %w", file.Name, err)
	}
	defer response.Body.Close()
	tmp, err := os.CreateTemp(".", ".backdrive-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, response.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write %q: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, name); err != nil {
		return fmt.Errorf("save %q: %w", name, err)
	}
	return nil
}

func upload(ctx context.Context, folderID, pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("invalid file pattern: %w", err)
	}
	var files []string
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("pattern %q matched no regular files", pattern)
	}
	svc, err := service(ctx)
	if err != nil {
		return err
	}
	for _, path := range files {
		if err := uploadOne(svc, folderID, path); err != nil {
			return err
		}
	}
	fmt.Printf("Uploaded %d file(s).\n", len(files))
	return nil
}

func uploadOne(svc *drive.Service, folderID, path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	fmt.Printf("Uploading %s\n", path)
	file := &drive.File{Name: filepath.Base(path), Parents: []string{folderID}, ModifiedTime: info.ModTime().Format(time.RFC3339)}
	_, err = svc.Files.Create(file).Media(in, googleapi.ContentType("application/octet-stream")).SupportsAllDrives(true).Fields("id").Do()
	if err != nil {
		return fmt.Errorf("upload %q: %w", path, err)
	}
	return nil
}
