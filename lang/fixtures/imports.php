<?php
namespace App\Http;

use OpenAI\Client;
use GuzzleHttp\Client as HttpClient, Monolog\Logger;
use Illuminate\Support\{Str, Facades\Log as Logs};
use function Laravel\Prompts\text;

function handle($key) {
    $client = OpenAI::client($key);
    return new HttpClient();
}

class Controller {
    public function index() {
        return Str::slug("x");
    }
}
