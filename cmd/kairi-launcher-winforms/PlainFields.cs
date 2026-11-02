using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Windows.Forms;

namespace KairiseiLauncher
{
    internal sealed class FieldItemList
    {
        private readonly List<string> items = new List<string>();

        public int Count { get { return items.Count; } }
        public string this[int index] { get { return items[index]; } }

        public void Add(string value)
        {
            items.Add(value);
        }

        public void AddRange(object[] values)
        {
            if (values == null) return;
            for (int i = 0; i < values.Length; i++)
                if (values[i] != null) items.Add(values[i].ToString());
        }

        public List<string> Snapshot()
        {
            return new List<string>(items);
        }
    }

    internal static class FieldChrome
    {
        public static Color Fill(ThemePalette palette, bool enabled, bool dark)
        {
            if (palette == null) return Color.FromArgb(244, 249, 252);
            if (enabled) return palette.Canvas;
            return dark ? Color.FromArgb(24, 32, 42) : Color.FromArgb(238, 243, 246);
        }

        public static GraphicsPath Round(Rectangle bounds, int radius)
        {
            var path = new GraphicsPath();
            int width = Math.Max(0, bounds.Width);
            int height = Math.Max(0, bounds.Height);
            if (width < 2 || height < 2)
            {
                path.AddRectangle(new Rectangle(bounds.X, bounds.Y, Math.Max(1, width), Math.Max(1, height)));
                return path;
            }
            int diameter = radius * 2;
            if (diameter > width) diameter = width;
            if (diameter > height) diameter = height;
            if ((diameter % 2) != 0) diameter -= 1;
            if (diameter < 2)
            {
                path.AddRectangle(new Rectangle(bounds.X, bounds.Y, width, height));
                return path;
            }
            int right = bounds.X + width;
            int bottom = bounds.Y + height;
            path.AddArc(bounds.X, bounds.Y, diameter, diameter, 180, 90);
            path.AddArc(right - diameter, bounds.Y, diameter, diameter, 270, 90);
            path.AddArc(right - diameter, bottom - diameter, diameter, diameter, 0, 90);
            path.AddArc(bounds.X, bottom - diameter, diameter, diameter, 90, 90);
            path.CloseFigure();
            return path;
        }

        public static void PaintBox(Graphics graphics, Rectangle bounds, ThemePalette palette, bool enabled, bool dark)
        {
            graphics.SmoothingMode = SmoothingMode.AntiAlias;
            var rect = new Rectangle(bounds.X, bounds.Y, Math.Max(1, bounds.Width - 1), Math.Max(1, bounds.Height - 1));
            using (var path = Round(rect, 8))
            using (var fill = new SolidBrush(Fill(palette, enabled, dark)))
            using (var pen = new Pen(palette == null ? Color.FromArgb(213, 228, 238) : palette.Line))
            {
                graphics.FillPath(fill, path);
                graphics.DrawPath(pen, path);
            }
        }
    }

    internal sealed class PlainSelect : Control
    {
        private readonly FieldItemList items = new FieldItemList();
        private int selectedIndex = -1;
        private ThemePalette palette;
        private bool dark;
        private FieldListPopup popup;

        public PlainSelect()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
            Font = new Font("Microsoft YaHei UI", 10F);
            Cursor = Cursors.Hand;
            Height = 36;
            MinimumSize = new Size(80, 36);
        }

        public FieldItemList Items { get { return items; } }
        public event EventHandler SelectedIndexChanged;

        public int SelectedIndex
        {
            get { return selectedIndex; }
            set
            {
                if (value == selectedIndex) return;
                selectedIndex = value;
                Invalidate();
                if (SelectedIndexChanged != null) SelectedIndexChanged(this, EventArgs.Empty);
            }
        }

        public void ApplyChrome(ThemePalette next, bool titleIsDark)
        {
            palette = next;
            dark = titleIsDark;
            Invalidate();
        }

        public override Size GetPreferredSize(Size proposedSize)
        {
            return new Size(Math.Max(80, proposedSize.Width), 36);
        }

        protected override void OnMouseDown(MouseEventArgs e)
        {
            base.OnMouseDown(e);
            if (!Enabled) return;
            TogglePopup();
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            FieldChrome.PaintBox(e.Graphics, ClientRectangle, palette, Enabled, dark);
            string text = selectedIndex >= 0 && selectedIndex < items.Count ? items[selectedIndex] : "";
            var color = !Enabled || palette == null ? (palette == null ? Color.FromArgb(77, 112, 134) : (Enabled ? palette.Text : palette.Muted)) : (Enabled ? palette.Text : palette.Muted);
            using (var brush = new SolidBrush(color))
            using (var format = new StringFormat { Alignment = StringAlignment.Near, LineAlignment = StringAlignment.Center, Trimming = StringTrimming.EllipsisCharacter, FormatFlags = StringFormatFlags.NoWrap })
                e.Graphics.DrawString(text, Font, brush, new RectangleF(12, 0, Math.Max(1, Width - 24), Height), format);
        }

        protected override void OnEnabledChanged(EventArgs e)
        {
            base.OnEnabledChanged(e);
            Invalidate();
        }

