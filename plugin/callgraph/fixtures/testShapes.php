<?php
namespace App;

use OpenAI;
use GuzzleHttp\Client as Http;

class Assistant
{
    public function ask(string $prompt): string
    {
        $client = OpenAI::client(getenv('OPENAI_API_KEY'));
        $result = $client->chat()->create(['model' => 'gpt-4o']);
        $http = new Http(['base_uri' => 'https://example.com']);
        $http->get('/status');
        return strtoupper($result->choices[0]->message->content);
    }
}
