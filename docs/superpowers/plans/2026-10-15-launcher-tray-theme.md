# Launcher Tray, Theme, and Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add tray minimize, a single-column launcher window, and a system/light/dark theme button without changing how the server process is started or stopped.

**Architecture:** Pure decisions (preferences, theme choice, the one service button, close policy) live in small C# types and run under `Kairisei-Launcher.exe --self-test`. `LauncherForm` draws the single-column window and calls those types. `ServerController` stays the only caller of `Start-Server.ps1` and `Stop-Server.ps1`.

**Tech Stack:** C# 5, .NET Framework 4, `csc.exe` at `%WINDIR%\Microsoft.NET\Framework64\v4.0.30319\csc.exe`, WinForms, `System.Web.Extensions` `JavaScriptSerializer`. No new assembly references.

## Global Constraints

- Build with the existing `tools/powershell/Build-Launcher.ps1` and that `csc.exe`. Do not use C# 6 or newer syntax: no `$""`, `?.`, `nameof`, `out var`, expression-bodied members, or auto-property initializers.
- Do not add assembly references. `NotifyIcon` is already in `System.Windows.Forms`. Title-bar dark mode is `dwmapi.dll` `DwmSetWindowAttribute`.
- Server start and stop stay in `ServerController`. Do not edit `server/`.
- `--smoke-test` keeps its current checks and exit codes: 0 when the packaged files exist, 2 when they do not.
- The title-bar minimize button still minimizes to the taskbar and does not create a tray icon.
- The public-address confirmation before start stays.
- Do not restyle the admin web UI. Do not show a balloon notification.
- Preference file is `%AppData%\Kairisei\launcher-preferences.json` with string fields `close_action` (`ask`, `tray`, `exit`) and `theme` (`system`, `light`, `dark`). Missing file, unreadable file, or unknown values fall back per field to `ask` and `system` and do not crash.
- There is no control on the main window to edit `close_action`.
- Theme click order is 跟随系统 → 浅色 → 深色 → 跟随系统. Hover text is exactly `跟随系统`, `浅色`, or `深色`.
- Registry value `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize\AppsUseLightTheme`: missing or `1` is light, `0` is dark. Fixed light or dark does not follow later system changes. While following the system, re-read on the existing about 1.5 second status timer.
- Service button rows, first match wins, in this order (this resolves the spec table so "clicked start" is not hidden by "no process"):
  1. The first status read has not finished: `确认服务中`, disabled.
  2. A stop is in progress and the server process still exists: `停止服务`, disabled.
  3. A start is in progress, or a process exists and health has not passed: `确认服务中`, disabled.
  4. Health has passed and no stop is in progress: `停止服务`, enabled.
  5. Otherwise: `启动服务`, enabled.
- Do not offer Stop before health has passed. When the process disappears, the button returns to `启动服务`.
- `启动服务` uses `#1494D3`. `停止服务` when enabled uses `#DE4E4E`. Disabled `确认服务中` and disabled `停止服务` use gray `#C5D0D8` with white text.
- User close with `ask`: if the window is minimized, restore it, then show `关闭启动器`. Default radio is `最小化到托盘`. The other radio is `直接关闭`. Checkbox `记住选项，下次不再提示` starts unchecked. Cancel writes nothing.
- User close with `tray` hides to the tray with no dialog. User close with `exit` uses the exit rule with no dialog.
- Tray icon exists only while the window is hidden to the tray. Menu items are `显示主页面` and `退出`. Double-click equals `显示主页面`. Showing the window removes the tray icon.
- Exit rule: no server process exits the launcher; a running server calls the existing stop and waits for that stop (about 25 seconds) before exiting; stop failure or an operation already in progress reopens the window and shows the existing failure text.
- Windows shutdown and logoff do not show the dialog and do not hide to the tray. If the server is running, stop and wait, then exit even when that stop fails. Task Manager end-task does not show the dialog and does not wait for stop.
- A second launch exits immediately and brings the existing window forward, restoring it from the tray when needed.
- Light palette: canvas `#F4F9FC`, surface `#FFFFFF`, text `#17374E`, muted `#4D7086`, accent `#0B679F`, start `#1494D3`, danger `#DE4E4E`, success `#2A9D64`, pale button `#E6F7FF`.
- Dark palette: canvas `#1C2430`, surface `#2B3544`, text `#E6EEF4`, muted `#9AABBA`, accent `#8EC8E6`, start `#1494D3`, danger `#DE4E4E`, success `#3CBF7A`, secondary button `#243244`.
- Sun icon: black edge, white interior. Moon circle equals the sun circle and sits to its right, overlapping it. The crescent is the left part of the sun disk not covered by the moon circle, filled black. Follow-system draws the full sun circle with that black crescent on the left and white on the right. Light draws only the sun. Dark draws only the black crescent, with no outer full-circle stroke. A dark title bar adds a white outer stroke around black edges and does not change those fills. A light title bar has no outer white stroke.
- Do not display the words `连接`, `战斗端口 = 主端口 + 1`, or `关闭窗口时`.
- Question-mark hover text, exactly:
  - Android 模拟器: `适用于本机 Android 模拟器。客户端默认地址就是 10.0.2.2，无需改 APK。`
  - 局域网: `填写客户端可访问的 IPv4 或域名，支持虚拟局域网及内网穿透。需放行或映射 TCP 主端口和战斗端口。`
  - 公网 IP / 域名: `填写这台服务器的公网 IPv4 或域名。需映射 TCP 主端口与战斗端口；Admin 不会公网开放。`
- Status text: no process `未运行`; process present but not healthy, or the first check still running, `检测中`; healthy `正在运行`.
- Default client size is 460 by 640. The log grows with the window. DPI scaling stays on.

## File structure

- Create `cmd/kairi-launcher-winforms/LauncherPreferences.cs` — load and save the JSON file.
- Create `cmd/kairi-launcher-winforms/LauncherSelfTest.cs` — assertions for `--self-test`.
- Create `cmd/kairi-launcher-winforms/ThemeChoice.cs` — theme enum, next choice, registry interpretation.
- Create `cmd/kairi-launcher-winforms/ServiceButton.cs` — the one-button state table.
- Create `cmd/kairi-launcher-winforms/ClosePolicy.cs` — user close, shutdown, and Task Manager outcomes.
- Create `cmd/kairi-launcher-winforms/ThemePalette.cs` — palettes, dark title bar, icon drawing.
- Create `cmd/kairi-launcher-winforms/ClosePromptForm.cs` — the modal dialog.
- Create `cmd/kairi-launcher-winforms/TrayIcon.cs` — notify icon shown only while hidden.
- Create `cmd/kairi-launcher-winforms/SingleInstance.cs` — mutex and show event.
- Create `cmd/kairi-launcher-winforms/LauncherForm.cs` — the window. Move the UI out of `Program.cs`.
- Modify `cmd/kairi-launcher-winforms/Program.cs` — keep `--smoke-test`, `ServerState`, and `DeploymentDefaults`; dispatch `--self-test` and the single instance.
- Modify `tools/powershell/Build-Launcher.ps1` — compile the new files and run `--self-test`.
- Do not modify `cmd/kairi-launcher-winforms/ServerController.cs`.

