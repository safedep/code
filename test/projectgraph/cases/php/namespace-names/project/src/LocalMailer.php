<?php
namespace App\Local;

class Mailer
{
    public function __construct()
    {
        unlink('x');
    }

    public function send(): void
    {
        passthru('y');
    }
}
