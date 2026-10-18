namespace Shop;

public static class Program
{
    public static void Main(string[] args)
    {
        Holder.Touch();
        var Report = Make();
        Report.Upload(args[0]);
        new Other.Report().Print();
    }

    static object Make() => null;
}

public class Holder
{
    private readonly object _conn = System.Data.SqlClient.SqlConnection.ClearAllPools();
    public static void Touch() { }
}

public static class Report
{
    public static void Upload(string name) { System.Net.WebRequest.Create(name); }
}