`ServerState` stays in `Program.cs` so `ServerController.cs` keeps compiling against it.

---

### Task 1: Preference file and self-test host

**Files:**
- Create: `cmd/kairi-launcher-winforms/LauncherPreferences.cs`
- Create: `cmd/kairi-launcher-winforms/LauncherSelfTest.cs`
- Modify: `cmd/kairi-launcher-winforms/Program.cs` (the `Main` method)
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `enum CloseAction { Ask, Tray, Exit }`
  - `enum ThemeChoice { System, Light, Dark }`
  - `sealed class LauncherPreferences` with public fields `CloseAction CloseAction` and `ThemeChoice Theme`
  - `static class LauncherPreferencesStore` with `LauncherPreferences Load(string path)` and `bool TrySave(string path, LauncherPreferences preferences)`
  - `static class LauncherSelfTest` with `int Run()`
  - `LauncherPreferencesStore.DefaultPath` returns `%AppData%\Kairisei\launcher-preferences.json`

- [ ] **Step 1: Write the failing test**

Create `cmd/kairi-launcher-winforms/LauncherSelfTest.cs`:

```csharp
using System;
using System.Collections.Generic;
using System.IO;

namespace KairiseiLauncher
{
    internal static class LauncherSelfTest
    {
        public static int Run()
        {
            var failures = new List<string>();
            PreferenceTests(failures);
            if (failures.Count == 0) return 0;
            try
            {
                File.WriteAllLines(Path.Combine(Path.GetTempPath(), "kairisei-launcher-self-test.txt"), failures.ToArray());
            }
            catch { }
            return 1;
        }

        private static void PreferenceTests(List<string> failures)
        {
            var missing = LauncherPreferencesStore.Load(Path.Combine(Path.GetTempPath(), "missing-kairisei-prefs.json"));
            Expect(failures, missing.CloseAction == CloseAction.Ask && missing.Theme == ThemeChoice.System, "missing file uses defaults");

            var directory = Path.Combine(Path.GetTempPath(), "kairisei-prefs-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(directory);
            try
            {
                var path = Path.Combine(directory, "launcher-preferences.json");
                File.WriteAllText(path, "{", System.Text.Encoding.UTF8);
                var broken = LauncherPreferencesStore.Load(path);
                Expect(failures, broken.CloseAction == CloseAction.Ask && broken.Theme == ThemeChoice.System, "broken json uses defaults");

                File.WriteAllText(path, "{\"close_action\":\"nope\",\"theme\":\"light\"}", System.Text.Encoding.UTF8);
                var partial = LauncherPreferencesStore.Load(path);
                Expect(failures, partial.CloseAction == CloseAction.Ask && partial.Theme == ThemeChoice.Light, "unknown close_action falls back");

                var saved = new LauncherPreferences();
                saved.CloseAction = CloseAction.Tray;
                saved.Theme = ThemeChoice.Dark;
                Expect(failures, LauncherPreferencesStore.TrySave(path, saved), "save returns true");
                var round = LauncherPreferencesStore.Load(path);
                Expect(failures, round.CloseAction == CloseAction.Tray && round.Theme == ThemeChoice.Dark, "round trip");

                var blocked = Path.Combine(directory, "not-a-directory");
                File.WriteAllText(blocked, "x", System.Text.Encoding.UTF8);
                var unsaved = new LauncherPreferences();
                unsaved.CloseAction = CloseAction.Exit;
                unsaved.Theme = ThemeChoice.Light;
                Expect(failures, !LauncherPreferencesStore.TrySave(Path.Combine(blocked, "launcher-preferences.json"), unsaved), "unwritable save returns false");
            }
            finally
            {
                try { Directory.Delete(directory, true); } catch { }
            }
        }

        private static void Expect(List<string> failures, bool condition, string message)
        {
            if (!condition) failures.Add(message);
        }
    }
}
```

In `Program.Main`, before `Application.EnableVisualStyles()`, add:

```csharp
if (args.Any(value => String.Equals(value, "--self-test", StringComparison.OrdinalIgnoreCase)))
    return LauncherSelfTest.Run();
```

In `Build-Launcher.ps1`, add `(Join-Path $source 'LauncherPreferences.cs')` and `(Join-Path $source 'LauncherSelfTest.cs')` to the `$arguments` array after `ServerController.cs`. After a successful `csc` invocation, run:

```powershell
& $output --self-test
if ($LASTEXITCODE -ne 0) { throw 'Launcher self-test failed.' }
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: `csc` fails because `LauncherPreferencesStore` does not exist, and the script throws `Launcher build failed.`

- [ ] **Step 3: Write the minimal implementation**

Create `cmd/kairi-launcher-winforms/LauncherPreferences.cs`:

```csharp
using System;
using System.IO;
using System.Text;
using System.Web.Script.Serialization;

namespace KairiseiLauncher
{
    internal enum CloseAction
    {
        Ask,
        Tray,
        Exit
    }

    internal enum ThemeChoice
    {
        System,
        Light,
        Dark
    }

    internal sealed class LauncherPreferences
    {
        public CloseAction CloseAction;
        public ThemeChoice Theme;
    }

    internal sealed class PreferenceDocument
    {
        public string close_action { get; set; }
        public string theme { get; set; }
    }

    internal static class LauncherPreferencesStore
    {
        private static readonly JavaScriptSerializer Serializer = new JavaScriptSerializer();

        public static string DefaultPath
        {
            get
            {
                return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Kairisei", "launcher-preferences.json");
            }
        }

        public static LauncherPreferences Load(string path)
        {
            var preferences = new LauncherPreferences();
            preferences.CloseAction = CloseAction.Ask;
            preferences.Theme = ThemeChoice.System;
            if (String.IsNullOrEmpty(path) || !File.Exists(path)) return preferences;
            try
            {
                var document = Serializer.Deserialize<PreferenceDocument>(File.ReadAllText(path, Encoding.UTF8));
                if (document == null) return preferences;
                preferences.CloseAction = ParseClose(document.close_action);
                preferences.Theme = ParseTheme(document.theme);
            }
            catch { }
            return preferences;
        }

        public static bool TrySave(string path, LauncherPreferences preferences)
        {
            try
            {
                var directory = Path.GetDirectoryName(path);
                if (!String.IsNullOrEmpty(directory)) Directory.CreateDirectory(directory);
                var document = new PreferenceDocument();
                document.close_action = FormatClose(preferences.CloseAction);
                document.theme = FormatTheme(preferences.Theme);
                File.WriteAllText(path, Serializer.Serialize(document), new UTF8Encoding(false));
                return true;
            }
            catch { return false; }
        }

