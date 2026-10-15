namespace KairiseiLauncher
{
    internal enum ServiceButtonKind
    {
        Confirming,
        Start,
        Stop
    }

    internal struct ServiceButtonState
    {
        public ServiceButtonKind Kind;
        public string Text;
        public bool Enabled;
    }

    internal static class ServiceButton
    {
        public static ServiceButtonState Resolve(bool initialCheckComplete, bool processExists, bool healthy, bool startInProgress, bool stopInProgress)
        {
            if (!initialCheckComplete) return Make(ServiceButtonKind.Confirming, "确认服务中", false);
            if (stopInProgress && processExists) return Make(ServiceButtonKind.Stop, "停止服务", false);
            if (startInProgress || (processExists && !healthy)) return Make(ServiceButtonKind.Confirming, "确认服务中", false);
            if (healthy && processExists) return Make(ServiceButtonKind.Stop, "停止服务", true);
            return Make(ServiceButtonKind.Start, "启动服务", true);
        }

        private static ServiceButtonState Make(ServiceButtonKind kind, string text, bool enabled)
        {
            var state = new ServiceButtonState();
            state.Kind = kind;
            state.Text = text;
            state.Enabled = enabled;
            return state;
        }
    }
}
