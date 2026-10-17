namespace Shop.Util;

public static class Ext
{
    public static void Shout(this Shop.Repo r) { System.Net.Dns.GetHostName(); }
    public static void Whisper(this string s) { System.Threading.Thread.Sleep(1); }
}

public static class Danger
{
    public static int Kill() => 1;
    public static int Leak() => 2;
    public static object MakeLock() => System.Environment.Exit(1);
    public static int Sum() => 3;
}