        private static CloseAction ParseClose(string value)
        {
            if (String.Equals(value, "tray", StringComparison.OrdinalIgnoreCase)) return CloseAction.Tray;
            if (String.Equals(value, "exit", StringComparison.OrdinalIgnoreCase)) return CloseAction.Exit;
            return CloseAction.Ask;
        }

        private static ThemeChoice ParseTheme(string value)
        {
            if (String.Equals(value, "light", StringComparison.OrdinalIgnoreCase)) return ThemeChoice.Light;
            if (String.Equals(value, "dark", StringComparison.OrdinalIgnoreCase)) return ThemeChoice.Dark;
            return ThemeChoice.System;
        }

        private static string FormatClose(CloseAction value)
        {
            if (value == CloseAction.Tray) return "tray";
            if (value == CloseAction.Exit) return "exit";
            return "ask";
        }

        private static string FormatTheme(ThemeChoice value)
        {
            if (value == ThemeChoice.Light) return "light";
            if (value == ThemeChoice.Dark) return "dark";
            return "system";
        }
    }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: the script prints the exe path and exits 0. `%TEMP%\kairisei-launcher-self-test.txt` is not created.

- [ ] **Step 5: Commit**

```bash
git add cmd/kairi-launcher-winforms/LauncherPreferences.cs cmd/kairi-launcher-winforms/LauncherSelfTest.cs cmd/kairi-launcher-winforms/Program.cs tools/powershell/Build-Launcher.ps1
git commit -m "Add launcher preference load and save"
```

---

### Task 2: Theme choice cycle and registry value

**Files:**
- Create: `cmd/kairi-launcher-winforms/ThemeChoice.cs`
- Modify: `cmd/kairi-launcher-winforms/LauncherSelfTest.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: `ThemeChoice` from Task 1
- Produces:
  - `enum ResolvedTheme { Light, Dark }`
  - `static class ThemeRules` with `ThemeChoice Next(ThemeChoice current)`, `string Caption(ThemeChoice current)`, `ResolvedTheme Resolve(ThemeChoice current, int? appsUseLightTheme)`

- [ ] **Step 1: Write the failing test**

Add `ThemeTests(failures);` immediately after `PreferenceTests(failures);` inside `LauncherSelfTest.Run`. Add this method to `LauncherSelfTest`:

```csharp
private static void ThemeTests(List<string> failures)
{
    Expect(failures, ThemeRules.Next(ThemeChoice.System) == ThemeChoice.Light, "system goes to light");
    Expect(failures, ThemeRules.Next(ThemeChoice.Light) == ThemeChoice.Dark, "light goes to dark");
    Expect(failures, ThemeRules.Next(ThemeChoice.Dark) == ThemeChoice.System, "dark goes to system");
    Expect(failures, ThemeRules.Caption(ThemeChoice.System) == "跟随系统", "system caption");
    Expect(failures, ThemeRules.Caption(ThemeChoice.Light) == "浅色", "light caption");
    Expect(failures, ThemeRules.Caption(ThemeChoice.Dark) == "深色", "dark caption");
    Expect(failures, ThemeRules.Resolve(ThemeChoice.System, null) == ResolvedTheme.Light, "missing registry is light");
    Expect(failures, ThemeRules.Resolve(ThemeChoice.System, 1) == ResolvedTheme.Light, "registry 1 is light");
    Expect(failures, ThemeRules.Resolve(ThemeChoice.System, 0) == ResolvedTheme.Dark, "registry 0 is dark");
    Expect(failures, ThemeRules.Resolve(ThemeChoice.Light, 0) == ResolvedTheme.Light, "fixed light ignores registry");
    Expect(failures, ThemeRules.Resolve(ThemeChoice.Dark, 1) == ResolvedTheme.Dark, "fixed dark ignores registry");
}
```

Add `(Join-Path $source 'ThemeChoice.cs')` to the `csc` file list in `Build-Launcher.ps1`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: compile fails because `ThemeRules` does not exist.

- [ ] **Step 3: Write the minimal implementation**

Create `cmd/kairi-launcher-winforms/ThemeChoice.cs`:

```csharp
namespace KairiseiLauncher
{
    internal enum ResolvedTheme
    {
        Light,
        Dark
    }

    internal static class ThemeRules
    {
        public static ThemeChoice Next(ThemeChoice current)
        {
            if (current == ThemeChoice.System) return ThemeChoice.Light;
            if (current == ThemeChoice.Light) return ThemeChoice.Dark;
            return ThemeChoice.System;
        }

        public static string Caption(ThemeChoice current)
        {
            if (current == ThemeChoice.Light) return "浅色";
            if (current == ThemeChoice.Dark) return "深色";
            return "跟随系统";
        }

