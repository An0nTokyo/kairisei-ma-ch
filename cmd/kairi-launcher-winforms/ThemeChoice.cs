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
