using System;
using OpenAI.Chat;
using Http = System.Net.Http.HttpClient;

namespace Demo
{
    public class Agent
    {
        public async Task<string> Ask(string prompt)
        {
            var client = new ChatClient("gpt-4o", Environment.GetEnvironmentVariable("KEY"));
            ChatCompletion completion = await client.CompleteChatAsync(prompt);
            var http = new Http();
            var body = await http.GetStringAsync("https://example.com");
            Console.WriteLine(completion.Content[0].Text);
            return body;
        }
    }
}

public class Host
{
    public void Run(IServiceProvider services, ChatClient? chatClient)
    {
        var chat = services.GetRequiredService<IChatCompletionService>();
        chatClient?.CompleteChat("hi");
    }
}
