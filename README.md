# Discord Voice Message Uploader

A small terminal app for sending audio files as Discord voice messages. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Requirements

- **Go 1.24 or newer**
- A Discord account token
- An audio file in one of the supported formats: `.mp3`, `.wav`, `.ogg`, `.aac`, `.flac`

## Building

### 1. Clone the repository

```bash
git clone https://github.com/olyno/discord-voice-message.git
cd discord-voice-message
```

### 2. Build the application

```bash
go build -o discord-voice-message
```

Or run it directly without building an executable:

```bash
go run .
```

There are no system graphics libraries to install — the UI runs entirely in the terminal.

## Usage

1. **Start the app**:

   ```bash
   ./discord-voice-message
   ```

2. **Enter your Discord token** in the **Token** field and press **Ctrl+S** to save it. The token is stored in a local file named `token` with **0600** permissions (only your user can read it).

3. **Enter the target ID** in the **Channel / User ID** field:
   - For a server channel, enter the **Channel ID**.
   - For a direct message, press **Ctrl+D** to enable **Is DM** and enter the **User ID**.

4. **Enter the path** to your audio file in the **Audio File** field.

5. **Enter how many times** to send the voice message in the **Times to Send** field.

6. Press **Enter** to send.

### Keyboard shortcuts

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Move focus between fields |
| `Enter` | Send the voice message |
| `Ctrl+S` | Save the token to `token` |
| `Ctrl+D` | Toggle DM mode |
| `Ctrl+R` | Clear status/error messages |
| `Ctrl+C` / `Esc` | Quit |

### DM messages

When **Is DM** is enabled, the app calls the Discord API to create or open a DM channel with the provided user ID, then sends the voice message to that channel. You only need the recipient's user ID, not the DM channel ID.

## Security notes

- Your token is stored in a file with `0600` permissions and is never printed to logs or error messages.
- The token is cleared from the input field after you save it.
- Do not share the `token` file or commit it to version control. The included `.gitignore` already ignores it.
- A Discord token is sensitive: treat it like a password.

## Supported audio formats

- `.mp3`
- `.wav`
- `.ogg`
- `.aac`
- `.flac`

## License

This project is provided as-is for educational and personal use. Use it responsibly and in accordance with Discord's Terms of Service.
