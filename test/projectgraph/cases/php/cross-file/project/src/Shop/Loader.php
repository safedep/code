<?php
namespace Shop;

use Symfony\Component\Yaml\Yaml;

class Loader
{
    public function __construct(private string $path)
    {
    }

    public function load(): array
    {
        return Yaml::parseFile($this->path);
    }

    public function dump(array $data): string
    {
        return Yaml::dump($data);
    }
}
