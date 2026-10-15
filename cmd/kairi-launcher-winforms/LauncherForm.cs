using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Runtime.InteropServices;
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
        private readonly ThemeCaptionHost captionHost;
        private readonly LauncherTray tray = new LauncherTray();
        private ComboBox mode;
        private ComboBox address;
        private NumericUpDown port;
        private Label stateBadge;
        private Label endpoints;
        private TextBox activity;
        private Button powerButton;
        private Button adminButton;
        private Button clientThemeButton;
        private Label questionMark;
        private string packagedHost = "10.0.2.2";
        private int packagedPort = 26020;
        private bool validationOnly;
        private bool busy;
        private bool refreshing;
        private bool initialCheckComplete;
        private OperationKind operationKind;
        private ServerState latestState;
        private string operationMessage = "";
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
            captionHost = new ThemeCaptionHost(CycleTheme, PaintThemeGlyph);
            tray.ShowMain += delegate { ShowFromTray(); };
            tray.ExitRequested += delegate { BeginForcedExit(); };
            SystemEvents.SessionEnding += OnSessionEnding;
            BuildUI();
            ApplyPalette();
            LoadDeploymentDefaults();
            HandleCreated += delegate
            {
                TitleBarTheme.Apply(Handle, resolved);
                PlaceThemeButton();
                SingleInstance.Listen(delegate
                {
                    if (IsHandleCreated) BeginInvoke(new Action(ShowFromTray));
                    else ShowFromTray();
                });
            };
            Shown += async delegate
            {
                PlaceThemeButton();
                PopulateAddresses();
                await RefreshStatus(true);
                if (IsDisposed || Disposing) return;
                timer.Interval = 1500;
                timer.Tick += async delegate { await RefreshStatus(false); };
                timer.Start();
            };
            Move += delegate { PlaceThemeButton(); };
            Resize += delegate { PlaceThemeButton(); };
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
                BeginUserClose();
                return;
            }
            base.OnFormClosing(e);
        }

        protected override void OnFormClosed(FormClosedEventArgs e)
        {
            SystemEvents.SessionEnding -= OnSessionEnding;
            timer.Stop();
            if (captionHost != null) captionHost.Hide();
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
                using (var prompt = new ClosePromptForm())
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
            else if (outcome == CloseOutcome.ExitAfterStop) ExitAfterStop();
            else if (outcome == CloseOutcome.StayBecauseBusy)
            {
                ShowFromTray();
                ShowFailure("另一项服务操作正在执行，请稍候。");
            }
        }

        private async void ExitAfterStop()
        {
            if (operationKind != OperationKind.None)
            {
                ShowFromTray();
                ShowFailure("另一项服务操作正在执行，请稍候。");
                return;
            }
            operationKind = OperationKind.Stop;
            busy = true;
            ApplyState(latestState, false);
            try
            {
                var result = await controller.StopAsync();
                if (IsDisposed || Disposing) return;
                if (result.exit_code == 0)
                {
                    allowExit = true;
                    Close();
                    return;
                }
                var text = String.IsNullOrWhiteSpace(result.output) ? "操作失败，退出码 " + result.exit_code : result.output.Trim();
                ShowFromTray();
                ShowFailure(text + "\r\n日志：" + result.diagnostic_path);
            }
            catch (Exception ex)
            {
                if (!IsDisposed && !Disposing)
                {
                    ShowFromTray();
                    ShowFailure(ex.Message);
                }
            }
            finally
            {
                if (!allowExit)
                {
                    busy = false;
                    operationKind = OperationKind.None;
                }
            }
            if (!allowExit && !IsDisposed && !Disposing) await RefreshStatus(false);
        }

        private void HideToTray()
        {
            if (captionHost != null) captionHost.Hide();
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
            PlaceThemeButton();
        }

        private void BuildUI()
        {
            Text = "乖离性百万亚瑟王 · 服务端启动器";
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(460, 640);
            MinimumSize = new Size(476, 700);
            Font = new Font("Microsoft YaHei UI", 10F);
            AutoScaleMode = AutoScaleMode.None;
            try { Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath); } catch { }

            var layout = new TableLayoutPanel
            {
                Dock = DockStyle.Fill,
                ColumnCount = 1,
                RowCount = 5,
                Padding = new Padding(12)
            };
            layout.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
            layout.RowStyles.Add(new RowStyle(SizeType.Absolute, 96F));
            layout.RowStyles.Add(new RowStyle(SizeType.Absolute, 230F));
            layout.RowStyles.Add(new RowStyle(SizeType.Absolute, 96F));
            layout.RowStyles.Add(new RowStyle(SizeType.Percent, 100F));
            layout.RowStyles.Add(new RowStyle(SizeType.Absolute, 28F));
            Controls.Add(layout);

            var header = new Panel { Dock = DockStyle.Fill };
            var iconBox = new PictureBox { Location = new Point(0, 8), Size = new Size(48, 48), SizeMode = PictureBoxSizeMode.Zoom };
            try { iconBox.Image = Icon.ToBitmap(); } catch { }
            header.Controls.Add(iconBox);
            header.Controls.Add(new Label
            {
                Text = "乖离性百万亚瑟王",
                Location = new Point(60, 4),
                AutoSize = true,
                Font = new Font("Microsoft YaHei UI", 16F, FontStyle.Bold)
            });
            stateBadge = new Label
            {
                Text = "● 检测中",
                AutoSize = true,
                Anchor = AnchorStyles.Top | AnchorStyles.Right,
                Location = new Point(300, 8),
                Font = new Font("Microsoft YaHei UI", 10F, FontStyle.Bold)
            };
            header.Controls.Add(stateBadge);
            header.Controls.Add(new Label
            {
                Text = "国服本地服务 · Windows x64",
                Location = new Point(60, 36),
                AutoSize = true
            });
            var sourceLink = new LinkLabel
            {
                Text = "本项目永久免费 · 从 GitHub 获取",
                Font = new Font("Microsoft YaHei UI", 10F, FontStyle.Bold),
                LinkColor = Color.FromArgb(220, 91, 48),
                ActiveLinkColor = Color.FromArgb(222, 78, 78),
                AutoSize = true,
                Location = new Point(60, 62)
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
            clientThemeButton = new Button
            {
                Size = new Size(16, 16),
                Location = new Point(404, 4),
                Anchor = AnchorStyles.Top | AnchorStyles.Right,
                FlatStyle = FlatStyle.Flat,
                TabStop = false
            };
            clientThemeButton.FlatAppearance.BorderSize = 0;
            clientThemeButton.Click += delegate { CycleTheme(); };
            clientThemeButton.Paint += delegate(object sender, PaintEventArgs args) { PaintThemeGlyph(args.Graphics, clientThemeButton.ClientRectangle); };
            header.Controls.Add(clientThemeButton);
            toolTip.SetToolTip(clientThemeButton, ThemeRules.Caption(themeChoice));
            layout.Controls.Add(header, 0, 0);

            var connection = new Panel { Dock = DockStyle.Fill };
            connection.Controls.Add(MakeLabel("使用场景", 0, 0));
            mode = new ComboBox
            {
                DropDownStyle = ComboBoxStyle.DropDownList,
                Location = new Point(0, 22),
                Width = 420,
                Anchor = AnchorStyles.Top | AnchorStyles.Left | AnchorStyles.Right
            };
            mode.Items.AddRange(new object[] { "Android 模拟器", "局域网", "公网 IP / 域名" });
            mode.SelectedIndex = 0;
            mode.SelectedIndexChanged += delegate { ApplyMode(); };
            connection.Controls.Add(mode);
            var addressLabel = MakeLabel("服务器地址", 0, 58);
            connection.Controls.Add(addressLabel);
            questionMark = new Label
            {
                Text = "?",
                AutoSize = false,
                Size = new Size(16, 16),
                Location = new Point(78, 56),
                TextAlign = ContentAlignment.MiddleCenter
            };
            questionMark.Paint += delegate(object sender, PaintEventArgs args)
            {
                args.Graphics.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.AntiAlias;
                using (var pen = new Pen(palette == null ? Color.FromArgb(11, 103, 159) : palette.Accent))
                    args.Graphics.DrawEllipse(pen, 0, 0, 15, 15);
            };
            connection.Controls.Add(questionMark);
            address = new ComboBox
            {
                DropDownStyle = ComboBoxStyle.DropDown,
                Location = new Point(0, 80),
                Width = 280,
                Text = "10.0.2.2"
            };
            address.TextChanged += delegate { UpdateClientConfig(); };
            connection.Controls.Add(address);
            var restoreDefault = MakeButton("恢复默认", 292, 78, 120, 28);
            restoreDefault.Click += delegate { RestoreDeploymentDefaults(); };
            connection.Controls.Add(restoreDefault);
            connection.Controls.Add(MakeLabel("主端口", 0, 116));
            port = new NumericUpDown
            {
                Location = new Point(0, 138),
                Size = new Size(120, 28),
                Minimum = 1,
                Maximum = 65533,
                Value = 26020
            };
            port.ValueChanged += delegate { UpdateClientConfig(); };
            connection.Controls.Add(port);
            endpoints = new Label
            {
                Text = "游戏 26020 · 战斗 26021 · Admin 127.0.0.1:26022",
                Location = new Point(0, 172),
                AutoSize = true
            };
            connection.Controls.Add(endpoints);
            layout.Controls.Add(connection, 0, 1);

            var actions = new Panel { Dock = DockStyle.Fill };
            powerButton = MakeButton("确认服务中", 0, 0, 420, 40);
            powerButton.Anchor = AnchorStyles.Top | AnchorStyles.Left | AnchorStyles.Right;
            powerButton.Enabled = false;
            powerButton.Click += async delegate { await PowerClicked(); };
            actions.Controls.Add(powerButton);
            adminButton = MakeButton("Admin面板", 0, 48, 200, 36);
            var logsButton = MakeButton("查看日志", 212, 48, 208, 36);
            logsButton.Anchor = AnchorStyles.Top | AnchorStyles.Right;
            adminButton.Click += delegate { OpenAdmin(); };
            logsButton.Click += delegate { OpenLogs(); };
            actions.Controls.Add(adminButton);
            actions.Controls.Add(logsButton);
            layout.Controls.Add(actions, 0, 2);

            activity = new TextBox
            {
                Dock = DockStyle.Fill,
                Multiline = true,
                ReadOnly = true,
                ScrollBars = ScrollBars.Vertical,
                BorderStyle = BorderStyle.FixedSingle
            };
            layout.Controls.Add(activity, 0, 3);
            layout.Controls.Add(new Label
            {
                Text = "请完整解压后运行；首次启动会创建全新本地存档。",
                Dock = DockStyle.Fill,
                TextAlign = ContentAlignment.MiddleLeft
            }, 0, 4);

            ApplyMode();
            ApplyServiceButton();
            AutoScaleDimensions = new SizeF(96F, 96F);
            AutoScaleMode = AutoScaleMode.Dpi;
            PerformAutoScale();
        }

        private static Label MakeLabel(string text, int x, int y)
        {
            return new Label { Text = text, Location = new Point(x, y), AutoSize = true };
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
            return button;
        }

        private void ApplyPalette()
        {
            palette = ThemePalette.For(resolved);
            BackColor = palette.Canvas;
            ForeColor = palette.Text;
            ColorControls(this);
            if (activity != null)
            {
                activity.BackColor = palette.Surface;
                activity.ForeColor = palette.Text;
            }
            if (stateBadge != null) stateBadge.ForeColor = latestState != null && latestState.IsReady ? palette.Success : palette.Muted;
            ApplyServiceButton();
            if (clientThemeButton != null) clientThemeButton.Invalidate();
            if (captionHost != null) captionHost.Invalidate();
            TitleBarTheme.Apply(Handle, resolved);
        }

        private void ColorControls(Control parent)
        {
            foreach (Control control in parent.Controls)
            {
                if (control == powerButton || control == adminButton) { }
                else if (control is Button)
                {
                    control.BackColor = palette.PaleButton;
                    control.ForeColor = palette.Accent;
                }
                else if (control is TextBox || control is ComboBox || control is NumericUpDown)
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
            toolTip.SetToolTip(captionHost, ThemeRules.Caption(themeChoice));
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

        private void PlaceThemeButton()
        {
            if (!IsHandleCreated || captionHost == null || clientThemeButton == null) return;
            RECT caption;
            RECT window;
            int status = DwmGetWindowAttribute(Handle, 5, out caption, Marshal.SizeOf(typeof(RECT)));
            GetWindowRect(Handle, out window);
            int themeLeft = window.Left + caption.Left - 20;
            bool fallback = status != 0 || caption.Right <= caption.Left || themeLeft < window.Left + 48;
            if (fallback)
            {
                captionHost.Hide();
                clientThemeButton.Visible = true;
                return;
            }
            clientThemeButton.Visible = false;
            int top = window.Top + caption.Top + Math.Max(0, (caption.Bottom - caption.Top - 16) / 2);
            captionHost.Bounds = new Rectangle(themeLeft, top, 16, 16);
            if (!captionHost.Visible) captionHost.Show(this);
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
            stateBadge.ForeColor = running ? palette.Success : palette.Muted;
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
                activity.Text = operationMessage + "\r\n服务运行中（PID " + state.process_id + "）\r\n客户端填写：" + state.advertise_host + ":" + state.port;
            }
            else
            {
                activity.Text = operationMessage + (state != null ? "\r\n服务进程存在，正在确认健康状态。" :
                    busy ? "" : "\r\n服务尚未运行。选择连接方式后点击「启动服务」。");
            }
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
            adminButton.BackColor = palette.PaleButton;
            adminButton.ForeColor = palette.Accent;
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

        [DllImport("dwmapi.dll")]
        private static extern int DwmGetWindowAttribute(IntPtr window, int attribute, out RECT bounds, int size);

        [DllImport("user32.dll")]
        private static extern bool GetWindowRect(IntPtr window, out RECT bounds);

        [StructLayout(LayoutKind.Sequential)]
        private struct RECT
        {
            public int Left, Top, Right, Bottom;
        }

        private sealed class ThemeCaptionHost : Form
        {
            private readonly Action click;
            private readonly Action<Graphics, Rectangle> paint;

            public ThemeCaptionHost(Action click, Action<Graphics, Rectangle> paint)
            {
                this.click = click;
                this.paint = paint;
                FormBorderStyle = FormBorderStyle.None;
                ShowInTaskbar = false;
                StartPosition = FormStartPosition.Manual;
                Size = new Size(16, 16);
                Click += delegate { click(); };
            }

            protected override bool ShowWithoutActivation { get { return true; } }

            protected override CreateParams CreateParams
            {
                get
                {
                    var parameters = base.CreateParams;
                    parameters.ExStyle |= 0x00000080 | 0x08000000;
                    return parameters;
                }
            }

            protected override void OnPaint(PaintEventArgs e)
            {
                base.OnPaint(e);
                paint(e.Graphics, ClientRectangle);
            }
        }
    }
}
