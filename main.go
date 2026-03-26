package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	appName    = "Luna Update Control"
	appVersion = "v0.1 beta 1"
	sep        = "=================================================="
)

var updateServices = []string{
	"wuauserv",
	"UsoSvc",
	"WaaSMedicSvc",
	"BITS",
	"dosvc",
}

var scheduledTasks = []string{
	`\Microsoft\Windows\WindowsUpdate\Automatic App Update`,
	`\Microsoft\Windows\WindowsUpdate\Scheduled Start`,
	`\Microsoft\Windows\WindowsUpdate\sih`,
	`\Microsoft\Windows\WindowsUpdate\sihboot`,
	`\Microsoft\Windows\UpdateOrchestrator\Schedule Scan`,
	`\Microsoft\Windows\UpdateOrchestrator\Schedule Scan Static Task`,
	`\Microsoft\Windows\UpdateOrchestrator\USO_UxBroker`,
	`\Microsoft\Windows\UpdateOrchestrator\Report policies`,
	`\Microsoft\Windows\UpdateOrchestrator\StartInstall`,
	`\Microsoft\Windows\UpdateOrchestrator\Reboot`,
	`\Microsoft\Windows\UpdateOrchestrator\Schedule Wake To Work`,
	`\Microsoft\Windows\UpdateOrchestrator\Schedule Work`,
	`\Microsoft\Windows\WaaSMedic\PerformRemediation`,
}

var medicExes = []string{
	`C:\Windows\System32\WaaSMedicAgent.exe`,
	`C:\Windows\System32\MoUsoCoreWorker.exe`,
	`C:\Windows\System32\UsoClient.exe`,
}

// ── Logon-lock ────────────────────────────────────────────────────────────────

// Original logon accounts — restored exactly by unlockServiceLogon
var serviceLogonAccounts = map[string]struct{ obj, password string }{
	"wuauserv":     {"LocalSystem", ""},
	"UsoSvc":       {"LocalSystem", ""},
	"WaaSMedicSvc": {"LocalSystem", ""},
	"BITS":         {"LocalSystem", ""},
	"dosvc":        {"NT AUTHORITY\\NetworkService", ""},
}

// A dummy local account that does not exist — logon will always fail (error 1069)
const dummyAccount = `.\WUpdateDisabledUser`
const dummyPassword = "Disabled!By!Script!99"

// ── Service ACL descriptors ───────────────────────────────────────────────────

// Default Windows service security descriptors (clean-install baseline)
var serviceDefaultSD = map[string]string{
	"wuauserv": "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)",
	"UsoSvc": "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)",
	"WaaSMedicSvc": "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)",
	"BITS": "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)",
	"dosvc": "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)",
}

// Locked SD: denies SERVICE_START and SERVICE_CHANGE_CONFIG to SYSTEM, IU, SU.
// Only Administrators (BA) retain full control, so only this script can reverse it.
var serviceLockedSD = map[string]string{
	"wuauserv": "D:(D;;RPWPDTSD;;;SY)(D;;RPWPDTSD;;;IU)(D;;RPWPDTSD;;;SU)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)" +
		"(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)",
	"UsoSvc": "D:(D;;RPWPDTSD;;;SY)(D;;RPWPDTSD;;;IU)(D;;RPWPDTSD;;;SU)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)" +
		"(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)",
	"WaaSMedicSvc": "D:(D;;RPWPDTSD;;;SY)(D;;RPWPDTSD;;;IU)(D;;RPWPDTSD;;;SU)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)" +
		"(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)",
	"BITS": "D:(D;;RPWPDTSD;;;SY)(D;;RPWPDTSD;;;IU)(D;;RPWPDTSD;;;SU)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)" +
		"(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)",
	"dosvc": "D:(D;;RPWPDTSD;;;SY)(D;;RPWPDTSD;;;IU)(D;;RPWPDTSD;;;SU)" +
		"(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)" +
		"(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)",
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	printBanner()
	checkAdmin()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n" + sep)
		showStatus()
		fmt.Println(sep)
		fmt.Println()
		fmt.Println("  [1]  Disable Windows Update ")
		fmt.Println("  [2]  Enable Windows Update ")
		fmt.Println("  [3]  Show detailed status")
		fmt.Println("  [0]  Quit")
		fmt.Println()
		fmt.Print("  Your choice: ")

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			disableUpdate()
			askRestart(reader)
		case "2":
			enableUpdate()
			askRestart(reader)
		case "3":
			showDetailedStatus()
			pause(reader)
		case "0", "q":
			fmt.Println("\n  Goodbye!")
			os.Exit(0)
		default:
			fmt.Println("\n  Invalid choice.")
		}
	}
}

