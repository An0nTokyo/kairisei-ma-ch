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
        public Color Line;

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
                palette.Line = Color.FromArgb(58, 70, 88);
                return palette;
            }
            palette.Canvas = Color.FromArgb(244, 249, 252);
            palette.Surface = Color.White;
            palette.Text = Color.FromArgb(23, 55, 78);
            palette.Muted = Color.FromArgb(77, 112, 134);
            palette.Accent = Color.FromArgb(11, 103, 159);
            palette.Success = Color.FromArgb(42, 157, 100);
            palette.PaleButton = Color.FromArgb(230, 247, 255);
            palette.Line = Color.FromArgb(213, 228, 238);
            return palette;
        }
    }

    internal static class ThemeIcon
    {
        public static void Draw(Graphics graphics, Rectangle bounds, ThemeChoice glyph, bool onDark)
        {
            if (bounds.Width <= 0 || bounds.Height <= 0) return;
            var color = onDark ? Color.FromArgb(230, 238, 244) : Color.FromArgb(23, 55, 78);
            var state = graphics.Save();
            graphics.SmoothingMode = SmoothingMode.AntiAlias;
            float size = Math.Min(bounds.Width, bounds.Height);
            graphics.TranslateTransform(bounds.X + ((bounds.Width - size) / 2f), bounds.Y + ((bounds.Height - size) / 2f));
            graphics.ScaleTransform(size / 24f, size / 24f);
            using (var brush = new SolidBrush(color))
            {
                if (glyph != ThemeChoice.Dark)
                {
                    using (var ring = Ring()) graphics.FillPath(brush, ring);
                }
                if (glyph != ThemeChoice.Light)
                {
                    using (var moon = Crescent()) graphics.FillPath(brush, moon);
                }
            }
            graphics.Restore(state);
        }

        private static GraphicsPath Ring()
        {
            var path = new GraphicsPath(FillMode.Alternate);
            path.AddEllipse(2f, 2f, 20f, 20f);
            path.AddEllipse(4f, 4f, 16f, 16f);
            return path;
        }

        private static GraphicsPath Crescent()
        {
            var path = new GraphicsPath();
            path.AddBezier(11.3807f, 2.01886f, 9.91573f, 3.38768f, 9f, 5.3369f, 9f, 7.49999f);
            path.AddBezier(9f, 7.49999f, 9f, 11.6421f, 12.3579f, 15f, 16.5f, 15f);
            path.AddBezier(16.5f, 15f, 18.6631f, 15f, 20.6123f, 14.0843f, 21.9811f, 12.6193f);
            path.AddBezier(21.9811f, 12.6193f, 21.6613f, 17.8537f, 17.3149f, 22f, 12f, 22f);
            path.AddBezier(12f, 22f, 6.47715f, 22f, 2f, 17.5228f, 2f, 12f);
            path.AddBezier(2f, 12f, 2f, 6.68514f, 6.14629f, 2.33869f, 11.3807f, 2.01886f);
            path.CloseFigure();
            return path;
        }
    }

    internal static class QuestionGlyph
    {
        public static void Draw(Graphics graphics, Rectangle bounds, Color color)
        {
            if (bounds.Width <= 0 || bounds.Height <= 0) return;
            var state = graphics.Save();
            graphics.SmoothingMode = SmoothingMode.AntiAlias;
            float size = Math.Min(bounds.Width, bounds.Height);
            graphics.TranslateTransform(bounds.X + ((bounds.Width - size) / 2f), bounds.Y + ((bounds.Height - size) / 2f));
            graphics.ScaleTransform(size / 24f, size / 24f);
            using (var brush = new SolidBrush(color))
            using (var ring = new GraphicsPath(FillMode.Alternate))
            using (var mark = new GraphicsPath())
            {
                ring.AddEllipse(2f, 2f, 20f, 20f);
                ring.AddEllipse(4f, 4f, 16f, 16f);
                mark.AddRectangle(new RectangleF(11f, 15f, 2f, 2f));
                mark.AddLine(13f, 13.3551f, 13f, 14f);
                mark.AddLine(13f, 14f, 11f, 14f);
                mark.AddLine(11f, 14f, 11f, 12.5f);
                mark.AddBezier(11f, 12.5f, 11f, 11.9477f, 11.4477f, 11.5f, 12f, 11.5f);
                mark.AddBezier(12f, 11.5f, 12.8284f, 11.5f, 13.5f, 10.8284f, 13.5f, 10f);
                mark.AddBezier(13.5f, 10f, 13.5f, 9.17157f, 12.8284f, 8.5f, 12f, 8.5f);
                mark.AddBezier(12f, 8.5f, 11.2723f, 8.5f, 10.6656f, 9.01823f, 10.5288f, 9.70577f);
                mark.AddLine(10.5288f, 9.70577f, 8.56731f, 9.31346f);
                mark.AddBezier(8.56731f, 9.31346f, 8.88637f, 7.70919f, 10.302f, 6.5f, 12f, 6.5f);
                mark.AddBezier(12f, 6.5f, 13.933f, 6.5f, 15.5f, 8.067f, 15.5f, 10f);
                mark.AddBezier(15.5f, 10f, 15.5f, 11.5855f, 14.4457f, 12.9248f, 13f, 13.3551f);
                mark.CloseFigure();
                graphics.FillPath(brush, ring);
                graphics.FillPath(brush, mark);
            }
            graphics.Restore(state);
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
