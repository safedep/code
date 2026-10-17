using System.IO;

namespace Shop
{
    public class Loader
    {
        private readonly string _path;

        public Loader(string path)
        {
            _path = path;
        }

        public string Load()
        {
            return File.ReadAllText(_path);
        }

        public void Save(string text)
        {
            File.WriteAllText(_path, text);
        }
    }
}