func printBanner() {
	fmt.Println()
	fmt.Println(" " + sep)
	fmt.Printf("   %s  %s\n", appName, appVersion)
	fmt.Println("   Disable or restore Windows Update")
	fmt.Println(" " + sep)
	fmt.Println()
}

// ── Admin elevation ───────────────────────────────────────────────────────────

func checkAdmin() {
	if isAdmin() {
		return
	}
	elevate()
	os.Exit(0)
}

func isAdmin() bool {
	out, err := runOut("whoami", "/groups")
	return err == nil && strings.Contains(out, "S-1-16-12288")
}

func elevate() {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	exe, _ := os.Executable()
	verb, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	dir, _ := syscall.UTF16PtrFromString(".")

	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exePtr)),
		0,
		uintptr(unsafe.Pointer(dir)),
		1,
	)

	if ret <= 32 {
		fmt.Println("  [!] Elevation failed. Right-click and choose 'Run as administrator'.")
		fmt.Print("\n  Press Enter to exit...")
		bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

// ── Status ────────────────────────────────────────────────────────────────────

func showStatus() {
	disabled := isDisabled()
	if disabled {
		fmt.Println("  Windows Update status:  [ DISABLED ] ")
	} else {
		fmt.Println("  Windows Update status:  [ ENABLED ] ")
	}
}

func isDisabled() bool {
	out, err := runOut("sc", "query", "wuauserv")
	if err != nil {
		return false
	}
	out2, _ := runOut("sc", "qc", "wuauserv")
	return strings.Contains(out2, "DISABLED") ||
		(!strings.Contains(out, "RUNNING") && strings.Contains(out2, "DEMAND_START") == false)
}

func showDetailedStatus() {
	fmt.Println("\n" + sep)
	fmt.Println("  DETAILED STATUS")
	fmt.Println(sep)

	fmt.Println("\n  Services:")
	for _, svc := range updateServices {
		out, _ := runOut("sc", "qc", svc)
		startType := "unknown"
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "START_TYPE") {
				startType = strings.TrimSpace(line)
				break
			}
		}
		fmt.Printf("    %-20s %s\n", svc, startType)
	}

	fmt.Println("\n  Executables (WaaSMedic):")
	for _, path := range medicExes {
		base := path[strings.LastIndex(path, `\`)+1:]
		bak := path + ".bak"
		outBak, _ := runOut("cmd", "/c", "if exist \""+bak+"\" echo EXISTS")
		outExe, _ := runOut("cmd", "/c", "if exist \""+path+"\" echo EXISTS")
		if strings.Contains(outBak, "EXISTS") {
			fmt.Printf("    RENAMED+READONLY (disabled): %s\n", base)
		} else if strings.Contains(outExe, "EXISTS") {
			fmt.Printf("    ACTIVE:                      %s\n", base)
		} else {
			fmt.Printf("    NOT FOUND:                   %s\n", base)
		}
	}

	fmt.Println("\n  WaaSMedicSvc DLL:")
	{
		dll := waaSMedicDLL
		bak := waaSMedicDLLBak
		outDir, _ := runOut("cmd", "/c", "if exist \""+dll+"\\*\" echo DIR")
		outBak, _ := runOut("cmd", "/c", "if exist \""+bak+"\" echo YES")
		outDll, _ := runOut("cmd", "/c", "if exist \""+dll+"\" echo YES")
		switch {
		case strings.Contains(outDir, "DIR"):
			fmt.Println("    DECOY FOLDER in place (DLL blocked) ")
		case strings.Contains(outBak, "YES"):
			fmt.Println("    RENAMED to .bak (no decoy folder yet)")
		case strings.Contains(outDll, "YES"):
			fmt.Println("    ACTIVE (not neutralised)")
		default:
			fmt.Println("    NOT FOUND")
		}
	}

	fmt.Println("\n  Registry (AU Policy):")
	out, _ := runOut("reg", "query",
		`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`,
		"/v", "NoAutoUpdate")
	if strings.Contains(out, "NoAutoUpdate") {
		fmt.Println("    NoAutoUpdate = 1  (policy set)")
	} else {
		fmt.Println("    NoAutoUpdate not set (update may be enabled)")
	}

	fmt.Println("\n  Settings page visibility:")
	vis, _ := runOut("reg", "query",
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\Explorer`,
		"/v", "SettingsPageVisibility")
	if strings.Contains(vis, "windowsupdate") {
		fmt.Println("    Windows Update page: HIDDEN ")
	} else {
		fmt.Println("    Windows Update page: VISIBLE")
	}

	fmt.Println("\n  Edge Update:")
	edgeDir := edgeUpdateBasePath()
	edgeDisable := edgeDir + "2"
	if _, err := os.Stat(edgeDisable); err == nil {
		fmt.Println("    DISABLED (EdgeUpdate2 present — real updater is renamed away)")
	} else if _, err2 := os.Stat(edgeDir); err2 == nil {
		fmt.Println("    ENABLED (EdgeUpdate folder present)")
	} else {
		fmt.Println("    NOT FOUND (EdgeUpdate folder missing)")
	}

	fmt.Println("\n  Service logon accounts:")
	for _, svc := range updateServices {
		out, _ := runOut("sc", "qc", svc)
		logon := "unknown"
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "SERVICE_START_NAME") {
				logon = strings.TrimSpace(line)
				break
			}
		}
		fmt.Printf("    %-20s %s\n", svc, logon)
	}
}

