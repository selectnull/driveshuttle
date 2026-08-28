# DriveShuttle

A command-line tool that transfers files to/from a Google Drive folder. It
works with Google Workspace accounts and shared drives (provided the account
has access).

## Setup

1. In [Google Cloud Console](https://console.cloud.google.com/), create/select
   a project, enable **Google Drive API**, and configure the OAuth consent
   screen. For a Workspace account, an administrator may need to allow the
   app/scopes.
2. Create an OAuth client ID of type **Desktop app**. Copy its **Client ID**
   and **Client secret** from the Cloud Console. (Google requires every Drive
   OAuth app to have a client registration.)
3. Authorize the program once:

   ```sh
   # Recommended: use the JSON downloaded from Google Cloud Console
   driveshuttle auth client_secret.json

   # Or enter the client ID and secret interactively
   driveshuttle auth
   ```

   The program then opens a browser for Google sign-in and consent; if it
   cannot open one, copy the displayed URL into a browser. When no JSON
   filename is supplied, it prompts for the client ID and secret.

   If DriveShuttle is running on a remote server and its localhost callback is
   not exposed to the internet, run this on your local machine before opening
   the authorization URL (replace `PORT` with the port in that URL and `HOST`
   with the server):

   ```sh
   ssh -N -L PORT:127.0.0.1:PORT HOST
   ```

   The OAuth redirect through local `127.0.0.1:PORT` will then be forwarded to
   the server.

   The OAuth client configuration and refresh token are saved with permissions
   `0600` at `$XDG_CONFIG_HOME/driveshuttle/config`; when `XDG_CONFIG_HOME` is
   unset, the standard XDG default is `~/.config/driveshuttle/config`.

## Usage

```sh
# Download remote .mkv files in Drive folder FOLDER_ID into the current directory
driveshuttle download FOLDER_ID '*.mkv'

# Upload matching regular local files to that Drive folder
driveshuttle upload FOLDER_ID '*.mkv'
```

`dl` and `ul` are short aliases for `download` and `upload`, respectively.

Get `FOLDER_ID` from the Drive folder URL:
`https://drive.google.com/drive/folders/FOLDER_ID`. Quote patterns so the shell
does not expand them. Upload globbing is performed on local paths; download
globbing is performed against names directly in the specified Drive folder.
Existing local files with the same name are replaced by downloads. Uploads
always create new Drive files rather than replacing same-named ones.

Google-native files (Docs, Sheets, etc.) cannot be downloaded as raw files and
are reported as an error; export them from Drive first. Files in subdirectories
are not traversed.

## Build

Building requires Go 1.22 or newer and
[`just`](https://github.com/casey/just). The program is pure Go, so
cross-compilation needs no C toolchain.

```sh
# Build macOS and Linux binaries for AMD64 and ARM64
just build

# Build only macOS binaries
just build darwin

# Build only Linux binaries
just build linux
```

Binaries are written to `build/`. Remove all build output with:

```sh
just clean
```

On Linux, browser launching uses `xdg-open`; if unavailable, copy the
authorization URL printed by `driveshuttle auth` into a browser yourself.

## LICENSE

MIT licensed. This tool was vibe coded and as such is pretty much worthless.
