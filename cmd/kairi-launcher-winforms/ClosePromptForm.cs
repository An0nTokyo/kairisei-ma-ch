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
