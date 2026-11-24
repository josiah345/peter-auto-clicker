# Building Peter Auto Clicker

Requires Go 1.21+. The app is pure Win32 and has no dependencies.

```
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o PeterAutoClicker.exe .
```

`rsrc_windows_amd64.syso` holds the icon and manifest. To regenerate it: `rsrc -manifest app.manifest -ico icon.ico -arch amd64 -o rsrc_windows_amd64.syso`
