namespace Shop;

public static class Program
{
    static void Persist(IStore store, string key)
    {
        store.Save(key);
    }

    public static void Main(string[] args)
    {
        Persist(new DbStore(), args[0]);
    }
}
