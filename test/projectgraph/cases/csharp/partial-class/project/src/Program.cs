namespace Shop.App;

using Shop.Data;

public static class Program
{
    public static void Main(string[] args)
    {
        var repo = new Repository();
        repo.Save(args[0]);
    }
}
