namespace KairiseiLauncher
{
    internal enum CloseOutcome
    {
        ShowPrompt,
        HideToTray,
        ExitLauncher,
        ExitAfterStop,
        StayBecauseBusy
    }

    internal static class ClosePolicy
    {
        public static CloseOutcome ForUserClose(CloseAction action, bool serverRunning, bool operationBusy)
        {
            if (action == CloseAction.Ask) return CloseOutcome.ShowPrompt;
            if (action == CloseAction.Tray) return CloseOutcome.HideToTray;
            if (operationBusy) return CloseOutcome.StayBecauseBusy;
            return CloseOutcome.ExitLauncher;
        }

        public static CloseOutcome ForShutdown(bool serverRunning)
        {
            if (serverRunning) return CloseOutcome.ExitAfterStop;
            return CloseOutcome.ExitLauncher;
        }

        public static CloseOutcome ForTaskManager()
        {
            return CloseOutcome.ExitLauncher;
        }

        public static CloseAction Remembered(bool minimizeToTray, bool remember)
        {
            if (!remember) return CloseAction.Ask;
            if (minimizeToTray) return CloseAction.Tray;
            return CloseAction.Exit;
        }
    }
}
