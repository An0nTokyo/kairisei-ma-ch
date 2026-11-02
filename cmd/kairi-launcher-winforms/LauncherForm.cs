using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Text;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows.Forms;
using Microsoft.Win32;

namespace KairiseiLauncher
{
    internal sealed class LauncherForm : Form
    {
        private enum OperationKind
        {
            None,
            Start,
            Stop
        }

        private readonly string root;
        private readonly ServerController controller;
        private readonly JavaScriptSerializer serializer = new JavaScriptSerializer();
        private readonly Timer timer = new Timer();
        private readonly ToolTip toolTip = new ToolTip();
        private readonly LauncherTray tray = new LauncherTray();
        private PlainSelect mode;
        private PlainAddress address;
        private PlainPort port;
        private Label stateBadge;
        private Label endpoints;
        private TextBox activity;
        private Button powerButton;
        private Button adminButton;
        private Button logsButton;
        private Button restoreButton;
        private Button clientThemeButton;
        private Label questionMark;
        private PictureBox iconBox;
        private TableLayoutPanel connectionCard;
        private FlowLayoutPanel addressHead;
        private TableLayoutPanel addressRow;
        private Panel logHost;
        private readonly System.Collections.Generic.List<Label> mutedLabels = new System.Collections.Generic.List<Label>();
        private string packagedHost = "10.0.2.2";
        private int packagedPort = 26020;
        private bool validationOnly;
        private bool busy;
        private bool refreshing;
        private bool initialCheckComplete;
        private OperationKind operationKind;
        private ServerState latestState;
        private string operationMessage = "";
        private bool promptOpen;
        private bool allowExit;
        private bool shutdownStarted;
        private LauncherPreferences preferences;
        private ThemeChoice themeChoice;
        private ResolvedTheme resolved;
        private ThemePalette palette;

        public LauncherForm()
        {
            root = AppDomain.CurrentDomain.BaseDirectory.TrimEnd(Path.DirectorySeparatorChar);
            controller = new ServerController(root);
            preferences = LauncherPreferencesStore.Load(LauncherPreferencesStore.DefaultPath);
            themeChoice = preferences.Theme;
            resolved = ThemeRules.Resolve(themeChoice, ReadAppsUseLightTheme());
            palette = ThemePalette.For(resolved);
            tray.ShowMain += delegate { ShowFromTray(); };
            tray.ExitRequested += delegate { BeginForcedExit(); };
            SystemEvents.SessionEnding += OnSessionEnding;
            BuildUI();
            ApplyPalette();
            LoadDeploymentDefaults();
            HandleCreated += delegate
            {
                TitleBarTheme.Apply(Handle, resolved);
                SingleInstance.Listen(delegate
                {
                    if (IsHandleCreated) BeginInvoke(new Action(ShowFromTray));
                    else ShowFromTray();
                });
            };
            Shown += async delegate
            {
                PopulateAddresses();
                await RefreshStatus(true);
                if (IsDisposed || Disposing) return;
                timer.Interval = 1500;
                timer.Tick += async delegate { await RefreshStatus(false); };
                timer.Start();
            };
        }

        protected override void OnFormClosing(FormClosingEventArgs e)
        {
            if (e.CloseReason == CloseReason.TaskManagerClosing)
            {
                allowExit = true;
                base.OnFormClosing(e);
                return;
            }
            if (e.CloseReason == CloseReason.WindowsShutDown)
            {
                if (!shutdownStarted && latestState != null)
                {
                    shutdownStarted = true;
                    Task.Run(() => controller.StopAsync()).GetAwaiter().GetResult();
                }
                allowExit = true;
                base.OnFormClosing(e);
                return;
            }
            if (e.CloseReason == CloseReason.UserClosing && !allowExit)
            {
                e.Cancel = true;
                if (!promptOpen)
                {
                    promptOpen = true;
                    BeginInvoke(new Action(delegate
                    {
                        try { BeginUserClose(); }
                        finally { promptOpen = false; }
                    }));
                }
                return;
            }
            base.OnFormClosing(e);
        }

        protected override void OnFormClosed(FormClosedEventArgs e)
        {
            SystemEvents.SessionEnding -= OnSessionEnding;
            timer.Stop();
            tray.Dispose();
            base.OnFormClosed(e);
        }

        private void OnSessionEnding(object sender, SessionEndingEventArgs args)
        {
            if (shutdownStarted) return;
            shutdownStarted = true;
            if (ClosePolicy.ForShutdown(latestState != null) == CloseOutcome.ExitAfterStop)
                Task.Run(() => controller.StopAsync()).GetAwaiter().GetResult();
        }

