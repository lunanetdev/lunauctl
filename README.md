# Luna Update Control

A lightweight Windows utility written in Go that gives you full, persistent control over Windows Update. It disables or restores the update system through a layered approach — going far beyond simply toggling a service — to ensure the setting actually sticks.

---

## Why Luna Update Control?

Windows Update is notoriously difficult to disable permanently. Microsoft's **WaaSMedic** self-healing mechanism actively monitors and reverts changes made to update services, registry keys, and scheduled tasks. Luna Update Control counters this by applying multiple independent layers of blocking, so that even if one layer is bypassed, the others hold.

---

## Features

- **Full disable / restore** — a single menu choice applies or reverses all changes cleanly
- **Service management** — stops and disables all update-related services (`wuauserv`, `UsoSvc`, `WaaSMedicSvc`, `BITS`, `dosvc`)
- **WaaSMedic neutralisation** — renames self-healing executables to `.bak` and replaces the service DLL with a decoy folder, preventing Windows from loading it
- **Registry blocks** — sets Group Policy and AU registry keys to block update access and redirects the WU server to `localhost`
- **ACL locking** — modifies service and registry key Access Control Lists so that even `SYSTEM` cannot start or reconfigure update services while disabled
- **Logon-lock** — reassigns service logon accounts to a non-existent dummy account, causing logon to fail (error 1069) as an additional layer
- **Scheduled task control** — disables all Windows Update and Update Orchestrator scheduled tasks
- **Settings page hiding** — removes the Windows Update entry from the Settings app so it cannot be accessed by the user
- **Edge Update control** — disables Microsoft Edge's auto-updater by renaming its folder and placing a read-only decoy file in its place
- **Clean restore** — reverting re-enables every layer in the correct order, restoring the system to its original state
- **Detailed status view** — inspect the current state of every controlled component at a glance
- **Auto elevation** — automatically requests administrator privileges via UAC if not already elevated

---

## How It Works

Disabling Windows Update applies **12 sequential steps**:

1. Stop all update services
2. Set all update services to `Disabled`
3. Lock the `WaaSMedicSvc` registry key permissions (prevents self-healing race)
4. Neutralise WaaSMedic — rename executables to `.bak` and replace the DLL with a decoy folder
5. Apply Group Policy and AU registry blocks
6. Verify the `WaaSMedicSvc` Start value hasn't been flipped back
7. Hide Windows Update from the Settings app
8. Disable all related scheduled tasks
9. Switch service logon accounts to a dummy account (logon-lock)
10. Lock service ACLs (deny `SERVICE_START` and `SERVICE_CHANGE_CONFIG` to `SYSTEM`)
11. Lock registry key ACLs
12. Disable Edge Update

Restoring Windows Update reverses all steps in the correct order, ensuring ACLs and permissions are restored before services are re-enabled.

---

## Requirements

- **Administrator privileges** (the tool will prompt for elevation automatically via UAC)
- No additional runtime or dependencies required — the binary is fully self-contained

| Binary | Architecture | Supported OS |
|---|---|---|
| `lunauctl.exe` | 64-bit | Windows 10, Windows 11 |
| `lunauctl.exe-86.exe` | 32-bit | Windows 10 only |

---

## Building from Source

You need [Go](https://golang.org/dl/) installed.

```bash
git clone https://github.com/your-username/luna-update-control.git
cd luna-update-control
```

64-bit build:
```bash
go build -ldflags="-s -w" -o lunauctl.exe main.go
```

32-bit build:
```bash
GOARCH=386 go build -ldflags="-s -w" -o lunauctl.exe-86.exe main.go
```

---

## Usage

1. Download the correct binary for your system (see [Requirements](#requirements))
2. Double-click the executable — UAC will prompt for elevation if needed
3. Use the menu:

```
  [1]  Disable Windows Update
  [2]  Enable Windows Update
  [3]  Show detailed status
  [0]  Quit
```

A restart is recommended after applying changes to ensure all layers take full effect.

---

## Affected Components

| Component | Action when disabled |
|---|---|
| `wuauserv` | Stopped + disabled |
| `UsoSvc` | Stopped + disabled |
| `WaaSMedicSvc` | Stopped + disabled + DLL replaced with decoy |
| `BITS` | Stopped + disabled |
| `dosvc` | Stopped + disabled |
| `WaaSMedicAgent.exe` | Renamed to `.bak` |
| `MoUsoCoreWorker.exe` | Renamed to `.bak` |
| `UsoClient.exe` | Renamed to `.bak` |
| `WaaSMedicSvc.dll` | Renamed to `.bak`, replaced with decoy folder |
| Windows Update scheduled tasks | Disabled (13 tasks) |
| Group Policy / AU registry keys | Set to block update access |
| Service ACLs | Locked (SYSTEM denied start/config rights) |
| Service logon accounts | Redirected to non-existent dummy account |
| Settings page | Windows Update entry hidden |
| Microsoft Edge Update | Folder renamed, decoy file placed |

---

## Disclaimer

This tool makes low-level changes to Windows services, registry keys, file permissions, and ACLs. While it is fully reversible via the **Enable** option, use it with the understanding that:

- Keeping Windows Update disabled for extended periods may leave your system without security patches
- Some Microsoft features and applications may behave unexpectedly without update services running
- Always test in a non-production environment first

The authors take no responsibility for any system instability or data loss resulting from use of this tool.

---

## License

This project is released into the public domain under [The Unlicense](LICENSE). You are free to use, copy, modify, merge, publish, distribute, sublicense, or sell it without any restrictions.
