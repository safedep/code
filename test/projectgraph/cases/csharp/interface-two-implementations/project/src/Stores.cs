namespace Shop;

public interface IStore
{
    void Save(string key);
}

public class DbStore : IStore
{
    public void Save(string key)
    {
        System.Data.SqlClient.SqlConnection.ClearAllPools();
    }
}

public class FileStore : IStore
{
    public void Save(string key)
    {
        System.IO.File.Delete(key);
    }
}