        public static ResolvedTheme Resolve(ThemeChoice current, int? appsUseLightTheme)
        {
            if (current == ThemeChoice.Light) return ResolvedTheme.Light;
            if (current == ThemeChoice.Dark) return ResolvedTheme.Dark;
            if (appsUseLightTheme.HasValue && appsUseLightTheme.Value == 0) return ResolvedTheme.Dark;
            return ResolvedTheme.Light;
        }
    }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/kairi-launcher-winforms/ThemeChoice.cs cmd/kairi-launcher-winforms/LauncherSelfTest.cs tools/powershell/Build-Launcher.ps1
git commit -m "Add launcher theme choice rules"
```

---

### Task 3: Service button state

**Files:**
- Create: `cmd/kairi-launcher-winforms/ServiceButton.cs`
- Modify: `cmd/kairi-launcher-winforms/LauncherSelfTest.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: nothing from Task 2
- Produces:
  - `enum ServiceButtonKind { Confirming, Start, Stop }`
  - `struct ServiceButtonState` with fields `ServiceButtonKind Kind`, `string Text`, `bool Enabled`
  - `static class ServiceButton` with `ServiceButtonState Resolve(bool initialCheckComplete, bool processExists, bool healthy, bool startInProgress, bool stopInProgress)`

- [ ] **Step 1: Write the failing test**

Add `ServiceButtonTests(failures);` after `ThemeTests(failures);` in `LauncherSelfTest.Run`. Add:

```csharp
private static void ServiceButtonTests(List<string> failures)
{
    ExpectButton(failures, ServiceButton.Resolve(false, false, false, false, false), "确认服务中", false, "first check");
    ExpectButton(failures, ServiceButton.Resolve(true, false, false, false, false), "启动服务", true, "idle");
    ExpectButton(failures, ServiceButton.Resolve(true, false, false, true, false), "确认服务中", false, "start before process");
    ExpectButton(failures, ServiceButton.Resolve(true, true, false, false, false), "确认服务中", false, "process not healthy");
    ExpectButton(failures, ServiceButton.Resolve(true, true, true, false, false), "停止服务", true, "healthy");
    ExpectButton(failures, ServiceButton.Resolve(true, true, true, false, true), "停止服务", false, "stopping while process exists");
    ExpectButton(failures, ServiceButton.Resolve(true, false, false, false, true), "启动服务", true, "process already gone");
}

private static void ExpectButton(List<string> failures, ServiceButtonState state, string text, bool enabled, string message)
{
    Expect(failures, state.Text == text && state.Enabled == enabled, message);
}
```

Add `(Join-Path $source 'ServiceButton.cs')` to the `csc` file list.

- [ ] **Step 2: Run the test to verify it fails**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: compile fails because `ServiceButton` does not exist.

- [ ] **Step 3: Write the minimal implementation**

Create `cmd/kairi-launcher-winforms/ServiceButton.cs`:

```csharp
namespace KairiseiLauncher
{
    internal enum ServiceButtonKind
    {
        Confirming,
        Start,
        Stop
    }

    internal struct ServiceButtonState
    {
        public ServiceButtonKind Kind;
        public string Text;
        public bool Enabled;
    }

    internal static class ServiceButton
    {
        public static ServiceButtonState Resolve(bool initialCheckComplete, bool processExists, bool healthy, bool startInProgress, bool stopInProgress)
        {
            if (!initialCheckComplete) return Make(ServiceButtonKind.Confirming, "确认服务中", false);
            if (stopInProgress && processExists) return Make(ServiceButtonKind.Stop, "停止服务", false);
            if (startInProgress || (processExists && !healthy)) return Make(ServiceButtonKind.Confirming, "确认服务中", false);
            if (healthy && processExists) return Make(ServiceButtonKind.Stop, "停止服务", true);
            return Make(ServiceButtonKind.Start, "启动服务", true);
        }

        private static ServiceButtonState Make(ServiceButtonKind kind, string text, bool enabled)
        {
            var state = new ServiceButtonState();
            state.Kind = kind;
            state.Text = text;
            state.Enabled = enabled;
            return state;
        }
    }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/kairi-launcher-winforms/ServiceButton.cs cmd/kairi-launcher-winforms/LauncherSelfTest.cs tools/powershell/Build-Launcher.ps1
git commit -m "Add launcher service button states"
```

---

### Task 4: Close policy

**Files:**
- Create: `cmd/kairi-launcher-winforms/ClosePolicy.cs`
- Modify: `cmd/kairi-launcher-winforms/LauncherSelfTest.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: `CloseAction` from Task 1
- Produces:
  - `enum CloseOutcome { ShowPrompt, HideToTray, ExitLauncher, ExitAfterStop, StayBecauseBusy }`
  - `static class ClosePolicy` with:
    - `CloseOutcome ForUserClose(CloseAction action, bool serverRunning, bool operationBusy)`
    - `CloseOutcome ForShutdown(bool serverRunning)`
    - `CloseOutcome ForTaskManager()`
    - `CloseAction Remembered(bool minimizeToTray, bool remember)`

- [ ] **Step 1: Write the failing test**

Add `CloseTests(failures);` after `ServiceButtonTests(failures);`. Add:

```csharp
private static void CloseTests(List<string> failures)
{
    Expect(failures, ClosePolicy.ForUserClose(CloseAction.Ask, true, false) == CloseOutcome.ShowPrompt, "ask shows prompt");
    Expect(failures, ClosePolicy.ForUserClose(CloseAction.Tray, true, true) == CloseOutcome.HideToTray, "tray hides even while busy");
    Expect(failures, ClosePolicy.ForUserClose(CloseAction.Exit, false, false) == CloseOutcome.ExitLauncher, "exit with no server");
    Expect(failures, ClosePolicy.ForUserClose(CloseAction.Exit, true, false) == CloseOutcome.ExitAfterStop, "exit stops a running server");
    Expect(failures, ClosePolicy.ForUserClose(CloseAction.Exit, true, true) == CloseOutcome.StayBecauseBusy, "busy exit stays");
    Expect(failures, ClosePolicy.ForShutdown(false) == CloseOutcome.ExitLauncher, "shutdown without server");
    Expect(failures, ClosePolicy.ForShutdown(true) == CloseOutcome.ExitAfterStop, "shutdown stops server");
    Expect(failures, ClosePolicy.ForTaskManager() == CloseOutcome.ExitLauncher, "task manager exits");
    Expect(failures, ClosePolicy.Remembered(true, false) == CloseAction.Ask, "unchecked remember stays ask");
    Expect(failures, ClosePolicy.Remembered(true, true) == CloseAction.Tray, "remember tray");
    Expect(failures, ClosePolicy.Remembered(false, true) == CloseAction.Exit, "remember exit");
}
```

Add `(Join-Path $source 'ClosePolicy.cs')` to the `csc` file list.

- [ ] **Step 2: Run the test to verify it fails**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: compile fails because `ClosePolicy` does not exist.

- [ ] **Step 3: Write the minimal implementation**

Create `cmd/kairi-launcher-winforms/ClosePolicy.cs`:

```csharp
namespace KairiseiLauncher
{
    internal enum CloseOutcome
    {
        ShowPrompt,
        HideToTray,
        ExitLauncher,
        ExitAfterStop,
        StayBecauseBusy
    }

    internal static class ClosePolicy
    {
        public static CloseOutcome ForUserClose(CloseAction action, bool serverRunning, bool operationBusy)
        {
            if (action == CloseAction.Ask) return CloseOutcome.ShowPrompt;
            if (action == CloseAction.Tray) return CloseOutcome.HideToTray;
            if (operationBusy) return CloseOutcome.StayBecauseBusy;
            if (serverRunning) return CloseOutcome.ExitAfterStop;
            return CloseOutcome.ExitLauncher;
        }

        public static CloseOutcome ForShutdown(bool serverRunning)
        {
            if (serverRunning) return CloseOutcome.ExitAfterStop;
            return CloseOutcome.ExitLauncher;
        }

        public static CloseOutcome ForTaskManager()
        {
            return CloseOutcome.ExitLauncher;
        }

