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
