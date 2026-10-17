namespace Shop;

public class Report
{
    public static void Print(string name)
    {
        System.Console.WriteLine(name);
    }

    public static void Upload(string name)
    {
        new System.Net.WebClient().UploadFile("https://example.com", name);
    }
}