        public static CloseAction Remembered(bool minimizeToTray, bool remember)
        {
            if (!remember) return CloseAction.Ask;
            if (minimizeToTray) return CloseAction.Tray;
            return CloseAction.Exit;
        }
    }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/kairi-launcher-winforms/ClosePolicy.cs cmd/kairi-launcher-winforms/LauncherSelfTest.cs tools/powershell/Build-Launcher.ps1
git commit -m "Add launcher close policy"
```

---

### Task 5: Palette and theme icon pixels

**Files:**
- Create: `cmd/kairi-launcher-winforms/ThemePalette.cs`
- Modify: `cmd/kairi-launcher-winforms/LauncherSelfTest.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: `ThemeChoice`, `ResolvedTheme`
- Produces:
  - `sealed class ThemePalette` with public `Color` fields `Canvas`, `Surface`, `Text`, `Muted`, `Accent`, `Start`, `Danger`, `Success`, `PaleButton`, `Disabled`
  - `static ThemePalette For(ResolvedTheme theme)`
  - `static class ThemeIcon` with `void Draw(Graphics graphics, Rectangle bounds, ThemeChoice glyph, bool titleBarIsDark)`
  - `static class TitleBarTheme` with `void Apply(IntPtr window, ResolvedTheme theme)` — failures are ignored

- [ ] **Step 1: Write the failing test**

Add `IconTests(failures);` after `CloseTests(failures);`. Add `using System.Drawing;` to `LauncherSelfTest.cs`. Add:

```csharp
private static void IconTests(List<string> failures)
{
    var light = ThemePalette.For(ResolvedTheme.Light);
    var dark = ThemePalette.For(ResolvedTheme.Dark);
    Expect(failures, light.Canvas.ToArgb() == unchecked((int)0xFFF4F9FC), "light canvas");
    Expect(failures, dark.Canvas.ToArgb() == unchecked((int)0xFF1C2430), "dark canvas");
    Expect(failures, light.Disabled.ToArgb() == unchecked((int)0xFFC5D0D8), "disabled gray");

    using (var bitmap = new Bitmap(64, 64))
    using (var graphics = Graphics.FromImage(bitmap))
    {
        graphics.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.None;
        graphics.Clear(Color.Magenta);
        ThemeIcon.Draw(graphics, new Rectangle(16, 16, 32, 32), ThemeChoice.Light, false);
        Expect(failures, Near(bitmap.GetPixel(32, 32), Color.White), "sun center is white");
        Expect(failures, bitmap.GetPixel(0, 0).ToArgb() == Color.Magenta.ToArgb(), "light title bar has no white halo");

        graphics.Clear(Color.FromArgb(43, 53, 68));
        ThemeIcon.Draw(graphics, new Rectangle(16, 16, 32, 32), ThemeChoice.System, false);
        Expect(failures, Near(bitmap.GetPixel(24, 32), Color.Black), "follow crescent is black");
        Expect(failures, Near(bitmap.GetPixel(40, 32), Color.White), "follow right side is white");

        graphics.Clear(Color.FromArgb(43, 53, 68));
        ThemeIcon.Draw(graphics, new Rectangle(16, 16, 32, 32), ThemeChoice.Dark, true);
        Expect(failures, Near(bitmap.GetPixel(18, 32), Color.White) || Near(bitmap.GetPixel(17, 32), Color.White), "dark title bar halo is white");
    }
}

private static bool Near(Color actual, Color expected)
{
    return System.Math.Abs(actual.R - expected.R) < 12
        && System.Math.Abs(actual.G - expected.G) < 12
        && System.Math.Abs(actual.B - expected.B) < 12;
}
```

Add `(Join-Path $source 'ThemePalette.cs')` to the `csc` file list.

- [ ] **Step 2: Run the test to verify it fails**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: compile fails because `ThemePalette` does not exist.

- [ ] **Step 3: Write the minimal implementation**

Create `cmd/kairi-launcher-winforms/ThemePalette.cs`. `For` assigns the Global Constraints colors. `Disabled` is `#C5D0D8` for both themes. `Draw` maps `bounds` onto a 64 by 64 space. The sun circle is center `(32,32)` radius `16`. The moon circle is center `(44,32)` radius `16`. Build the crescent with a `Region`: the sun ellipse minus the moon ellipse. Fill the sun white and stroke it black with width `2.5` in 64-space, scaled by `bounds.Width / 64f`. For `ThemeChoice.Light`, draw only that sun. For `ThemeChoice.Dark`, fill only the crescent black and do not stroke the full circle. For `ThemeChoice.System`, fill the sun white, fill the crescent black, then stroke the sun black. When `titleBarIsDark` is true, stroke the same path in white with a pen about `2` units wider before the black stroke, so the white ring sits outside the black edge. Set `SmoothingMode.AntiAlias` only when the caller has not already set `SmoothingMode.None`.

`TitleBarTheme.Apply` P/Invokes `DwmSetWindowAttribute`. Try attribute `20` first and then `19`. The value is `1` for dark and `0` for light. Catch every failure and return.

```csharp
using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;

namespace KairiseiLauncher
{
    internal sealed class ThemePalette
    {
        public Color Canvas;
        public Color Surface;
        public Color Text;
        public Color Muted;
        public Color Accent;
        public Color Start;
        public Color Danger;
        public Color Success;
        public Color PaleButton;
        public Color Disabled;

        public static ThemePalette For(ResolvedTheme theme)
        {
            var palette = new ThemePalette();
            palette.Start = Color.FromArgb(20, 148, 211);
            palette.Danger = Color.FromArgb(222, 78, 78);
            palette.Disabled = Color.FromArgb(197, 208, 216);
            if (theme == ResolvedTheme.Dark)
            {
                palette.Canvas = Color.FromArgb(28, 36, 48);
                palette.Surface = Color.FromArgb(43, 53, 68);
                palette.Text = Color.FromArgb(230, 238, 244);
                palette.Muted = Color.FromArgb(154, 171, 186);
                palette.Accent = Color.FromArgb(142, 200, 230);
                palette.Success = Color.FromArgb(60, 191, 122);
                palette.PaleButton = Color.FromArgb(36, 50, 68);
                return palette;
            }
            palette.Canvas = Color.FromArgb(244, 249, 252);
            palette.Surface = Color.White;
            palette.Text = Color.FromArgb(23, 55, 78);
            palette.Muted = Color.FromArgb(77, 112, 134);
            palette.Accent = Color.FromArgb(11, 103, 159);
            palette.Success = Color.FromArgb(42, 157, 100);
            palette.PaleButton = Color.FromArgb(230, 247, 255);
            return palette;
        }
    }

    internal static class ThemeIcon
    {
        public static void Draw(Graphics graphics, Rectangle bounds, ThemeChoice glyph, bool titleBarIsDark)
        {
            if (bounds.Width <= 0 || bounds.Height <= 0) return;
            var previous = graphics.Save();
            graphics.TranslateTransform(bounds.X, bounds.Y);
            graphics.ScaleTransform(bounds.Width / 64f, bounds.Height / 64f);
            using (var sun = new GraphicsPath())
            using (var moon = new GraphicsPath())
            {
                sun.AddEllipse(16, 16, 32, 32);
                moon.AddEllipse(28, 16, 32, 32);
                using (var crescent = new Region(sun))
                {
                    crescent.Exclude(moon);
                    if (glyph != ThemeChoice.Dark)
                    {
                        using (var fill = new SolidBrush(Color.White)) graphics.FillPath(fill, sun);
                    }
                    if (glyph != ThemeChoice.Light)
                    {
                        using (var fill = new SolidBrush(Color.Black)) graphics.FillRegion(fill, crescent);
                    }
                    if (glyph != ThemeChoice.Dark)
                    {
                        if (titleBarIsDark)
                        {
                            using (var halo = new Pen(Color.White, 5f)) graphics.DrawPath(halo, sun);
                        }
                        using (var edge = new Pen(Color.Black, 2.5f)) graphics.DrawPath(edge, sun);
                    }
                    else if (titleBarIsDark)
                    {
                        using (var halo = new Pen(Color.White, 5f)) graphics.DrawPath(halo, CrescentPath(sun, moon));
                    }
                }
            }
            graphics.Restore(previous);
        }

        private static GraphicsPath CrescentPath(GraphicsPath sun, GraphicsPath moon)
        {
            var path = new GraphicsPath();
            path.AddPath(sun, false);
            path.AddPath(moon, false);
            path.SetMarkers();
            return path;
        }
    }

    internal static class TitleBarTheme
    {
        public static void Apply(IntPtr window, ResolvedTheme theme)
        {
            if (window == IntPtr.Zero) return;
            int useDark = theme == ResolvedTheme.Dark ? 1 : 0;
            try
            {
                if (DwmSetWindowAttribute(window, 20, ref useDark, 4) != 0)
                    DwmSetWindowAttribute(window, 19, ref useDark, 4);
            }
            catch { }
        }

        [DllImport("dwmapi.dll")]
        private static extern int DwmSetWindowAttribute(IntPtr window, int attribute, ref int value, int size);
    }
}
```

The dark-glyph halo above uses a combined path that may not outline only the crescent. If Step 4's halo pixel misses, replace the dark-glyph halo with a white fill of a slightly larger crescent: sun ellipse expanded by 3 units in 64-space, minus the same moon ellipse, drawn before the black crescent. Keep the black crescent geometry unchanged.

- [ ] **Step 4: Run the test to verify it passes**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0. If only the halo pixel fails, adjust the white stroke width as the last paragraph of Step 3 says, then run this command again until it exits 0. Do not change the expected colors of the sun center, the black crescent, or the white right side.

- [ ] **Step 5: Commit**

```bash
git add cmd/kairi-launcher-winforms/ThemePalette.cs cmd/kairi-launcher-winforms/LauncherSelfTest.cs tools/powershell/Build-Launcher.ps1
git commit -m "Draw launcher sun and moon theme icons"
```

---

### Task 6: Close dialog and tray icon

**Files:**
- Create: `cmd/kairi-launcher-winforms/ClosePromptForm.cs`
- Create: `cmd/kairi-launcher-winforms/TrayIcon.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: `CloseAction`, `ClosePolicy.Remembered`, `ThemePalette`
- Produces:
  - `sealed class ClosePromptForm : Form` with `DialogResult` behavior. After `ShowDialog`, read `bool MinimizeToTray` and `bool Remember`. The accepted result is `DialogResult.OK` only.
  - `sealed class LauncherTray : IDisposable` with `void Show(Icon icon)`, `void Hide()`, `bool Visible`, and events `EventHandler ShowMain` and `EventHandler ExitRequested`.

- [ ] **Step 1: Write the dialog and tray**

Create `cmd/kairi-launcher-winforms/ClosePromptForm.cs`:

```csharp
using System.Drawing;
using System.Windows.Forms;

namespace KairiseiLauncher
{
    internal sealed class ClosePromptForm : Form
    {
        private readonly RadioButton tray;
        private readonly RadioButton exit;
        private readonly CheckBox remember;

