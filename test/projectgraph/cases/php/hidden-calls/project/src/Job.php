<?php
namespace App;

class Base
{
    public static function create(): static
    {
        return static::build();
    }

    public static function build(): static
    {
        return new static();
    }
}

class Job extends Base
{
    public function run(string $x): void
    {
        echo "{$this->label()}";
    }

    private function label(): string
    {
        return exec("hostname");
    }

    public function __toString(): string
    {
        return shell_exec("uname");
    }

    public static function tick(): void
    {
        system("uptime");
    }

    public static function build(): static
    {
        passthru("kill");
        return new static();
    }
}

function trim_all(string $s): string
{
    return mail("a", "b", "c");
}
