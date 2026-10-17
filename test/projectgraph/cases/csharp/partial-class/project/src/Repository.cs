namespace Shop.Data;

public partial class Repository
{
    public void Save(string key)
    {
        Validate(key);
        Write(key);
    }
}
