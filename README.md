# backdrive

A small command-line uploader/downloader for a Google Drive folder. It works with Google Workspace accounts and shared drives (provided the account has access).

## Setup

1. In [Google Cloud Console](https://console.cloud.google.com/), create/select a project, enable **Google Drive API**, and configure the OAuth consent screen. For a Workspace account, an administrator may need to allow the app/scopes.
2. Create an OAuth client ID of type **Desktop app**. Copy its **Client ID** and **Client secret** from the Cloud Console. (Google requires every Drive OAuth app to have a client registration.)
3. Authorize the program once:

   ```sh
   # Recommended: use the JSON downloaded from Google Cloud Console
   backdrive auth client_secret.json

   # Or enter the client ID and secret interactively
   backdrive auth
   ```

   The program then opens a browser for Google sign-in and consent; if it cannot open one, copy the displayed URL into a browser. When no JSON filename is supplied, it prompts for the client ID and secret.

   The OAuth client configuration and refresh token are saved with permissions `0600` at `$XDG_CONFIG_HOME/backdrive/config`; when `XDG_CONFIG_HOME` is unset, the standard XDG default is `~/.config/backdrive/config`.

## Usage

```sh
# Download remote .mkv files in Drive folder FOLDER_ID into the current directory
backdrive dl FOLDER_ID '*.mkv'

# Upload matching regular local files to that Drive folder
backdrive ul FOLDER_ID '*.mkv'
```

Get `FOLDER_ID` from the Drive folder URL: `https://drive.google.com/drive/folders/FOLDER_ID`. Quote patterns so the shell does not expand them. Upload globbing is performed on local paths; download globbing is performed against names directly in the specified Drive folder. Existing local files with the same name are replaced by downloads. Uploads always create new Drive files rather than replacing same-named ones.

Google-native files (Docs, Sheets, etc.) cannot be downloaded as raw files and are reported as an error; export them from Drive first. Files in subdirectories are not traversed.

## Build

Requires Go 1.22 or newer:

```sh
go build -o backdrive .
```

### Cross-compile macOS and Linux

The program is pure Go, so cross-compilation needs no C toolchain:

```sh
# macOS Apple Silicon and Intel
GOOS=darwin GOARCH=arm64 go build -o backdrive-darwin-arm64 .
GOOS=darwin GOARCH=amd64 go build -o backdrive-darwin-amd64 .

# Linux x86-64 and ARM64
GOOS=linux GOARCH=amd64 go build -o backdrive-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o backdrive-linux-arm64 .
```

On Linux, browser launching uses `xdg-open`; if unavailable, copy the authorization URL printed by `backdrive auth` into a browser yourself.
