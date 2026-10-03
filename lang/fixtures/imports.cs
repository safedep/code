using System;
using System.Text.Json;
using static System.Math;
global using Microsoft.Extensions.Logging;
using Http = System.Net.Http.HttpClient;

namespace App.Core
{
    using OpenAI.Chat;

    public class Service
    {
        public Service(int size) { }

        public async Task<int> Run(string prompt)
        {
            using (var stream = File.OpenRead("x")) { }
            int Local() => 1;
            return Local();
        }
    }
}
