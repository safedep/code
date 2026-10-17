<?php
namespace App;

trait Uploads
{
    public function upload(string $name): void
    {
        file_put_contents('/tmp/' . $name, 'x');
    }
}
