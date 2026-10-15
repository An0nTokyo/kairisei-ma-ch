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
            ThemeTests(failures);
            ServiceButtonTests(failures);
            CloseTests(failures);
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
    }
}
