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
            float cx = bounds.X + (bounds.Width / 2f);
            float cy = bounds.Y + (bounds.Height / 2f);
            float radius = Math.Min(bounds.Width, bounds.Height) / 2f - 2f;
            if (radius < 2f) radius = Math.Min(bounds.Width, bounds.Height) / 2f;
            float shift = bounds.Width * (10f / 32f);
            var sun = Ellipse(cx, cy, radius);
            var moon = Ellipse(cx + shift, cy, radius);
            using (sun)
            using (moon)
            using (var crescent = new Region(sun))
            {
                crescent.Exclude(moon);
                if (glyph == ThemeChoice.Dark && titleBarIsDark)
                {
                    using (var haloSun = Ellipse(cx, cy, radius + 2f))
                    using (var halo = new Region(haloSun))
                    {
                        halo.Exclude(moon);
                        using (var fill = new SolidBrush(Color.White)) graphics.FillRegion(fill, halo);
                    }
                }
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
                        using (var halo = new Pen(Color.White, 3f)) graphics.DrawPath(halo, sun);
                    }
                    using (var edge = new Pen(Color.Black, 1.5f)) graphics.DrawPath(edge, sun);
                }
            }
        }

        private static GraphicsPath Ellipse(float cx, float cy, float radius)
        {
            var path = new GraphicsPath();
            path.AddEllipse(cx - radius, cy - radius, radius * 2f, radius * 2f);
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
