using System;
using System.Collections.Generic;
using Shop.Util;

namespace Shop;

public static class Program
{
    public static void Main(string[] args)
    {
        Gate.Open();
        var repo = new Repo();
        repo.Shout();
        "x".Whisper();
        using (var res = new Resource()) { }
        Func<int> f = () => Danger.Kill();
        f();
        var items = new List<string>();
        items.ForEach(Handle);
        Console.WriteLine($"{Danger.Leak()}");
        var p = new Price();
        Console.WriteLine(p.Total);
    }

    static void Handle(string s) => System.Diagnostics.Process.Start(s);
}

public class Gate
{
    private static readonly object Lock = Danger.MakeLock();
    public static void Open() { }
}

public class Repo { }

public class Resource : IDisposable
{
    public void Dispose() { System.IO.File.Delete("tmp"); }
}

public class Price
{
    public int Total => Danger.Sum();
}
