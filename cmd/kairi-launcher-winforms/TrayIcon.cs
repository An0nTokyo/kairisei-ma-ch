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
