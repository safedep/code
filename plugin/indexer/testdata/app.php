<?php

namespace Shop;

use Shop\Util\Helper;
use Symfony\Component\Yaml\Yaml;

#[Route('/run')]
class App extends Base implements Runner
{
    use Audits;

    public function __construct(private Store $store) {}

    public function run(string $path): void
    {
        $this->store->save(Yaml::parseFile($path));
        $this->audit();
        Helper::assist($path);
        array_map([$this, 'audit'], []);
    }

    private function audit(): void
    {
        error_log('ran');
    }

    public static function main(array $argv): void
    {
        (new static(new Store()))->run($argv[1]);
    }
}

interface Runner
{
    public function run(string $path): void;
}