        private void BeginUserClose()
        {
            var outcome = ClosePolicy.ForUserClose(preferences.CloseAction, latestState != null, operationKind != OperationKind.None);
            if (outcome == CloseOutcome.ShowPrompt)
            {
                if (WindowState == FormWindowState.Minimized) WindowState = FormWindowState.Normal;
                using (var prompt = new ClosePromptForm(palette))
                {
                    if (prompt.ShowDialog(this) != DialogResult.OK) return;
                    var remembered = ClosePolicy.Remembered(prompt.MinimizeToTray, prompt.Remember);
                    if (remembered != CloseAction.Ask)
                    {
                        preferences.CloseAction = remembered;
                        LauncherPreferencesStore.TrySave(LauncherPreferencesStore.DefaultPath, preferences);
                    }
                    outcome = prompt.MinimizeToTray
                        ? CloseOutcome.HideToTray
                        : ClosePolicy.ForUserClose(CloseAction.Exit, latestState != null, operationKind != OperationKind.None);
                }
            }
            ApplyCloseOutcome(outcome);
        }

        private void BeginForcedExit()
        {
            ApplyCloseOutcome(ClosePolicy.ForUserClose(CloseAction.Exit, latestState != null, operationKind != OperationKind.None));
        }

        private void ApplyCloseOutcome(CloseOutcome outcome)
        {
            if (outcome == CloseOutcome.HideToTray) HideToTray();
            else if (outcome == CloseOutcome.ExitLauncher)
            {
                allowExit = true;
                Close();
            }
            else if (outcome == CloseOutcome.StayBecauseBusy)
            {
                ShowFromTray();
                ShowFailure("另一项服务操作正在执行，请稍候。");
            }
        }

        private void HideToTray()
        {
            Hide();
            ShowInTaskbar = false;
            tray.Show(Icon != null ? Icon : SystemIcons.Application);
        }

        private void ShowFromTray()
        {
            ShowInTaskbar = true;
            Show();
            if (WindowState == FormWindowState.Minimized) WindowState = FormWindowState.Normal;
            Activate();
            tray.Hide();
        }

