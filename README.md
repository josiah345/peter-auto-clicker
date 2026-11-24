# Peter Auto Clicker

<img src="logo.jpg" width="160" align="right" alt="Peter Auto Clicker logo">

A free Windows mouse recorder and auto clicker. It records the exact path and pace of your mouse (movements, clicks, scrolling and drags) and replays it once, a set number of times, or forever.

## Download

Get **PeterAutoClicker.zip** from the [Releases](../../releases/latest) page, unzip it and run `PeterAutoClicker.exe`.
Nothing to install, and it works offline.

> The first time you open it, Windows may say *"Windows protected your PC"* because the app is new and not code-signed. Click **More info → Run anyway**.

## How to use

| Button | Key | What it does |
|---|---|---|
| ● REC | F6 | Start recording, press again to finish |
| ▶ PLAY | F7 | Play the recording |
| ■ STOP | F8 | Stop |

| Icon | Meaning |
|---|---|
| 🔁 | How many times to repeat |
| ∞ | Loop forever (until F8) |
| ⏳ | Pause between loops (ms) |
| ⏩ | Speed (1 = real pace, 2 = twice as fast) |
| 🎯 Precise | Replays every movement at the exact recorded pace |
| ⚡ Smart | Use if it lags: keeps the rhythm and never misses a click |
| ✋ | Stop playback when you move the mouse yourself |
| ➖ | Hide the window while running |
| 📌 | Keep the window on top |
| 🌙 | Dark mode |
| 💾 📂 🗑 | Save, open and delete recordings |

Hover over any icon in the app for a tip.

## Good to know

- Only the mouse is recorded, not the keyboard.
- To click inside programs running as Administrator, run the app as Administrator too.
- Games that move the camera with the mouse often ignore replayed movement.
- Settings and your last recording are stored in `%APPDATA%\PeterAutoClicker`.

---

Free to download and use. © 2026 josiah345, all rights reserved: please don't re-upload or sell it. See [LICENSE](LICENSE). Built with Go ([notices](THIRD_PARTY_NOTICES.txt)).

For developers: see [BUILDING.md](BUILDING.md).
