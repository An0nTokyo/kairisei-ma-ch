using System;
using System.Threading;

namespace KairiseiLauncher
{
    internal static class SingleInstance
    {
        private const string MutexName = "Local\\KairiseiLauncher";
        private const string EventName = "Local\\KairiseiLauncher.Show";

        public static bool TryBecomePrimary(out Mutex mutex)
        {
            bool created;
            mutex = new Mutex(true, MutexName, out created);
            if (created) return true;
            mutex.Dispose();
            mutex = null;
            return false;
        }

        public static void SignalExisting()
        {
            EventWaitHandle existing;
            if (EventWaitHandle.TryOpenExisting(EventName, out existing))
            {
                using (existing) existing.Set();
            }
        }

        public static void Listen(Action show)
        {
            var handle = new EventWaitHandle(false, EventResetMode.AutoReset, EventName);
            var thread = new Thread(delegate()
            {
                while (true)
                {
                    handle.WaitOne();
                    show();
                }
            });
            thread.IsBackground = true;
            thread.Start();
        }
    }
}