        private void BuildUI()
        {
            Text = "乖离性百万亚瑟王 · 服务端启动器";
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(460, 640);
            MinimumSize = new Size(476, 700);
            Font = new Font("Microsoft YaHei UI", 10F);
            AutoScaleMode = AutoScaleMode.None;
            try { Icon = LoadWindowIcon(); } catch { Icon = SystemIcons.Application; }
            if (Icon == null) Icon = SystemIcons.Application;

            var layout = new TableLayoutPanel
            {
                Dock = DockStyle.Fill,
                ColumnCount = 1,
                RowCount = 5,
                Padding = new Padding(12)
            };
            layout.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
            layout.RowStyles.Add(new RowStyle(SizeType.Absolute, 96F));
            layout.RowStyles.Add(new RowStyle(SizeType.AutoSize));
            layout.RowStyles.Add(new RowStyle(SizeType.AutoSize));
            layout.RowStyles.Add(new RowStyle(SizeType.Percent, 100F));
            layout.RowStyles.Add(new RowStyle(SizeType.AutoSize));
            Controls.Add(layout);

            var header = new Panel { Dock = DockStyle.Fill };
            iconBox = new PictureBox { Location = new Point(0, 8), Size = new Size(42, 42), SizeMode = PictureBoxSizeMode.Zoom };
            iconBox.Image = Icon.ToBitmap();
            iconBox.Resize += delegate { RoundControl(iconBox, 8); };
            header.Controls.Add(iconBox);
            header.Controls.Add(new Label
            {
                Text = "乖离性百万亚瑟王",
                Location = new Point(56, 2),
                AutoSize = true,
                Font = new Font("Microsoft YaHei UI", 15F, FontStyle.Bold)
            });
            stateBadge = new Label
            {
                Text = "● 检测中",
                AutoSize = true,
                Padding = new Padding(10, 3, 10, 3),
                Font = new Font("Microsoft YaHei UI", 9F, FontStyle.Bold)
            };
            header.Controls.Add(stateBadge);
            header.Controls.Add(MakeMuted("国服本地服务 · Windows x64", new Point(56, 32)));
            var sourceLink = new LinkLabel
            {
                Text = "本项目永久免费 · 从 GitHub 获取",
                Font = new Font("Microsoft YaHei UI", 9F, FontStyle.Bold),
                LinkColor = Color.FromArgb(220, 91, 48),
                ActiveLinkColor = Color.FromArgb(222, 78, 78),
                AutoSize = true,
                Location = new Point(56, 54)
            };
            sourceLink.LinkClicked += delegate
            {
                try
                {
                    Process.Start(new ProcessStartInfo("https://github.com/kuuhaku1314/kairisei-ma-ch") { UseShellExecute = true });
                }
                catch (Exception ex) { ShowFailure("打开项目主页失败：" + ex.Message); }
            };
            header.Controls.Add(sourceLink);
            header.Resize += delegate { PlaceHeaderChrome(header); };
            layout.Controls.Add(header, 0, 0);

            connectionCard = new TableLayoutPanel
            {
                Dock = DockStyle.Top,
                AutoSize = true,
                AutoSizeMode = AutoSizeMode.GrowAndShrink,
                ColumnCount = 1,
                Padding = new Padding(12, 10, 12, 12),
                Margin = new Padding(0, 4, 0, 12)
            };
            connectionCard.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
            connectionCard.Resize += delegate { RoundControl(connectionCard, 10); };
            connectionCard.Controls.Add(MakeMuted("使用场景"), 0, 0);
            mode = new PlainSelect
            {
                Dock = DockStyle.Fill,
                Margin = new Padding(0, 2, 0, 10)
            };
            mode.Items.AddRange(new object[] { "Android 模拟器", "局域网", "公网 IP / 域名" });
            mode.SelectedIndex = 0;
            mode.SelectedIndexChanged += delegate { ApplyMode(); };
            connectionCard.Controls.Add(mode, 0, 1);
            addressHead = new FlowLayoutPanel
            {
                FlowDirection = FlowDirection.LeftToRight,
                AutoSize = true,
                WrapContents = false,
                Margin = new Padding(0, 0, 0, 4)
            };
            var addressLabel = MakeMuted("服务器地址");
            addressLabel.Margin = new Padding(0, 0, 6, 0);
            questionMark = new Label
            {
                Text = "",
                AutoSize = false,
                Size = new Size(16, 16),
                Margin = new Padding(0)
            };
            questionMark.Paint += delegate(object sender, PaintEventArgs args)
            {
                var accent = palette == null ? Color.FromArgb(11, 103, 159) : palette.Accent;
                QuestionGlyph.Draw(args.Graphics, questionMark.ClientRectangle, accent);
            };
            addressHead.Controls.Add(addressLabel);
            addressHead.Controls.Add(questionMark);
            connectionCard.Controls.Add(addressHead, 0, 2);
            addressRow = new TableLayoutPanel
            {
                Dock = DockStyle.Top,
                AutoSize = true,
                ColumnCount = 2,
                Margin = new Padding(0, 0, 0, 10)
            };
            addressRow.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
            addressRow.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 112F));
            address = new PlainAddress
            {
                Dock = DockStyle.Fill,
                Margin = new Padding(0, 0, 8, 0),
                Text = "10.0.2.2"
            };
            address.TextChanged += delegate { UpdateClientConfig(); };
            restoreButton = MakeButton("恢复默认", 0, 0, 104, 28);
            restoreButton.Dock = DockStyle.Fill;
            restoreButton.Margin = new Padding(0);
            restoreButton.Click += delegate { RestoreDeploymentDefaults(); };
            addressRow.Controls.Add(address, 0, 0);
            addressRow.Controls.Add(restoreButton, 1, 0);
            connectionCard.Controls.Add(addressRow, 0, 3);
            connectionCard.Controls.Add(MakeMuted("主端口"), 0, 4);
            port = new PlainPort
            {
                Width = 112,
                Margin = new Padding(0, 2, 0, 4),
                Minimum = 1,
                Maximum = 65533,
                Value = 26020
            };
            port.ValueChanged += delegate { UpdateClientConfig(); };
            connectionCard.Controls.Add(port, 0, 5);
            endpoints = MakeMuted("游戏 26020 · 战斗 26021 · Admin 127.0.0.1:26022");
            endpoints.Margin = new Padding(0, 8, 0, 0);
            connectionCard.Controls.Add(endpoints, 0, 6);
            layout.Controls.Add(connectionCard, 0, 1);