        public ClosePromptForm()
        {
            Text = "关闭启动器";
            Font = new Font("Microsoft YaHei UI", 10F);
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterParent;
            ClientSize = new Size(360, 180);
            tray = new RadioButton { Text = "最小化到托盘", Location = new Point(24, 20), AutoSize = true, Checked = true };
            exit = new RadioButton { Text = "直接关闭", Location = new Point(24, 52), AutoSize = true };
            remember = new CheckBox { Text = "记住选项，下次不再提示", Location = new Point(24, 88), AutoSize = true };
            var ok = new Button { Text = "确定", DialogResult = DialogResult.OK, Location = new Point(160, 130), Size = new Size(80, 30) };
            var cancel = new Button { Text = "取消", DialogResult = DialogResult.Cancel, Location = new Point(252, 130), Size = new Size(80, 30) };
            AcceptButton = ok;
            CancelButton = cancel;
            Controls.AddRange(new Control[] { tray, exit, remember, ok, cancel });
        }

        public bool MinimizeToTray { get { return tray.Checked; } }
        public bool Remember { get { return remember.Checked; } }
    }
}
```

Create `cmd/kairi-launcher-winforms/TrayIcon.cs`:

```csharp
using System;
using System.Drawing;
using System.Windows.Forms;

namespace KairiseiLauncher
{
    internal sealed class LauncherTray : IDisposable
    {
        private readonly NotifyIcon icon = new NotifyIcon();
        public event EventHandler ShowMain;
        public event EventHandler ExitRequested;
        public bool Visible { get; private set; }

        public LauncherTray()
        {
            var menu = new ContextMenuStrip();
            var show = new ToolStripMenuItem("显示主页面");
            var exit = new ToolStripMenuItem("退出");
            show.Click += delegate { var handler = ShowMain; if (handler != null) handler(this, EventArgs.Empty); };
            exit.Click += delegate { var handler = ExitRequested; if (handler != null) handler(this, EventArgs.Empty); };
            menu.Items.Add(show);
            menu.Items.Add(exit);
            icon.ContextMenuStrip = menu;
            icon.Text = "乖离性百万亚瑟王";
            icon.Visible = false;
            icon.DoubleClick += delegate { var handler = ShowMain; if (handler != null) handler(this, EventArgs.Empty); };
        }

        public void Show(Icon source)
        {
            icon.Icon = source;
            icon.Visible = true;
            Visible = true;
        }

        public void Hide()
        {
            icon.Visible = false;
            Visible = false;
        }

