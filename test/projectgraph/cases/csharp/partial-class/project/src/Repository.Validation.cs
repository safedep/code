namespace Shop.Data;

using System.Text.RegularExpressions;

public partial class Repository
{
    private void Validate(string key)
    {
        Regex.IsMatch(key, "^[a-z]+$");
    }

    private void Write(string key)
    {
        System.IO.File.AppendAllText("db.txt", key);
    }
}