            var actions = new TableLayoutPanel
            {
                Dock = DockStyle.Top,
                AutoSize = true,
                ColumnCount = 2,
                Margin = new Padding(0, 0, 0, 8)
            };
            actions.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50F));
            actions.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50F));
            powerButton = MakeButton("确认服务中", 0, 0, 120, 40);
            powerButton.Dock = DockStyle.Fill;
            powerButton.Margin = new Padding(0, 0, 0, 8);
            powerButton.Enabled = false;
            powerButton.Click += async delegate { await PowerClicked(); };
            actions.SetColumnSpan(powerButton, 2);
            actions.Controls.Add(powerButton, 0, 0);
            adminButton = MakeButton("Admin面板", 0, 0, 120, 36);
            adminButton.Dock = DockStyle.Fill;
            adminButton.Margin = new Padding(0, 0, 6, 0);
            logsButton = MakeButton("查看日志", 0, 0, 120, 36);
            logsButton.Dock = DockStyle.Fill;
            logsButton.Margin = new Padding(6, 0, 0, 0);
            adminButton.Click += delegate { OpenAdmin(); };
            adminButton.Paint += delegate(object sender, PaintEventArgs args)
            {
                if (adminButton.Enabled || palette == null) return;
                args.Graphics.Clear(palette.Disabled);
                TextRenderer.DrawText(args.Graphics, adminButton.Text, adminButton.Font, adminButton.ClientRectangle, Color.White,
                    TextFormatFlags.HorizontalCenter | TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis);
            };
            logsButton.Click += delegate { OpenLogs(); };
            actions.Controls.Add(adminButton, 0, 1);
            actions.Controls.Add(logsButton, 1, 1);
            layout.Controls.Add(actions, 0, 2);

            logHost = new Panel { Dock = DockStyle.Fill, Padding = new Padding(12, 10, 8, 10), Margin = new Padding(0, 4, 0, 0) };
            logHost.Resize += delegate
            {
                logHost.Region = null;
                logHost.Invalidate();
            };
            logHost.Paint += delegate(object sender, PaintEventArgs args)
            {
                if (palette == null) return;
                args.Graphics.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.AntiAlias;
                using (var canvas = new SolidBrush(palette.Canvas))
                    args.Graphics.FillRectangle(canvas, logHost.ClientRectangle);
                using (var path = FieldChrome.Round(new Rectangle(0, 0, Math.Max(1, logHost.Width - 1), Math.Max(1, logHost.Height - 1)), 8))
                using (var fill = new SolidBrush(palette.Surface))
                using (var pen = new Pen(palette.Line))
                {
                    args.Graphics.FillPath(fill, path);
                    args.Graphics.DrawPath(pen, path);
                }
            };
            activity = new TextBox
            {
                Dock = DockStyle.Fill,
                Multiline = true,
                ReadOnly = true,
                ScrollBars = ScrollBars.Vertical,
                BorderStyle = BorderStyle.None,
                Font = new Font("Microsoft YaHei UI", 9F)
            };
            logHost.Controls.Add(activity);
            layout.Controls.Add(logHost, 0, 3);
            var footerRow = new TableLayoutPanel
            {
                Dock = DockStyle.Top,
                AutoSize = true,
                ColumnCount = 2,
                Margin = new Padding(0, 8, 0, 0)
            };
            footerRow.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
            footerRow.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 28F));
            footerRow.RowStyles.Add(new RowStyle(SizeType.AutoSize));
            var footer = MakeMuted("请完整解压后运行；首次启动会创建全新本地存档。");
            footer.Dock = DockStyle.Fill;
            footer.TextAlign = ContentAlignment.MiddleLeft;
            footer.Margin = new Padding(0, 2, 8, 0);
            clientThemeButton = new Button
            {
                Size = new Size(22, 22),
                Anchor = AnchorStyles.Right,
                FlatStyle = FlatStyle.Flat,
                TabStop = false,
                Margin = new Padding(0, 1, 0, 0)
            };
            clientThemeButton.FlatAppearance.BorderSize = 0;
            clientThemeButton.Click += delegate { CycleTheme(); };
            clientThemeButton.Paint += delegate(object sender, PaintEventArgs args) { PaintThemeGlyph(args.Graphics, clientThemeButton.ClientRectangle); };
            toolTip.SetToolTip(clientThemeButton, ThemeRules.Caption(themeChoice));
            footerRow.Controls.Add(footer, 0, 0);
            footerRow.Controls.Add(clientThemeButton, 1, 0);
            layout.Controls.Add(footerRow, 0, 4);

            ApplyMode();
            ApplyServiceButton();
            AutoScaleDimensions = new SizeF(96F, 96F);
            AutoScaleMode = AutoScaleMode.Dpi;
            PerformAutoScale();
            AlignAddressMark(addressLabel);
        }

        private void AlignAddressMark(Label label)
        {
            if (label == null || questionMark == null) return;
            label.Margin = new Padding(0, 0, 6, 0);
            int side = label.Height > 0 ? label.Height : label.PreferredSize.Height;
            questionMark.Size = new Size(side, side);
            questionMark.Margin = new Padding(0);
        }

        private Label MakeMuted(string text)
        {
            return MakeMuted(text, Point.Empty);
        }

        private Label MakeMuted(string text, Point location)
        {
            var label = new Label
            {
                Text = text,
                AutoSize = true,
                Location = location,
                Font = new Font("Microsoft YaHei UI", 9F)
            };
            mutedLabels.Add(label);
            return label;
        }

        private static Button MakeButton(string text, int x, int y, int width, int height)
        {
            var button = new Button
            {
                Text = text,
                Location = new Point(x, y),
                Size = new Size(width, height),
                FlatStyle = FlatStyle.Flat,
                Cursor = Cursors.Hand,
                UseVisualStyleBackColor = false
            };
            button.FlatAppearance.BorderSize = 0;
            button.Resize += delegate { RoundControl(button, 8); };
            return button;
        }

        private static void RoundControl(Control control, int radius)
        {
            if (control == null || control.Width < radius * 2 || control.Height < radius * 2) return;
            using (var path = new System.Drawing.Drawing2D.GraphicsPath())
            {
                int diameter = radius * 2;
                var bounds = new Rectangle(0, 0, control.Width, control.Height);
                path.AddArc(bounds.X, bounds.Y, diameter, diameter, 180, 90);
                path.AddArc(bounds.Right - diameter, bounds.Y, diameter, diameter, 270, 90);
                path.AddArc(bounds.Right - diameter, bounds.Bottom - diameter, diameter, diameter, 0, 90);
                path.AddArc(bounds.X, bounds.Bottom - diameter, diameter, diameter, 90, 90);
                path.CloseFigure();
                control.Region = new Region(path);
            }
        }

        private void ApplyPalette()
        {
            palette = ThemePalette.For(resolved);
            BackColor = palette.Canvas;
            ForeColor = palette.Text;
            ColorControls(this);
            if (connectionCard != null) connectionCard.BackColor = palette.Surface;
            if (addressHead != null) addressHead.BackColor = palette.Surface;
            if (addressRow != null) addressRow.BackColor = palette.Surface;
            if (logHost != null)
            {
                logHost.Region = null;
                logHost.BackColor = palette.Canvas;
            }
            if (activity != null)
            {
                activity.BackColor = palette.Surface;
                activity.ForeColor = palette.Muted;
            }
            bool dark = resolved == ResolvedTheme.Dark;
            if (mode != null) mode.ApplyChrome(palette, dark);
            if (address != null) address.ApplyChrome(palette, dark);
            if (port != null) port.ApplyChrome(palette, dark);
            for (int i = 0; i < mutedLabels.Count; i++)
            {
                mutedLabels[i].ForeColor = palette.Muted;
                if (mutedLabels[i].Parent != null) mutedLabels[i].BackColor = mutedLabels[i].Parent.BackColor;
            }
            if (questionMark != null && questionMark.Parent != null) questionMark.BackColor = questionMark.Parent.BackColor;
            StyleBadge();
            StyleQuietButton(restoreButton);
            StyleAdminButton();
            StyleQuietButton(logsButton);
            ApplyServiceButton();
            if (questionMark != null) questionMark.Invalidate();
            if (clientThemeButton != null)
            {
                clientThemeButton.BackColor = palette.Canvas;
                clientThemeButton.FlatAppearance.MouseOverBackColor = palette.Canvas;
                clientThemeButton.FlatAppearance.MouseDownBackColor = palette.Canvas;
                clientThemeButton.UseVisualStyleBackColor = false;
                clientThemeButton.Invalidate();
            }
            RoundControl(connectionCard, 10);
            RoundControl(iconBox, 8);
            TitleBarTheme.Apply(Handle, resolved);
        }

        private void StyleQuietButton(Button button)
        {
            if (button == null || palette == null) return;
            button.BackColor = palette.PaleButton;
            button.ForeColor = palette.Accent;
            button.FlatAppearance.MouseOverBackColor = palette.PaleButton;
            button.FlatAppearance.MouseDownBackColor = palette.PaleButton;
            RoundControl(button, 8);
        }

        private void StyleAdminButton()
        {
            if (adminButton == null || palette == null) return;
            var fill = adminButton.Enabled ? palette.PaleButton : palette.Disabled;
            var text = adminButton.Enabled ? palette.Accent : Color.White;
            adminButton.BackColor = fill;
            adminButton.ForeColor = text;
            adminButton.FlatAppearance.MouseOverBackColor = fill;
            adminButton.FlatAppearance.MouseDownBackColor = fill;
            adminButton.UseVisualStyleBackColor = false;
            RoundControl(adminButton, 8);
        }

        private void StyleBadge()
        {
            if (stateBadge == null || palette == null) return;
            bool running = latestState != null && latestState.IsReady;
            bool checking = !initialCheckComplete || (latestState != null && !latestState.IsReady);
            if (checking) stateBadge.ForeColor = palette.Muted;
            else if (running) stateBadge.ForeColor = palette.Success;
            else stateBadge.ForeColor = palette.Accent;
            if (running) stateBadge.BackColor = resolved == ResolvedTheme.Dark ? Color.FromArgb(28, 58, 46) : Color.FromArgb(226, 245, 234);
            else stateBadge.BackColor = palette.PaleButton;
            RoundControl(stateBadge, Math.Max(8, stateBadge.Height / 2));
        }

        private void ColorControls(Control parent)
        {
            foreach (Control control in parent.Controls)
            {
                if (control == powerButton || control == adminButton || control == logsButton || control == restoreButton || control == clientThemeButton) { }
                else if (control is Button)
                {
                    control.BackColor = palette.PaleButton;
                    control.ForeColor = palette.Accent;
                }
                else if (control is PlainSelect || control is PlainAddress || control is PlainPort)
                {
                    control.BackColor = palette.Surface;
                }
                else if (control is TextBox)
                {
                    control.BackColor = palette.Surface;
                    control.ForeColor = palette.Text;
                }
                else
                {
                    control.BackColor = palette.Canvas;
                    control.ForeColor = control is LinkLabel ? control.ForeColor : palette.Text;
                }
                ColorControls(control);
            }
        }

        private void CycleTheme()
        {
            themeChoice = ThemeRules.Next(themeChoice);
            preferences.Theme = themeChoice;
            LauncherPreferencesStore.TrySave(LauncherPreferencesStore.DefaultPath, preferences);
            resolved = ThemeRules.Resolve(themeChoice, ReadAppsUseLightTheme());
            toolTip.SetToolTip(clientThemeButton, ThemeRules.Caption(themeChoice));
            ApplyPalette();
        }

        private void PaintThemeGlyph(Graphics graphics, Rectangle bounds)
        {
            ThemeIcon.Draw(graphics, bounds, themeChoice, resolved == ResolvedTheme.Dark);
        }

        private static int? ReadAppsUseLightTheme()
        {
            try
            {
                using (var key = Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize"))
                {
                    if (key == null) return null;
                    var value = key.GetValue("AppsUseLightTheme");
                    if (value == null) return null;
                    return Convert.ToInt32(value);
                }
            }
            catch { return null; }
        }

        private void PlaceHeaderChrome(Control header)
        {
            if (stateBadge == null || header == null) return;
            stateBadge.Left = Math.Max(0, header.ClientSize.Width - stateBadge.PreferredSize.Width);
            stateBadge.Top = 4;
        }

        private Icon LoadWindowIcon()
        {
            Icon loaded = null;
            try { loaded = Icon.ExtractAssociatedIcon(Application.ExecutablePath); } catch { }
            if (loaded == null)
            {
                var iconFile = Path.Combine(root, "resource-set", "launcher.ico");
                if (File.Exists(iconFile))
                {
                    try { loaded = new Icon(iconFile); } catch { }
                }
            }
            return loaded ?? SystemIcons.Application;
        }

        private void PopulateAddresses()
        {
            var candidates = new SortedSet<string>(StringComparer.OrdinalIgnoreCase);
            try
            {
                foreach (var network in NetworkInterface.GetAllNetworkInterfaces())
                {
                    if (network.OperationalStatus != OperationalStatus.Up || network.NetworkInterfaceType == NetworkInterfaceType.Loopback) continue;
                    foreach (var item in network.GetIPProperties().UnicastAddresses)
                    {
                        if (item.Address.AddressFamily == AddressFamily.InterNetwork)
                            candidates.Add(item.Address.ToString());
                    }
                }
            }
            catch { }
            foreach (var candidate in candidates) address.Items.Add(candidate);
        }

        private void LoadDeploymentDefaults()
        {
            var path = Path.Combine(root, "deployment.json");
            if (!File.Exists(path)) return;
            try
            {
                var defaults = serializer.Deserialize<DeploymentDefaults>(File.ReadAllText(path, Encoding.UTF8));
                var parsed = defaults == null ? null : NormalizeHost(defaults.advertise_host);
                if (parsed == null || defaults.port < 1 || defaults.port > 65533) return;
                packagedHost = parsed;
                packagedPort = defaults.port;
                validationOnly = defaults.validation_only;
                if (validationOnly) Text += " · 测试包";
                RestoreDeploymentDefaults();
            }
            catch { }
        }

        private void RestoreDeploymentDefaults()
        {
            var host = NormalizeHost(packagedHost);
            if (host == null) return;
            var parsed = ParseIPv4(host);
            mode.SelectedIndex = host == "10.0.2.2"
                ? 0 : parsed != null && IsPrivate(parsed) ? 1 : 2;
            address.Text = host;
            port.Value = Math.Max(port.Minimum, Math.Min(port.Maximum, packagedPort));
            UpdateClientConfig();
        }

        private static bool IsPrivate(IPAddress value)
        {
            var bytes = value.GetAddressBytes();
            return bytes.Length == 4 && (bytes[0] == 10 || bytes[0] == 127 ||
                (bytes[0] == 172 && bytes[1] >= 16 && bytes[1] <= 31) ||
                (bytes[0] == 192 && bytes[1] == 168));
        }

        private void ApplyMode()
        {
            if (mode.SelectedIndex == 0)
            {
                address.Text = "10.0.2.2";
                address.Enabled = operationKind == OperationKind.None;
                if (operationKind == OperationKind.None) address.Enabled = false;
            }
            else if (mode.SelectedIndex == 1)
            {
                address.Enabled = operationKind == OperationKind.None;
                var current = ParseIPv4(address.Text);
                if (NormalizeHost(address.Text) == null || (current != null && current.ToString() == "10.0.2.2"))
                    address.Text = address.Items.Count > 0 ? address.Items[0].ToString() : "192.168.1.100";
            }
            else
            {
                address.Enabled = operationKind == OperationKind.None;
            }
            if (questionMark != null) toolTip.SetToolTip(questionMark, HintForMode());
            UpdateClientConfig();
        }

        private string HintForMode()
        {
            if (mode.SelectedIndex == 0) return "适用于本机 Android 模拟器。客户端默认地址就是 10.0.2.2，无需改 APK。";
            if (mode.SelectedIndex == 1) return "填写客户端可访问的 IPv4 或域名，支持虚拟局域网及内网穿透。需放行或映射 TCP 主端口和战斗端口。";
            return "填写这台服务器的公网 IPv4 或域名。需映射 TCP 主端口与战斗端口；Admin 不会公网开放。";
        }

        private static IPAddress ParseIPv4(string text)
        {
            IPAddress parsed;
            return IPAddress.TryParse((text ?? "").Trim(), out parsed) && parsed.AddressFamily == AddressFamily.InterNetwork ? parsed : null;
        }

        private string ValidateAddress()
        {
            var host = NormalizeHost(address.Text);
            if (host == null) throw new InvalidOperationException("连接地址必须是有效的 IPv4 或域名；端口请在下方填写。");
            return host;
        }

        private static string NormalizeHost(string text)
        {
            string host = (text ?? "").Trim().ToLowerInvariant();
            if (host.Length == 0 || host.Length > 253) return null;
            if (System.Text.RegularExpressions.Regex.IsMatch(host, "^[0-9.]+$"))
            {
                if (!System.Text.RegularExpressions.Regex.IsMatch(host, @"^(?:0|[1-9][0-9]{0,2})(?:\.(?:0|[1-9][0-9]{0,2})){3}$")) return null;
                var ip = ParseIPv4(host);
                if (ip == null || ip.GetAddressBytes()[0] == 0 || ip.GetAddressBytes()[0] >= 224) return null;
            }
            else foreach (string label in host.Split('.'))
                if (!System.Text.RegularExpressions.Regex.IsMatch(label, "^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$")) return null;
            return host;
        }

        private void UpdateClientConfig()
        {
            if (port == null || address == null || endpoints == null) return;
            var basePort = Decimal.ToInt32(port.Value);
            endpoints.Text = "游戏 " + basePort + " · 战斗 " + (basePort + 1) + " · Admin 127.0.0.1:" + (basePort + 2);
        }

        private async Task PowerClicked()
        {
            var state = CurrentButton();
            if (state.Kind == ServiceButtonKind.Start) await StartServer();
            else if (state.Kind == ServiceButtonKind.Stop) await StopServer();
        }

        private async Task StartServer()
        {
            if (busy) return;
            try
            {
                var host = ValidateAddress();
                var basePort = Decimal.ToInt32(port.Value);
                if (mode.SelectedIndex == 2)
                {
                    var answer = MessageBox.Show(this,
                        "公网联机需要你在路由器或云防火墙映射 TCP " + basePort + " 和 " + (basePort + 1) + "。\r\n" +
                        "Admin 端口不会对外开放。继续启动吗？",
                        "公网联机提示", MessageBoxButtons.OKCancel, MessageBoxIcon.Information);
                    if (answer != DialogResult.OK) return;
                }
                await RunOperation("正在启动服务……", OperationKind.Start, () => controller.StartAsync(host, basePort, validationOnly));
            }
            catch (Exception ex) { ShowFailure(ex.Message); }
        }

        private async Task StopServer()
        {
            if (busy) return;
            await RunOperation("正在停止服务……", OperationKind.Stop, () => controller.StopAsync());
        }

        private async Task RunOperation(string message, OperationKind kind, Func<Task<OperationResult>> action)
        {
            if (busy) return;
            busy = true;
            operationKind = kind;
            operationMessage = message;
            ApplyState(latestState, false);
            try
            {
                var result = await action();
                if (IsDisposed || Disposing) return;
                operationMessage = result.exit_code == 0 ? "服务操作已完成。" :
                    (String.IsNullOrWhiteSpace(result.output) ? "操作失败，退出码 " + result.exit_code : result.output.Trim());
                if (result.exit_code != 0) ShowFailure(operationMessage + "\r\n日志：" + result.diagnostic_path);
            }
            catch (Exception ex)
            {
                if (!IsDisposed && !Disposing) ShowFailure(ex.Message);
            }
            finally
            {
                busy = false;
                operationKind = OperationKind.None;
            }
            await RefreshStatus(false);
        }

        private static string Quote(string value)
        {
            if (value == null) return "\"\"";
            if (value.IndexOf('"') >= 0) throw new InvalidOperationException("参数中不能包含双引号。");
            return "\"" + value + "\"";
        }

        private async Task RefreshStatus(bool initial)
        {
            if (refreshing || IsDisposed || Disposing) return;
            refreshing = true;
            try
            {
                latestState = await controller.ReadStateAsync();
                initialCheckComplete = true;
                if (themeChoice == ThemeChoice.System)
                {
                    var next = ThemeRules.Resolve(themeChoice, ReadAppsUseLightTheme());
                    if (next != resolved)
                    {
                        resolved = next;
                        ApplyPalette();
                    }
                }
                if (IsDisposed || Disposing) return;
                ApplyState(latestState, initial);
            }
            finally { refreshing = false; }
        }

        private ServiceButtonState CurrentButton()
        {
            return ServiceButton.Resolve(
                initialCheckComplete,
                latestState != null,
                latestState != null && latestState.IsReady,
                operationKind == OperationKind.Start,
                operationKind == OperationKind.Stop);
        }

        private void ApplyState(ServerState state, bool initial)
        {
            var running = state != null && state.IsReady;
            if (!initialCheckComplete || (state != null && !state.IsReady)) stateBadge.Text = "● 检测中";
            else if (running) stateBadge.Text = "● 正在运行";
            else stateBadge.Text = "● 未运行";
            StyleBadge();
            if (stateBadge.Parent != null) PlaceHeaderChrome(stateBadge.Parent);
            ApplyServiceButton();
            bool inputs = operationKind == OperationKind.None;
            mode.Enabled = inputs;
            port.Enabled = inputs;
            address.Enabled = inputs && mode.SelectedIndex != 0;
            adminButton.Enabled = state != null && state.IsReady;
            if (running)
            {
                endpoints.Text = "游戏 " + state.port + " · 战斗 " + state.battle_port + " · Admin 127.0.0.1:" + state.admin_port;
                if (initial)
                {
                    port.Value = Math.Max(port.Minimum, Math.Min(port.Maximum, state.port));
                    var runningHost = NormalizeHost(state.advertise_host);
                    if (runningHost != null)
                    {
                        var runningAddress = ParseIPv4(runningHost);
                        mode.SelectedIndex = runningHost == "10.0.2.2"
                            ? 0 : runningAddress != null && IsPrivate(runningAddress) ? 1 : 2;
                        address.Text = runningHost;
                    }
                }
                var detail = "服务运行中（PID " + state.process_id + "）\r\n客户端填写：" + state.advertise_host + ":" + state.port;
                activity.Text = operationMessage.Length == 0 ? detail : operationMessage + "\r\n" + detail;
                FitLogScroll();
            }
            else
            {
                var detail = state != null ? "服务进程存在，正在确认健康状态。" :
                    busy ? "" : "服务尚未运行。选择连接方式后点击「启动服务」。";
                activity.Text = operationMessage.Length == 0 ? detail : operationMessage + "\r\n" + detail;
                FitLogScroll();
            }
        }

        private void FitLogScroll()
        {
            if (activity == null || activity.Font == null) return;
            int lines = activity.GetLineFromCharIndex(Math.Max(0, activity.TextLength - 1)) + 1;
            int visible = Math.Max(1, activity.ClientSize.Height / Math.Max(1, activity.Font.Height));
            activity.ScrollBars = lines > visible ? ScrollBars.Vertical : ScrollBars.None;
        }

        private void ApplyServiceButton()
        {
            if (powerButton == null || palette == null) return;
            var state = CurrentButton();
            powerButton.Text = state.Text;
            powerButton.Enabled = state.Enabled;
            if (!state.Enabled)
            {
                powerButton.BackColor = palette.Disabled;
                powerButton.ForeColor = Color.White;
            }
            else if (state.Kind == ServiceButtonKind.Stop)
            {
                powerButton.BackColor = palette.Danger;
                powerButton.ForeColor = Color.White;
            }
            else
            {
                powerButton.BackColor = palette.Start;
                powerButton.ForeColor = Color.White;
            }
            powerButton.FlatAppearance.MouseOverBackColor = powerButton.BackColor;
            powerButton.FlatAppearance.MouseDownBackColor = powerButton.BackColor;
            RoundControl(powerButton, 8);
            StyleAdminButton();
        }

        private void OpenAdmin()
        {
            if (latestState == null || !latestState.IsReady)
            {
                MessageBox.Show(this, "服务尚未运行，请先点击「启动服务」。", "Admin",
                    MessageBoxButtons.OK, MessageBoxIcon.Information);
                return;
            }
            var adminPort = latestState != null && latestState.admin_port > 0
                ? latestState.admin_port : Decimal.ToInt32(port.Value) + 2;
            try { Process.Start(new ProcessStartInfo("http://127.0.0.1:" + adminPort + "/") { UseShellExecute = true }); }
            catch (Exception ex) { ShowFailure("打开 Admin 失败：" + ex.Message); }
        }

        private void OpenLogs()
        {
            var path = Path.Combine(root, "_local", "runtime");
            try
            {
                Directory.CreateDirectory(path);
                Process.Start(new ProcessStartInfo("explorer.exe", Quote(path)) { UseShellExecute = true });
            }
            catch (Exception ex) { ShowFailure("打开日志目录失败：" + ex.Message); }
        }

        private void ShowFailure(string message)
        {
            operationMessage = "失败：" + message;
            activity.Text = operationMessage;
            MessageBox.Show(this, message, "乖离性服务端启动器", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }

    }
}
