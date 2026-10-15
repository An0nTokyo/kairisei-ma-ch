using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows.Forms;

namespace KairiseiLauncher
{
    internal sealed class ServerState
    {
        public string status { get; set; }
        public int process_id { get; set; }
        public string advertise_host { get; set; }
        public int port { get; set; }
        public int battle_port { get; set; }
        public int admin_port { get; set; }
        public string admin_url { get; set; }
        public string stdout_path { get; set; }
        public string stderr_path { get; set; }
        public bool IsReady { get; set; }
    }

    internal sealed class DeploymentDefaults
    {
        public string advertise_host { get; set; }
        public int port { get; set; }
        public bool validation_only { get; set; }
    }

    internal static class Program
    {
        [STAThread]
        private static int Main(string[] args)
        {
            if (args.Any(value => String.Equals(value, "--smoke-test", StringComparison.OrdinalIgnoreCase)))
            {
                var root = AppDomain.CurrentDomain.BaseDirectory;
                var resourceSet = Path.Combine(root, "resource-set", "resource-set.json");
                var contentRoot = File.Exists(resourceSet) ? "resource-set" : "";
                var required = new[]
                {
                    "kairi-server.exe", "Start-Server.ps1", "Stop-Server.ps1",
                    Path.Combine(contentRoot, "server", "config", "cn602-save-template.json"),
                    Path.Combine(contentRoot, "_local", "control", "server", "cn602-card-runtime-master.json")
                };
                if (File.Exists(resourceSet) && !File.Exists(Path.Combine(root, "Read-ResourceSet.ps1"))) return 2;
                return required.All(value => File.Exists(Path.Combine(root, value))) ? 0 : 2;
            }

            if (args.Any(value => String.Equals(value, "--self-test", StringComparison.OrdinalIgnoreCase)))
                return LauncherSelfTest.Run();

            Mutex mutex;
            if (!SingleInstance.TryBecomePrimary(out mutex))
            {
                SingleInstance.SignalExisting();
                return 0;
            }
            using (mutex)
            {
                Application.EnableVisualStyles();
                Application.SetCompatibleTextRenderingDefault(false);
                Application.Run(new LauncherForm());
                return 0;
            }
        }
    }
}
