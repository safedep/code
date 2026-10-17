<?php
namespace App;

class Mailer
{
    public function send(string $to): void
    {
        mail($to, 's', 'b');
    }
}

function helper(): void
{
    exec('ls');
}
