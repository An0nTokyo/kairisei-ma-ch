using System.Drawing;
using System.Drawing.Drawing2D;
using System.Windows.Forms;

namespace KairiseiLauncher
{
    internal sealed class ClosePromptForm : Form
    {
        private readonly RadioButton tray;
        private readonly RadioButton exit;
        private readonly CheckBox remember;

        public ClosePromptForm(ThemePalette palette)
        {
            Text = "关闭启动器";
            Font = new Font("Microsoft YaHei UI", 10F);
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterParent;
            ClientSize = new Size(360, 188);
            BackColor = palette.Canvas;
            ForeColor = palette.Text;
            tray = new RadioButton { Text = "最小化到托盘", Location = new Point(24, 20), AutoSize = true, Checked = true, BackColor = palette.Canvas, ForeColor = palette.Text };
            exit = new RadioButton { Text = "直接关闭", Location = new Point(24, 52), AutoSize = true, BackColor = palette.Canvas, ForeColor = palette.Muted };
            remember = new CheckBox { Text = "记住选项，下次不再提示", Location = new Point(24, 92), AutoSize = true, BackColor = palette.Canvas, ForeColor = palette.Text };
            var ok = MakeDialogButton("确定", 168, 140, palette.Accent, Color.White);
            ok.DialogResult = DialogResult.OK;
            var cancel = MakeDialogButton("取消", 256, 140, palette.Surface, palette.Text);
            cancel.DialogResult = DialogResult.Cancel;
            AcceptButton = ok;
            CancelButton = cancel;
            Controls.AddRange(new Control[] { tray, exit, remember, ok, cancel });
        }

        public bool MinimizeToTray { get { return tray.Checked; } }
        public bool Remember { get { return remember.Checked; } }

        private static Button MakeDialogButton(string text, int x, int y, Color back, Color fore)
        {
            var button = new Button
            {
                Text = text,
                Location = new Point(x, y),
                Size = new Size(80, 32),
                FlatStyle = FlatStyle.Flat,
                BackColor = back,
                ForeColor = fore,
                Cursor = Cursors.Hand,
                UseVisualStyleBackColor = false
            };
            button.FlatAppearance.BorderSize = 0;
            button.FlatAppearance.MouseOverBackColor = back;
            button.FlatAppearance.MouseDownBackColor = back;
            button.Resize += delegate { Round(button, 8); };
            Round(button, 8);
            return button;
        }

        private static void Round(Control control, int radius)
        {
            if (control.Width < radius * 2 || control.Height < radius * 2) return;
            using (var path = new GraphicsPath())
            {
                int d = radius * 2;
                var bounds = new Rectangle(0, 0, control.Width, control.Height);
                path.AddArc(bounds.X, bounds.Y, d, d, 180, 90);
                path.AddArc(bounds.Right - d, bounds.Y, d, d, 270, 90);
                path.AddArc(bounds.Right - d, bounds.Bottom - d, d, d, 0, 90);
                path.AddArc(bounds.X, bounds.Bottom - d, d, d, 90, 90);
                path.CloseFigure();
                control.Region = new Region(path);
            }
        }
    }
}