        public void Dispose()
        {
            icon.Dispose();
        }
    }
}
```

`ClosePromptForm` does not write the preference file. Add both files to the `csc` argument list.

- [ ] **Step 2: Build**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0, and `--self-test` still passes because this task does not change it.

- [ ] **Step 3: Commit**

```bash
git add cmd/kairi-launcher-winforms/ClosePromptForm.cs cmd/kairi-launcher-winforms/TrayIcon.cs tools/powershell/Build-Launcher.ps1
git commit -m "Add launcher close dialog and tray icon"
```

---

### Task 7: Single-column window

**Files:**
- Create: `cmd/kairi-launcher-winforms/LauncherForm.cs`
- Modify: `cmd/kairi-launcher-winforms/Program.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: `LauncherPreferencesStore`, `ThemeRules`, `ThemePalette`, `ThemeIcon`, `TitleBarTheme`, `ServiceButton`, `ClosePolicy`, `ClosePromptForm`, `LauncherTray`, `ServerController`, `ServerState`, `DeploymentDefaults`
- Produces: `sealed class LauncherForm : Form`

- [ ] **Step 1: Move the window out of Program.cs**

Delete the entire `LauncherForm` class from `Program.cs`. Keep `ServerState`, `DeploymentDefaults`, and `Program`. `Program.Main` still handles `--smoke-test` and `--self-test`, then enables visual styles and runs `new LauncherForm()`.

Create `LauncherForm.cs` in namespace `KairiseiLauncher`. Copy these methods from the deleted class without changing their bodies: `PopulateAddresses`, `LoadDeploymentDefaults`, `RestoreDeploymentDefaults`, `IsPrivate`, `ParseIPv4`, `ValidateAddress`, `NormalizeHost`, `StartServer`, `StopServer`, `RunOperation`, `Quote`, `RefreshStatus`, `OpenAdmin`, `OpenLogs`, `ShowFailure`.

Change the copied `OpenAdmin` message text from `服务尚未运行，请先点击“启动服务”。` to `服务尚未运行，请先点击「启动服务」。` The activity text that mentions the start button uses `「启动服务」` as well.

Replace `ApplyMode` so it no longer writes a hint label. It still sets the address text and `address.Enabled` exactly as before. After that, set the question-mark tooltip to the three strings in Global Constraints, chosen by `mode.SelectedIndex` 0, 1, or 2.

Replace `ApplyState` and `SetBusy` with one method that:

- sets `initialCheckComplete` true after the first successful `RefreshStatus`
- tracks `operationKind` as none, start, or stop around `RunOperation`
- declares `enum OperationKind { None, Start, Stop }` and calls `ServiceButton.Resolve(initialCheckComplete, latestState != null, latestState != null && latestState.IsReady, operationKind == OperationKind.Start, operationKind == OperationKind.Stop)`
- applies that result to the single power button text, enabled flag, and color from the active `ThemePalette`
- sets the status text from Global Constraints
- leaves address, mode, and port disabled while `operationKind` is not none, and leaves address disabled when the mode is Android 模拟器
- enables `Admin面板` only when `latestState != null && latestState.IsReady`

`RunOperation` must set `operationKind` before the awaited call and clear it in `finally`. Starting sets start. Stopping sets stop.

- [ ] **Step 2: Build the single-column layout**

`ClientSize` is `460, 640`. `MinimumSize` is `476, 700`. `Font` is `Microsoft YaHei UI` 10pt. `AutoScaleMode` ends as `Dpi`, matching the current form. Text is `乖离性百万亚瑟王 · 服务端启动器`.

Dock from the outside in:

- Top header panel, height 96: 48px product icon, title `乖离性百万亚瑟王`, status label anchored right, subtitle `国服本地服务 · Windows x64`, link `本项目永久免费 · 从 GitHub 获取` with the existing GitHub click handler.
- Bottom footer label: `请完整解压后运行；首次启动会创建全新本地存档。`
- Fill log `TextBox`: multiline, read-only, vertical scroll.
- Top connection panel: `使用场景` combo with `Android 模拟器`, `局域网`, `公网 IP / 域名`; label `服务器地址` immediately followed by a 16 by 16 `?` control; address combo and `恢复默认` on the next row; `主端口` `NumericUpDown` range 1 through 65533, value 26020; under it a label `游戏 26020 · 战斗 26021 · Admin 127.0.0.1:26022` updated by the existing `UpdateClientConfig` math.
- Top action panel: one power button docked top, full width, height 40, initial text `确认服务中`, disabled. Under it, `Admin面板` and `查看日志` split the width. Wire them to `StartServer` or `StopServer` based on `ServiceButtonKind`, `OpenAdmin`, and `OpenLogs`.

There is no GroupBox titled `连接`, no label containing `战斗端口 = 主端口 + 1`, and no `关闭窗口时` control.

`ApplyPalette(ThemePalette)` sets `BackColor`, `ForeColor`, button colors, and the log colors from the palette. Call it from the constructor with `ThemePalette.For(ResolvedTheme.Light)` and again whenever the resolved theme changes.

Load preferences from `LauncherPreferencesStore.DefaultPath` in the constructor. Read `AppsUseLightTheme` with `Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize")`. A missing key or missing value is `null`, which `ThemeRules.Resolve` treats as light. On each status tick, if `Theme` is `System`, read it again and apply when the resolved theme changes.

The theme button is a 16 by 16 owner-drawn control. `ThemeIcon.Draw` paints the current `ThemeChoice`. Its tooltip is `ThemeRules.Caption`. Click calls `ThemeRules.Next`, `TrySave`s the preferences, and applies the palette. Place it by reading caption-button bounds:

```csharp
[DllImport("dwmapi.dll")]
private static extern int DwmGetWindowAttribute(IntPtr window, int attribute, out RECT bounds, int size);

[StructLayout(LayoutKind.Sequential)]
private struct RECT { public int Left, Top, Right, Bottom; }
```

Attribute `5` is `DWMWA_CAPTION_BUTTON_BOUNDS`. Those coordinates are relative to the window. Position a borderless owned `Form` of 16 by 16 device pixels immediately left of that rectangle, showing the same theme button. On move, resize, and DPI change, reposition it. If the API fails, the rectangle is empty, or the 16px slot would start left of `window.Left + 48`, hide that popup and show the 16px button at the top-right of the client area instead. Both controls share one click handler.

Call `TitleBarTheme.Apply(Handle, resolved)` after the handle exists and whenever the resolved theme changes. Pass `titleBarIsDark: resolved == ResolvedTheme.Dark` into `ThemeIcon.Draw`.

Add `LauncherForm.cs` to the `csc` file list.

- [ ] **Step 3: Build**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add cmd/kairi-launcher-winforms/LauncherForm.cs cmd/kairi-launcher-winforms/Program.cs tools/powershell/Build-Launcher.ps1
git commit -m "Relayout the launcher as a single column"
```

---

### Task 8: Tray, exit, shutdown, and single instance

**Files:**
- Create: `cmd/kairi-launcher-winforms/SingleInstance.cs`
- Modify: `cmd/kairi-launcher-winforms/LauncherForm.cs`
- Modify: `cmd/kairi-launcher-winforms/Program.cs`
- Modify: `tools/powershell/Build-Launcher.ps1`

**Interfaces:**
- Consumes: `ClosePolicy`, `LauncherTray`, `ClosePromptForm`, `ServerController.StopAsync`
- Produces:
  - `static class SingleInstance` with `bool TryBecomePrimary(out Mutex mutex)`, `void SignalExisting()`, and `void Listen(Action show)`

- [ ] **Step 1: Write single-instance activation**

Create `SingleInstance.cs`:

```csharp
using System;
using System.Threading;

namespace KairiseiLauncher
{
    internal static class SingleInstance
    {
        private const string MutexName = "Local\\KairiseiLauncher";
        private const string EventName = "Local\\KairiseiLauncher.Show";

