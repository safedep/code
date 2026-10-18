using System.IO;
using Shop.Util;

namespace Shop;

[ApiController]
public partial class App : ControllerBase, IRunner
{
    private readonly Store store = new Store();

    [HttpGet("/run")]
    public void Run(string path)
    {
        store.Save(File.ReadAllText(path));
        this.Audit();
        Helper.Assist(path);
    }

    private void Audit() => Logger.Info("ran");

    public static void Main(string[] args) => new App().Run(args[0]);
}

public interface IRunner
{
    void Run(string path);
}

public static class Extensions
{
    public static string Trimmed(this string s) => s.Trim();
}