        private void TogglePopup()
        {
            if (popup != null && !popup.IsDisposed) { popup.Close(); return; }
            if (items.Count == 0) return;
            popup = new FieldListPopup(items.Snapshot(), palette, dark, selectedIndex);
            popup.Chosen += delegate(int index) { SelectedIndex = index; };
            var origin = PointToScreen(new Point(0, Height + 4));
            popup.ShowAt(FindForm(), origin, Width);
        }
    }

    internal sealed class PlainAddress : UserControl
    {
        private readonly TextBox edit = new TextBox();
        private readonly FieldItemList items = new FieldItemList();
        private ThemePalette palette;
        private bool dark;
        private FieldListPopup popup;

        public PlainAddress()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
            Font = new Font("Microsoft YaHei UI", 10F);
            Height = 36;
            MinimumSize = new Size(80, 36);
            edit.BorderStyle = BorderStyle.None;
            edit.Font = Font;
            edit.Text = "10.0.2.2";
            edit.TextChanged += delegate { Invalidate(); OnTextChanged(EventArgs.Empty); };
            edit.MouseDown += delegate(object sender, MouseEventArgs e)
            {
                if (e.X >= edit.Width - 4 && items.Count > 0) ShowList();
            };
            Controls.Add(edit);
        }

        public FieldItemList Items { get { return items; } }

        public override string Text
        {
            get { return edit.Text; }
            set { edit.Text = value ?? ""; }
        }

        public void ApplyChrome(ThemePalette next, bool titleIsDark)
        {
            palette = next;
            dark = titleIsDark;
            Color fill = FieldChrome.Fill(palette, Enabled, dark);
            edit.BackColor = fill;
            edit.ForeColor = palette == null ? SystemColors.WindowText : (Enabled ? palette.Text : palette.Muted);
            Invalidate();
        }

        public override Size GetPreferredSize(Size proposedSize)
        {
            return new Size(Math.Max(80, proposedSize.Width), 36);
        }

        protected override void OnResize(EventArgs e)
        {
            base.OnResize(e);
            int right = ShowChevron ? 28 : 12;
            edit.Bounds = new Rectangle(12, 9, Math.Max(10, Width - 12 - right), Math.Max(10, Height - 18));
            if (Width < 2 || Height < 2) return;
            using (var path = FieldChrome.Round(new Rectangle(0, 0, Width - 1, Height - 1), 8))
                Region = new Region(path);
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            FieldChrome.PaintBox(e.Graphics, ClientRectangle, palette, Enabled, dark);
            if (!Enabled)
            {
                using (var brush = new SolidBrush(palette == null ? Color.FromArgb(77, 112, 134) : palette.Muted))
                using (var format = new StringFormat { Alignment = StringAlignment.Near, LineAlignment = StringAlignment.Center, FormatFlags = StringFormatFlags.NoWrap })
                    e.Graphics.DrawString(edit.Text, Font, brush, new RectangleF(12, 0, Math.Max(1, Width - 24), Height), format);
            }
            else if (ShowChevron && palette != null)
            {
                using (var pen = new Pen(palette.Muted, 1.5f))
                {
                    int x = Width - 18;
                    int y = Height / 2 - 2;
                    e.Graphics.DrawLine(pen, x, y, x + 4, y + 4);
                    e.Graphics.DrawLine(pen, x + 4, y + 4, x + 8, y);
                }
            }
        }

        protected override void OnMouseDown(MouseEventArgs e)
        {
            base.OnMouseDown(e);
            if (Enabled && ShowChevron && e.X >= Width - 28) ShowList();
        }

        protected override void OnEnabledChanged(EventArgs e)
        {
            base.OnEnabledChanged(e);
            edit.Visible = Enabled;
            ApplyChrome(palette, dark);
        }

        private bool ShowChevron { get { return Enabled && items.Count > 0; } }

        private void ShowList()
        {
            if (popup != null && !popup.IsDisposed) { popup.Close(); return; }
            popup = new FieldListPopup(items.Snapshot(), palette, dark, -1);
            popup.Chosen += delegate(int index)
            {
                if (index >= 0 && index < items.Count) Text = items[index];
            };
            popup.ShowAt(FindForm(), PointToScreen(new Point(0, Height + 4)), Width);
        }
    }

    internal sealed class PlainPort : UserControl
    {
        private readonly TextBox edit = new TextBox();
        private decimal current = 26020;
        private ThemePalette palette;
        private bool dark;

        public PlainPort()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
            Font = new Font("Microsoft YaHei UI", 10F);
            Width = 112;
            Height = 36;
            Minimum = 1;
            Maximum = 65533;
            edit.BorderStyle = BorderStyle.None;
            edit.Font = Font;
            edit.Text = "26020";
            edit.KeyPress += OnKeyPress;
            edit.Leave += delegate { CommitText(); };
            edit.TextChanged += delegate { Invalidate(); };
            Controls.Add(edit);
        }

        public decimal Minimum { get; set; }
        public decimal Maximum { get; set; }
        public event EventHandler ValueChanged;

        public decimal Value
        {
            get { return current; }
            set { SetValue(value, true); }
        }

        public void ApplyChrome(ThemePalette next, bool titleIsDark)
        {
            palette = next;
            dark = titleIsDark;
            edit.BackColor = FieldChrome.Fill(palette, Enabled, dark);
            edit.ForeColor = palette == null ? SystemColors.WindowText : palette.Text;
            Invalidate();
        }

        protected override void OnResize(EventArgs e)
        {
            base.OnResize(e);
            edit.Bounds = new Rectangle(12, 9, Math.Max(10, Width - 24), Math.Max(10, Height - 18));
            if (Width < 2 || Height < 2) return;
            using (var path = FieldChrome.Round(new Rectangle(0, 0, Width - 1, Height - 1), 8))
                Region = new Region(path);
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            FieldChrome.PaintBox(e.Graphics, ClientRectangle, palette, Enabled, dark);
            if (!Enabled)
            {
                using (var brush = new SolidBrush(palette == null ? Color.FromArgb(77, 112, 134) : palette.Muted))
                using (var format = new StringFormat { Alignment = StringAlignment.Near, LineAlignment = StringAlignment.Center, FormatFlags = StringFormatFlags.NoWrap })
                    e.Graphics.DrawString(edit.Text, Font, brush, new RectangleF(12, 0, Math.Max(1, Width - 24), Height), format);
            }
        }

        protected override void OnEnabledChanged(EventArgs e)
        {
            base.OnEnabledChanged(e);
            edit.Visible = Enabled;
            ApplyChrome(palette, dark);
        }

        private void OnKeyPress(object sender, KeyPressEventArgs e)
        {
            if (e.KeyChar == (char)Keys.Return)
            {
                CommitText();
                e.Handled = true;
                return;
            }
            if (char.IsControl(e.KeyChar)) return;
            if (!char.IsDigit(e.KeyChar)) e.Handled = true;
        }

        private void CommitText()
        {
            int parsed;
            if (!int.TryParse(edit.Text, out parsed)) parsed = Decimal.ToInt32(current);
            SetValue(parsed, true);
        }

        private void SetValue(decimal value, bool writeText)
        {
            if (value < Minimum) value = Minimum;
            if (value > Maximum) value = Maximum;
            bool changed = value != current;
            current = value;
            if (writeText)
            {
                string text = Decimal.ToInt32(current).ToString();
                if (edit.Text != text) edit.Text = text;
            }
            if (changed && ValueChanged != null) ValueChanged(this, EventArgs.Empty);
        }
    }

    internal sealed class FieldListPopup : Form
    {
        public event Action<int> Chosen;

        public FieldListPopup(List<string> items, ThemePalette palette, bool dark, int selected)
        {
            FormBorderStyle = FormBorderStyle.None;
            ShowInTaskbar = false;
            StartPosition = FormStartPosition.Manual;
            Font = new Font("Microsoft YaHei UI", 10F);
            Color fill = palette == null || !dark ? Color.FromArgb(214, 230, 240) : palette.Canvas;
            Color edge = palette == null || !dark ? Color.FromArgb(168, 196, 214) : palette.Line;
            Color pale = palette == null ? Color.FromArgb(230, 247, 255) : palette.PaleButton;
            BackColor = edge;
            Padding = new Padding(1);
            int height = Math.Min(240, Math.Max(36, items.Count * 32 + 8));
            ClientSize = new Size(160, height);
            var list = new Panel { Dock = DockStyle.Fill, AutoScroll = true, BackColor = fill };
            Controls.Add(list);
            for (int i = items.Count - 1; i >= 0; i--)
            {
                int index = i;
                var row = new Label
                {
                    Text = items[i],
                    AutoSize = false,
                    Height = 32,
                    Dock = DockStyle.Top,
                    TextAlign = ContentAlignment.MiddleLeft,
                    Padding = new Padding(12, 0, 12, 0),
                    BackColor = i == selected ? pale : fill,
                    ForeColor = palette == null ? Color.FromArgb(23, 55, 78) : palette.Text,
                    Cursor = Cursors.Hand
                };
                row.MouseEnter += delegate { row.BackColor = pale; };
                row.MouseLeave += delegate
                {
                    row.BackColor = index == selected ? pale : fill;
                };
                row.Click += delegate { if (Chosen != null) Chosen(index); Close(); };
                list.Controls.Add(row);
            }
            Deactivate += delegate { Close(); };
            using (var path = FieldChrome.Round(new Rectangle(0, 0, 160, height), 8))
                Region = new Region(path);
        }

        public void ShowAt(Form owner, Point screen, int width)
        {
            Width = Math.Max(120, width);
            var area = Screen.FromPoint(screen).WorkingArea;
            int x = screen.X;
            int y = screen.Y;
            if (y + Height > area.Bottom) y = screen.Y - Height - 40;
            if (x + Width > area.Right) x = area.Right - Width;
            Location = new Point(x, y);
            using (var path = FieldChrome.Round(new Rectangle(0, 0, Width - 1, Height - 1), 8))
                Region = new Region(path);
            Show(owner);
        }
    }
}