// ── Disable ───────────────────────────────────────────────────────────────────

func disableUpdate() {
	fmt.Println("\n" + sep)
	fmt.Println("  DISABLING WINDOWS UPDATE")
	fmt.Println(sep)

	fmt.Println("\n  [1/11] Stopping services...")
	for _, svc := range updateServices {
		run("sc", "stop", svc)
		fmt.Printf("        Stopped: %s\n", svc)
	}
	time.Sleep(1 * time.Second)

	fmt.Println("\n  [2/11] Disabling services...")
	for _, svc := range updateServices {
		run("sc", "config", svc, "start=", "disabled")
		fmt.Printf("        Disabled: %s\n", svc)
	}

	// Lock the WaaSMedicSvc registry key BEFORE writing Start=4.
	// If we write Start=4 first, WaaSMedic may race and flip it back to 3
	// before we get a chance to deny SYSTEM write access.
	fmt.Println("\n  [3/11] Locking WaaSMedicSvc registry key permissions...")
	lockWaaSMedicRegistryPerms()

	fmt.Println("\n  [4/11] Neutralizing WaaSMedic (self-healing blocker)...")
	killWaaSMedic()

	fmt.Println("\n  [5/11] Applying registry blocks...")
	applyRegistry()

	fmt.Println("\n  [6/11] Verifying WaaSMedicSvc Start value is still 4...")
	verifyWaaSMedicStartValue()

	fmt.Println("\n  [7/11] Hiding Windows Update from Settings...")
	hideSettingsPage()

	fmt.Println("\n  [8/11] Disabling scheduled tasks...")
	for _, task := range scheduledTasks {
		run("schtasks", "/Change", "/TN", task, "/DISABLE")
	}
	fmt.Println("        Done.")

	fmt.Println("\n  [9/11] Changing service logon accounts (logon-lock)...")
	lockServiceLogon()

	fmt.Println("\n  [10/11] Locking service ACLs...")
	lockServiceACLs()

	fmt.Println("\n  [11/12] Locking registry key ACLs...")
	lockRegistryACLs()

	fmt.Println("\n  [12/12] Disabling Edge Update...")
	disableEdgeUpdate()

	fmt.Println()
	fmt.Println("   Windows Update has been disabled.")
}

// ── Enable ────────────────────────────────────────────────────────────────────

func enableUpdate() {
	fmt.Println("\n" + sep)
	fmt.Println("  RESTORING WINDOWS UPDATE")
	fmt.Println(sep)

	fmt.Println("\n  [1/11] Restoring registry key ACLs...")
	unlockRegistryACLs()

	fmt.Println("\n  [2/11] Restoring WaaSMedicSvc registry key permissions...")
	unlockWaaSMedicRegistryPerms()

	fmt.Println("\n  [3/11] Restoring service ACLs...")
	unlockServiceACLs()

	fmt.Println("\n  [4/11] Restoring service logon accounts...")
	unlockServiceLogon()

	fmt.Println("\n  [5/11] Re-enabling services...")
	for _, svc := range updateServices {
		startType := "demand"
		if svc == "BITS" || svc == "dosvc" {
			startType = "auto"
		}
		run("sc", "config", svc, "start=", startType)
		fmt.Printf("        Enabled: %s\n", svc)
	}

	fmt.Println("\n  [6/11] Starting services...")
	for _, svc := range []string{"BITS", "wuauserv", "UsoSvc", "dosvc"} {
		run("sc", "start", svc)
		fmt.Printf("        Started: %s\n", svc)
	}

	fmt.Println("\n  [7/11] Restoring WaaSMedic executables...")
	restoreWaaSMedic()

	fmt.Println("\n  [8/11] Removing registry blocks...")
	removeRegistry()

	fmt.Println("\n  [9/11] Restoring Windows Update in Settings...")
	showSettingsPage()

	fmt.Println("\n  [10/11] Re-enabling scheduled tasks...")
	for _, task := range scheduledTasks {
		run("schtasks", "/Change", "/TN", task, "/ENABLE")
	}
	fmt.Println("        Done.")

	fmt.Println("\n  [11/12] Starting WaaSMedicSvc...")
	run("sc", "start", "WaaSMedicSvc")
	fmt.Println("        Done.")

	fmt.Println("\n  [12/12] Restoring Edge Update...")
	enableEdgeUpdate()

	fmt.Println()
	fmt.Println("   Windows Update has been restored.")
}

