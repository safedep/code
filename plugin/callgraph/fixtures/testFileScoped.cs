using OpenAI.Chat;

namespace Demo.App;

public class Agent
{
    public void Ask()
    {
        var client = new ChatClient("gpt-4o", "key");
        client.CompleteChat("hi");
    }
}
