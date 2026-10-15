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