// ── WaaSMedic ─────────────────────────────────────────────────────────────────

func killWaaSMedic() {
	run("sc", "config", "WaaSMedicSvc", "start=", "disabled")

	for _, path := range medicExes {
		bak := path + ".bak"
		base := path[strings.LastIndex(path, `\`)+1:]

		// Check if already renamed
		outBak, _ := runOut("cmd", "/c", "if exist \""+bak+"\" echo YES")
		if strings.Contains(outBak, "YES") {
			// Already renamed — ensure read-only is set anyway
			run("attrib", "+R", bak)
			fmt.Printf("        Already renamed (read-only ensured): %s\n", base)
			continue
		}

		// Check if exe exists
		outExe, _ := runOut("cmd", "/c", "if exist \""+path+"\" echo YES")
		if !strings.Contains(outExe, "YES") {
			fmt.Printf("        Not found (skip): %s\n", base)
			continue
		}

		// Take ownership + grant permissions
		run("takeown", "/f", path, "/a")
		run("icacls", path, "/grant", "Administrators:F", "/c")

		// Rename to .bak
		_, err := runOut("cmd", "/c", "rename \""+path+"\" \""+base+".bak\"")
		if err != nil {
			fmt.Printf("        Warning - could not rename: %s\n", base)
			continue
		}

		// Set read-only on the .bak so it can't be renamed back or executed easily
		run("attrib", "+R", bak)
		fmt.Printf("        Renamed → .bak + read-only set: %s\n", base)
	}

	// Block via registry as well
	run("reg", "add",
		`HKLM\SYSTEM\CurrentControlSet\Services\WaaSMedicSvc`,
		"/v", "Start", "/t", "REG_DWORD", "/d", "4", "/f")

	// Rename WaaSMedicSvc.dll → .bak, then create a same-named folder in its
	// place.  Windows cannot load a directory as a DLL, so the service cannot
	// start even if every other block is somehow bypassed.
	killWaaSMedicDLL()
}

const waaSMedicDLL = `C:\WINDOWS\System32\WaaSMedicSvc.dll`
const waaSMedicDLLBak = `C:\WINDOWS\System32\WaaSMedicSvc.dll.bak`

func killWaaSMedicDLL() {
	dll := waaSMedicDLL
	bak := waaSMedicDLLBak

	// Kill any process that may be holding the DLL open.
	run("taskkill", "/F", "/IM", "WaaSMedicAgent.exe")
	run("taskkill", "/F", "/IM", "MoUsoCoreWorker.exe")
	run("taskkill", "/F", "/IM", "UsoClient.exe")
	run("taskkill", "/F", "/IM", "WaaSMedic.exe")
	run("sc", "stop", "WaaSMedicSvc")
	time.Sleep(2 * time.Second)

	// Take ownership and grant Administrators full control.
	run("takeown", "/f", dll, "/a")
	run("icacls", dll, "/grant", "Administrators:F", "/c")

	// Remove read-only in case it was set.
	run("attrib", "-R", dll)

	// Remove read-only from .bak in case it exists from a previous run.
	run("attrib", "-R", bak)

	// Rename DLL → .bak using Go's os.Rename (no cmd/shell quoting issues).
	err := os.Rename(dll, bak)
	if err != nil {
		fmt.Printf("        Warning - could not rename WaaSMedicSvc.dll: %v\n", err)
	} else {
		run("attrib", "+R", bak)
		fmt.Println("        WaaSMedicSvc.dll renamed → .bak + read-only set.")
	}

	// Small delay to let the filesystem settle after the rename.
	time.Sleep(2 * time.Second)

	// Create decoy folder using os.Mkdir (no shell).
	err2 := os.Mkdir(dll, 0755)
	if err2 != nil {
		fmt.Printf("        Warning - could not create decoy folder: %v\n", err2)
	} else {
		run("attrib", "+R", dll)
		fmt.Println("        Decoy folder created: WaaSMedicSvc.dll (folder, read-only).")
	}
}

func restoreWaaSMedicDLL() {
	dll := waaSMedicDLL
	bak := waaSMedicDLLBak

	// Remove decoy folder if present
	outDir, _ := runOut("cmd", "/c", "if exist \""+dll+"\\*\" echo DIR")
	if strings.Contains(outDir, "DIR") {
		run("attrib", "-R", dll)
		err := os.Remove(dll)
		if err != nil {
			fmt.Printf("        Warning - could not remove decoy folder: %v\n", err)
			return
		}
		fmt.Println("        Decoy folder removed: WaaSMedicSvc.dll.")
	}

	// Restore .bak → DLL
	outBak, _ := runOut("cmd", "/c", "if exist \""+bak+"\" echo YES")
	if strings.Contains(outBak, "YES") {
		run("attrib", "-R", bak)
		_, err := runOut("cmd", "/c", "rename \""+bak+"\" \"WaaSMedicSvc.dll\"")
		if err != nil {
			fmt.Println("        Warning - could not restore WaaSMedicSvc.dll")
			run("attrib", "+R", bak) // re-lock if restore failed
		} else {
			fmt.Println("        WaaSMedicSvc.dll restored from .bak.")
		}
	} else {
		fmt.Println("        WaaSMedicSvc.dll.bak not found (skip).")
	}
}

func restoreWaaSMedic() {
	for _, path := range medicExes {
		bak := path + ".bak"
		base := path[strings.LastIndex(path, `\`)+1:]

		outBak, _ := runOut("cmd", "/c", "if exist \""+bak+"\" echo YES")
		if !strings.Contains(outBak, "YES") {
			fmt.Printf("        Not renamed (skip): %s\n", base)
			continue
		}

		// Strip read-only before attempting rename
		run("attrib", "-R", bak)

		_, err := runOut("cmd", "/c", "rename \""+bak+"\" \""+base+"\"")
		if err != nil {
			fmt.Printf("        Warning - could not restore: %s\n", base)
			// Re-apply read-only if restore failed so it stays locked
			run("attrib", "+R", bak)
		} else {
			fmt.Printf("        Restored (read-only removed): %s\n", base)
		}
	}

	// Restore the DLL (must happen before re-enabling the service)
	restoreWaaSMedicDLL()

	run("reg", "add",
		`HKLM\SYSTEM\CurrentControlSet\Services\WaaSMedicSvc`,
		"/v", "Start", "/t", "REG_DWORD", "/d", "3", "/f")
}

// ── Service logon lock ────────────────────────────────────────────────────────

func lockServiceLogon() {
	for svc := range serviceLogonAccounts {
		_, err := runOut("sc", "config", svc,
			"obj=", dummyAccount,
			"password=", dummyPassword)
		if err != nil {
			fmt.Printf("        Warning - could not change logon: %s\n", svc)
		} else {
			fmt.Printf("        Logon locked: %s\n", svc)
		}
	}
}

func unlockServiceLogon() {
	for svc, acct := range serviceLogonAccounts {
		runOut("sc", "config", svc,
			"obj=", acct.obj,
			"password=", acct.password)
		fmt.Printf("        Logon restored: %s → %s\n", svc, acct.obj)
	}
}

// ── Service ACL lock ──────────────────────────────────────────────────────────

func lockServiceACLs() {
	for svc, sd := range serviceLockedSD {
		_, err := runOut("sc", "sdset", svc, sd)
		if err != nil {
			fmt.Printf("        Warning - sdset failed: %s\n", svc)
		} else {
			fmt.Printf("        ACL locked: %s\n", svc)
		}
	}
}

func unlockServiceACLs() {
	for svc, sd := range serviceDefaultSD {
		runOut("sc", "sdset", svc, sd)
		fmt.Printf("        ACL restored: %s\n", svc)
	}
}

// ── Registry ACL lock ─────────────────────────────────────────────────────────

func lockRegistryACLs() {
	services := []string{"wuauserv", "UsoSvc", "WaaSMedicSvc", "BITS", "dosvc"}
	for _, svc := range services {
		ps := fmt.Sprintf(`
$path = 'SYSTEM\CurrentControlSet\Services\%s'
$key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
    $path,
    [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
    [System.Security.AccessControl.RegistryRights]::ChangePermissions
)
$acl = $key.GetAccessControl()
$deny = New-Object System.Security.AccessControl.RegistryAccessRule(
    'NT AUTHORITY\SYSTEM',
    'SetValue,CreateSubKey,Delete',
    'ContainerInherit,ObjectInherit',
    'None',
    'Deny'
)
$acl.AddAccessRule($deny)
$key.SetAccessControl($acl)
$key.Close()
`, svc)
		run("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
		fmt.Printf("        Registry ACL locked: %s\n", svc)
	}
}

func unlockRegistryACLs() {
	services := []string{"wuauserv", "UsoSvc", "WaaSMedicSvc", "BITS", "dosvc"}
	for _, svc := range services {
		ps := fmt.Sprintf(`
$path = 'SYSTEM\CurrentControlSet\Services\%s'
$key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
    $path,
    [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
    [System.Security.AccessControl.RegistryRights]::ChangePermissions
)
$acl = $key.GetAccessControl()
$rules = $acl.Access | Where-Object {
    $_.IdentityReference -eq 'NT AUTHORITY\SYSTEM' -and
    $_.AccessControlType -eq 'Deny'
}
foreach ($rule in $rules) { $acl.RemoveAccessRule($rule) | Out-Null }
$key.SetAccessControl($acl)
$key.Close()
`, svc)
		run("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
		fmt.Printf("        Registry ACL restored: %s\n", svc)
	}
}

// ── WaaSMedicSvc registry key permission lock ─────────────────────────────────
// Standard sc/reg commands return "Access Denied" for WaaSMedicSvc.
// The only reliable way to prevent it from re-enabling itself is to deny
// SYSTEM full control over its registry key (matching the manual regedit steps).

func lockWaaSMedicRegistryPerms() {
	// Two-phase approach:
	// Phase A — lock the key BEFORE writing Start=4, so WaaSMedic cannot race
	//           and overwrite it. We deny SYSTEM write/delete but keep
	//           Administrators full control so this script can still set Start=4.
	// Phase B — called again after Start=4 is written (see disableUpdate) to
	//           confirm the value is still 4; if not, it rewrites it.
	ps := `
$regPath = 'SYSTEM\CurrentControlSet\Services\WaaSMedicSvc'

# Take ownership — transfer from SYSTEM to Administrators
$key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
    $regPath,
    [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
    [System.Security.AccessControl.RegistryRights]::ChangePermissions -bor
    [System.Security.AccessControl.RegistryRights]::TakeOwnership
)
if (-not $key) { Write-Host "ERROR: could not open key"; exit 1 }
$acl = $key.GetAccessControl()
$adminSid = New-Object System.Security.Principal.SecurityIdentifier(
    [System.Security.Principal.WellKnownSidType]::BuiltinAdministratorsSid, $null)
$acl.SetOwner($adminSid)
$key.SetAccessControl($acl)

# Deny NT AUTHORITY\SYSTEM SetValue + CreateSubKey + Delete on this key and subkeys.
# This stops WaaSMedicSvc from writing Start=3 back after reboot.
# Administrators are NOT denied, so this script can still write Start=4 afterwards.
$systemSid = New-Object System.Security.Principal.SecurityIdentifier(
    [System.Security.Principal.WellKnownSidType]::LocalSystemSid, $null)
$denyRule = New-Object System.Security.AccessControl.RegistryAccessRule(
    $systemSid,
    'SetValue,CreateSubKey,Delete,WriteKey',
    'ContainerInherit,ObjectInherit',
    'None',
    'Deny'
)
$acl2 = $key.GetAccessControl()
$acl2.AddAccessRule($denyRule)
$key.SetAccessControl($acl2)
$key.Close()
Write-Host "WaaSMedicSvc registry key: owner->Administrators, SYSTEM write/delete denied."
`
	out, err := runOut("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		fmt.Printf("        Warning - WaaSMedicSvc registry perm lock failed: %v\n", err)
	} else {
		fmt.Printf("        %s\n", strings.TrimSpace(out))
	}
}

// verifyWaaSMedicStartValue checks that Start=4 is still set after locking,
// and rewrites it if WaaSMedic managed to flip it before the lock landed.
func verifyWaaSMedicStartValue() {
	out, _ := runOut("reg", "query",
		`HKLM\SYSTEM\CurrentControlSet\Services\WaaSMedicSvc`,
		"/v", "Start")
	if strings.Contains(out, "0x4") {
		fmt.Println("        Verified: WaaSMedicSvc Start = 4 (Disabled) ✓")
		return
	}
	// Value was flipped — rewrite it now that the key is locked to SYSTEM
	fmt.Println("        Warning: Start value was changed — rewriting Start=4 ...")
	_, err := runOut("reg", "add",
		`HKLM\SYSTEM\CurrentControlSet\Services\WaaSMedicSvc`,
		"/v", "Start", "/t", "REG_DWORD", "/d", "4", "/f")
	if err != nil {
		fmt.Println("        ERROR: could not rewrite Start value.")
	} else {
		fmt.Println("        Rewritten: WaaSMedicSvc Start = 4 (Disabled) ✓")
	}
}

func unlockWaaSMedicRegistryPerms() {
	ps := `
$regPath = 'SYSTEM\CurrentControlSet\Services\WaaSMedicSvc'

$key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
    $regPath,
    [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
    [System.Security.AccessControl.RegistryRights]::ChangePermissions -bor
    [System.Security.AccessControl.RegistryRights]::TakeOwnership
)
$acl = $key.GetAccessControl()

# Remove all Deny rules so Windows can manage the key again
$denyRules = $acl.Access | Where-Object { $_.AccessControlType -eq 'Deny' }
foreach ($rule in $denyRules) { $acl.RemoveAccessRule($rule) | Out-Null }

# Restore owner to SYSTEM (Windows default for service registry keys)
$systemSid = New-Object System.Security.Principal.SecurityIdentifier(
    [System.Security.Principal.WellKnownSidType]::LocalSystemSid, $null)
$acl.SetOwner($systemSid)
$key.SetAccessControl($acl)
$key.Close()
Write-Host "WaaSMedicSvc registry key: Deny rules removed, owner restored to SYSTEM."
`
	out, err := runOut("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		fmt.Printf("        Warning - WaaSMedicSvc registry perm restore failed: %v\n", err)
	} else {
		fmt.Printf("        %s\n", strings.TrimSpace(out))
	}
}

// ── Settings page visibility ──────────────────────────────────────────────────

func hideSettingsPage() {
	const (
		keyPath   = `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\Explorer`
		valueName = "SettingsPageVisibility"
	)

	existing, _ := runOut("reg", "query", keyPath, "/v", valueName)

	var newValue string
	if strings.Contains(existing, valueName) {
		for _, line := range strings.Split(existing, "\n") {
			if strings.Contains(line, valueName) {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					cur := parts[len(parts)-1]
					if !strings.Contains(cur, "windowsupdate") {
						if strings.HasSuffix(cur, ";") {
							newValue = cur + "windowsupdate"
						} else {
							newValue = cur + ";windowsupdate"
						}
					} else {
						newValue = cur
					}
				}
				break
			}
		}
	}
	if newValue == "" {
		newValue = "hide:windowsupdate"
	}

	run("reg", "add", keyPath, "/v", valueName, "/t", "REG_SZ", "/d", newValue, "/f")
	fmt.Println("        Windows Update hidden from Settings.")
}

func showSettingsPage() {
	const (
		keyPath   = `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\Explorer`
		valueName = "SettingsPageVisibility"
	)

	existing, err := runOut("reg", "query", keyPath, "/v", valueName)
	if err != nil || !strings.Contains(existing, valueName) {
		fmt.Println("        Nothing to restore (value not set).")
		return
	}

	var curValue string
	for _, line := range strings.Split(existing, "\n") {
		if strings.Contains(line, valueName) {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				curValue = parts[len(parts)-1]
			}
			break
		}
	}

	if curValue == "" || curValue == "hide:windowsupdate" {
		run("reg", "delete", keyPath, "/v", valueName, "/f")
		fmt.Println("        SettingsPageVisibility removed (clean restore).")
		return
	}

	newValue := strings.ReplaceAll(curValue, ";windowsupdate", "")
	newValue = strings.ReplaceAll(newValue, "windowsupdate;", "")
	newValue = strings.ReplaceAll(newValue, "windowsupdate", "")

	if newValue == "hide:" || newValue == "" {
		run("reg", "delete", keyPath, "/v", valueName, "/f")
	} else {
		run("reg", "add", keyPath, "/v", valueName, "/t", "REG_SZ", "/d", newValue, "/f")
	}
	fmt.Println("        Windows Update restored in Settings.")
}

// ── Registry ──────────────────────────────────────────────────────────────────

func applyRegistry() {
	regAdd := func(path, name, kind, value string) {
		run("reg", "add", path, "/v", name, "/t", kind, "/d", value, "/f")
	}

	regAdd(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, "NoAutoUpdate", "REG_DWORD", "1")
	regAdd(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, "AUOptions", "REG_DWORD", "1")
	regAdd(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "DisableWindowsUpdateAccess", "REG_DWORD", "1")
	regAdd(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "WUServer", "REG_SZ", "http://localhost:0")
	regAdd(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "WUStatusServer", "REG_SZ", "http://localhost:0")
	regAdd(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "UseWUServer", "REG_DWORD", "1")
	regAdd(`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update`, "AUOptions", "REG_DWORD", "1")

	// Disable services via registry (belt + suspenders with sc config above)
	for _, svc := range []string{"wuauserv", "UsoSvc", "BITS", "dosvc", "WaaSMedicSvc"} {
		regAdd(`HKLM\SYSTEM\CurrentControlSet\Services\`+svc, "Start", "REG_DWORD", "4")
	}

	fmt.Println("        Done.")
}

func removeRegistry() {
	regDel := func(path, name string) {
		run("reg", "delete", path, "/v", name, "/f")
	}
	regAdd := func(path, name, kind, value string) {
		run("reg", "add", path, "/v", name, "/t", kind, "/d", value, "/f")
	}

	regDel(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, "NoAutoUpdate")
	regDel(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, "AUOptions")
	regDel(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "DisableWindowsUpdateAccess")
	regDel(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "WUServer")
	regDel(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "WUStatusServer")
	regDel(`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`, "UseWUServer")

	// Restore service start types
	regAdd(`HKLM\SYSTEM\CurrentControlSet\Services\wuauserv`, "Start", "REG_DWORD", "3")
	regAdd(`HKLM\SYSTEM\CurrentControlSet\Services\UsoSvc`, "Start", "REG_DWORD", "3")
	regAdd(`HKLM\SYSTEM\CurrentControlSet\Services\BITS`, "Start", "REG_DWORD", "2")
	regAdd(`HKLM\SYSTEM\CurrentControlSet\Services\dosvc`, "Start", "REG_DWORD", "2")
	regAdd(`HKLM\SYSTEM\CurrentControlSet\Services\WaaSMedicSvc`, "Start", "REG_DWORD", "3")

	fmt.Println("        Done.")
}

// ── Restart prompt ────────────────────────────────────────────────────────────

func askRestart(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println(" " + sep)
	fmt.Println("  A restart is required to fully apply all changes.")
	fmt.Print("  Restart now? [Y/N]: ")

	input, _ := reader.ReadString('\n')
	if strings.ToUpper(strings.TrimSpace(input)) == "Y" {
		fmt.Println("  Restarting in 5 seconds...")
		time.Sleep(5 * time.Second)
		run("shutdown", "/r", "/t", "0")
		os.Exit(0)
	}
	fmt.Println()
}

func pause(reader *bufio.Reader) {
	fmt.Print("\n  Press Enter to continue...")
	reader.ReadString('\n')
}

// ── Edge Update ───────────────────────────────────────────────────────────────

const (
	edgeUpdateDir     = `%ProgramFiles(x86)%\Microsoft\EdgeUpdate`
	edgeUpdateDisable = `%ProgramFiles(x86)%\Microsoft\EdgeUpdate2`
	edgeUpdateBackup  = `%ProgramFiles(x86)%\Microsoft\EdgeUpdate_disable`
)

func edgeUpdateBasePath() string {
	// On 64-bit Windows, ProgramFiles(x86) is set; on 32-bit it is empty/unset.
	x86 := os.Getenv("ProgramFiles(x86)")
	if x86 != "" {
		return x86 + `\Microsoft\EdgeUpdate`
	}
	return os.Getenv("ProgramFiles") + `\Microsoft\EdgeUpdate`
}

func disableEdgeUpdate() {
	edgeDir := edgeUpdateBasePath()
	edgeDisable := edgeDir + "-disable"

	if _, err := os.Stat(edgeDisable); err == nil {
		fmt.Println("        Edge Update already disabled (EdgeUpdate-disable exists).")
		return
	}

	if _, err := os.Stat(edgeDir); err != nil {
		fmt.Println("        EdgeUpdate folder not found — skipping.")
		return
	}

	// Kill the process and take ownership of the folder.
	run("taskkill", "/F", "/IM", "MicrosoftEdgeUpdate.exe")
	run("takeown", "/f", edgeDir, "/a", "/r", "/d", "y")
	run("icacls", edgeDir, "/grant", "Administrators:F", "/t", "/c")
	run("attrib", "-R", edgeDir)

	// Delay to let the process fully release handles.
	time.Sleep(2 * time.Second)

	// Rename EdgeUpdate folder → EdgeUpdate-disable.
	err := os.Rename(edgeDir, edgeDisable)
	if err != nil {
		fmt.Printf("        Warning: could not rename EdgeUpdate folder: %v\n", err)
		return
	}
	fmt.Println("        EdgeUpdate folder renamed → EdgeUpdate-disable.")

	// Create a decoy file named EdgeUpdate in place of the folder.
	f, err2 := os.Create(edgeDir)
	if err2 != nil {
		fmt.Printf("        Warning: could not create decoy file: %v\n", err2)
	} else {
		f.Close()
		run("attrib", "+R", edgeDir)
		fmt.Println("        Decoy file created: EdgeUpdate (read-only).")
	}

	fmt.Println("        Edge Update disabled.")
}

func enableEdgeUpdate() {
	edgeDir := edgeUpdateBasePath()
	edgeDisable := edgeDir + "-disable"

	if _, err := os.Stat(edgeDisable); err != nil {
		fmt.Println("        EdgeUpdate-disable not found — Edge Update may already be enabled.")
		return
	}

	// Remove the decoy file.
	run("attrib", "-R", edgeDir)
	err := os.Remove(edgeDir)
	if err != nil {
		fmt.Printf("        Warning: could not remove decoy file: %v\n", err)
	} else {
		fmt.Println("        Decoy file removed.")
	}

	// Rename EdgeUpdate-disable → EdgeUpdate.
	err2 := os.Rename(edgeDisable, edgeDir)
	if err2 != nil {
		fmt.Printf("        Warning: could not restore EdgeUpdate folder: %v\n", err2)
		return
	}
	fmt.Println("        EdgeUpdate folder restored.")
}



func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Run()
}

func runOut(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
