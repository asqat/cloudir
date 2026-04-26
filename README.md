# cloudir

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **Real-time directory sync with GoogleDrive**

`cloudir` is a fast, reliable, and cross-platform CLI utility that synchronizes a local directory with Google Drive in real-time. Think of it as `rsync` with a focus on cloud-native workflows and instant updates.

## ✨ Features

- 🔄 **Two-Way Sync**: Automatically synchronizes changes both from local-to-remote and remote-to-local.
- ⚡ **Real-Time Monitoring**: Uses `fsnotify` to detect and propagate local file changes instantly.
- 🧠 **Smart Conflict Resolution**: Choose between `latest` (timestamp-based), `local`, or `remote` strategies.
- 🗄️ **Persistent State**: SQLite-backed metadata tracking ensures efficient incremental updates and minimal API calls.
- 📦 **Large File Support**: Handles chunked uploads for large files and maintains full folder hierarchies.
- 🚀 **Performance**: Concurrent workers and efficient diffing (hashing + timestamps).
- 🛡️ **Reliability**: Automatic OAuth2 token refresh and exponential backoff for API retries.
- 📊 **UX**: Clean CLI output with real-time progress bars for active transfers.

## 🚀 Installation

Ensure you have [Go 1.25+](https://golang.org/dl/) installed.

```bash
# Clone the repository
git clone https://github.com/asqat/cloudir.git
cd cloudir

# Build the binary
go build -o cloudir ./cmd/cloudir

# Move to your path (optional)
mv cloudir /usr/local/bin/
```

## 🛠️ Setup & OAuth

To use `cloudir`, you need to set up a Google Cloud Project and obtain credentials.

1.  **Create a Project**: Go to the [Google Cloud Console](https://console.cloud.google.com/).
2.  **Enable API**: Search for **Google Drive API** and click **Enable**.
3.  **OAuth Consent Screen**:
    - Choose **External** User Type.
    - Add the scope: `.../auth/drive.file` (allows access to files created/opened by this app).
4.  **Create Credentials**:
    - Go to **Credentials** -> **Create Credentials** -> **OAuth client ID**.
    - Select **Desktop app** as the application type.
    - Download the JSON file and save it as `credentials.json` in your working directory.
5.  **Initialize**:
    ```bash
    ./cloudir init
    ```
    Follow the link in your browser, authorize the application, and paste the authorization code back into the terminal. This will create a `token.json` file.

## 📖 Usage

### 🔄 Real-Time Sync

Synchronize a local directory with a specific Google Drive folder ID.

```bash
./cloudir sync --dir ./my-folder --drive-folder-id "1abc123_YOUR_DRIVE_FOLDER_ID"
```

### 🔍 Check Status

View the currently tracked files and their sync status.

```bash
./cloudir status
```

### ⚙️ Command Options

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--dir` | **(Required)** Local path to synchronize. | - |
| `--drive-folder-id` | **(Required)** Google Drive folder ID. | - |
| `--conflict-strategy` | Conflict resolution: `local`, `remote`, `latest`. | `latest` |
| `--credentials` | Path to your `credentials.json` file. | `credentials.json` |
| `--interval` | Fallback polling interval for remote changes (sec). | `30` |
| `--dry-run` | Simulate changes without performing actual writes. | `false` |
| `--verbose` | Enable debug logging. | `false` |

## 📁 Project Structure

```text
├── cmd/cloudir        # Application entry point
├── internal/cli       # CLI commands (Cobra)
├── internal/drive     # Google Drive API integration
├── internal/sync      # Core sync engine logic
├── internal/watcher   # Local FS event monitoring (fsnotify)
├── internal/state     # SQLite-based state management
└── internal/config    # Configuration handling (Viper)
```

## 🚫 Ignoring Files

Standard ignores (like `.git`, `node_modules`, `.DS_Store`) are hardcoded. Support for a `.cloudirignore` file is coming soon.

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.