        public static bool TryBecomePrimary(out Mutex mutex)
        {
            bool created;
            mutex = new Mutex(true, MutexName, out created);
            if (created) return true;
            mutex.Dispose();
            mutex = null;
            return false;
        }

        public static void SignalExisting()
        {
            EventWaitHandle existing;
            if (EventWaitHandle.TryOpenExisting(EventName, out existing))
            {
                using (existing) existing.Set();
            }
        }

        public static void Listen(Action show)
        {
            var handle = new EventWaitHandle(false, EventResetMode.AutoReset, EventName);
            var thread = new Thread(delegate ()
            {
                while (true)
                {
                    handle.WaitOne();
                    show();
                }
            });
            thread.IsBackground = true;
            thread.Start();
        }
    }
}
```

In `Program.Main`, after the smoke and self tests and before `Application.Run`:

```csharp
Mutex mutex;
if (!SingleInstance.TryBecomePrimary(out mutex))
{
    SingleInstance.SignalExisting();
    return 0;
}
using (mutex)
{
    Application.EnableVisualStyles();
    Application.SetCompatibleTextRenderingDefault(false);
    Application.Run(new LauncherForm());
    return 0;
}
```

`LauncherForm` calls `SingleInstance.Listen` on the UI thread startup and marshals `show` back with `BeginInvoke`. The show action calls the same restore path as the tray `显示主页面` item.

Add `SingleInstance.cs` to the `csc` file list.

- [ ] **Step 2: Handle user close**

Override `OnFormClosing`. Use a field `allowExit` so a programmatic exit does not prompt again.

- `CloseReason.TaskManagerClosing`: set `allowExit` and return without cancel.
- `CloseReason.WindowsShutDown`: do not cancel. If `latestState != null`, call stop off the UI synchronization context and ignore the result:

```csharp
Task.Run(() => controller.StopAsync()).GetAwaiter().GetResult();
```

`Task.Run` unwraps `Func<Task<OperationResult>>` into `Task<OperationResult>`, so one `GetResult()` returns the `OperationResult`. Waiting on `StopAsync()` directly from the UI thread deadlocks because that method does not use `ConfigureAwait(false)`. Then return.

- `CloseReason.UserClosing` when `allowExit` is false: `e.Cancel = true`, then `ClosePolicy.ForUserClose`. `serverRunning` is `latestState != null`. `operationBusy` is `operationKind != none`.
  - `ShowPrompt`: if `WindowState` is `Minimized`, set it to `Normal` first. `ShowDialog(this)`. On `DialogResult.OK`, compute `ClosePolicy.Remembered` and `TrySave` when it is not `Ask`. Then run the tray or exit outcome.
  - `HideToTray`: `Hide()`, `ShowInTaskbar = false`, `tray.Show(Icon)`.
  - `ExitAfterStop`: await `controller.StopAsync()` from an `async` UI method that was started after canceling the close. On `exit_code == 0`, set `allowExit` and call `Close()`. On any other code, call `ShowFailure` with the existing operation text and `ShowFromTray()`.
  - `StayBecauseBusy`: `ShowFromTray()` and `ShowFailure("另一项服务操作正在执行，请稍候。")`.
  - `ExitLauncher`: set `allowExit` and `Close()`.

Also subscribe to `Microsoft.Win32.SystemEvents.SessionEnding`. When it fires, use `ClosePolicy.ForShutdown` and the same off-context stop wait. Do not show the dialog. A field `shutdownStarted` makes a second shutdown close a no-op.

`ShowFromTray` sets `ShowInTaskbar = true`, `Show()`, `WindowState = Normal`, `Activate()`, and `tray.Hide()`.

The title-bar minimize button is left on the default `Minimize` box. Do not handle it.

- [ ] **Step 3: Build**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add cmd/kairi-launcher-winforms/SingleInstance.cs cmd/kairi-launcher-winforms/LauncherForm.cs cmd/kairi-launcher-winforms/Program.cs tools/powershell/Build-Launcher.ps1
git commit -m "Close the launcher to the tray or after stop"
```

---

### Task 9: Manual acceptance

**Files:**
- Modify: none, unless a check below fails. Then fix the smallest file that owns the failing behavior and add a failing assertion to `LauncherSelfTest` first when the failure is a pure decision.

**Interfaces:**
- Consumes: the built `_local/bin/Kairisei-Launcher.exe`

- [ ] **Step 1: Build and self-test**

Run: `powershell -NoProfile -File .\tools\powershell\Build-Launcher.ps1`

Expected: exit 0.

Run the exe with `--smoke-test` from a directory that is not a server package.

Expected: exit code 2.

- [ ] **Step 2: Click through the window**

Launch `_local/bin/Kairisei-Launcher.exe` from a built tree, or from a real server package if one is available. Confirm:

- The window is a single column: theme icon, scenario, `服务器地址` plus `?`, address, port, endpoint line, one full-width power button, `Admin面板`, `查看日志`, log.
- Minimize goes to the taskbar and does not show a tray icon.
- Close defaults to `最小化到托盘`. Cancel leaves the window open.
- Choosing tray removes the taskbar button and shows the tray icon. Double-click and `显示主页面` restore it and remove the tray icon. `退出` with no server process exits.
- Remembering `直接关闭` and closing again does not show the dialog. With no server, the process exits. Deleting `%AppData%\Kairisei\launcher-preferences.json` makes the next close ask again.
- The power button starts as `确认服务中` and becomes `启动服务` once the first check finds no server. It does not become `停止服务` before health would pass.
- The theme button cycles `跟随系统`, `浅色`, `深色`. On a dark title bar the icon keeps black and white fills and has a white outer edge. Hover shows the current name.
- The question mark is after the `服务器地址` label. Hover text changes with the scenario.
- Starting the exe again while it is open focuses the existing window instead of opening a second one.

A packaged server is required to watch a real stop during shutdown. If no package is available, record that shutdown-stop was not exercised. Do not fake a passing result.

- [ ] **Step 3: Commit any fixes**

If Step 2 required code changes, commit only those fixes:

```bash
git add cmd/kairi-launcher-winforms tools/powershell/Build-Launcher.ps1
git commit -m "Fix launcher tray and theme acceptance issues"
```

If Step 2 passed with no code changes, do not create an empty commit.
