namespace Web;

public static class Health
{
    public static string Check()
    {
        return System.Environment.MachineName;
    }

    public static string Unused()
    {
        return System.Environment.UserName;
    }
}
