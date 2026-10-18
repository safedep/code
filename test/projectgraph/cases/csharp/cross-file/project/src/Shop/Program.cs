using System;

namespace Shop
{
    public static class Program
    {
        public static void Main(string[] args)
        {
            var loader = new Loader(args[0]);
            loader.Load();
        }
    }
}
